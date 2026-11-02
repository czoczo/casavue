// finding the icon of a web application: the metadata of its HTML page, its
// PWA manifest, the paths favicons are usually found under and a curated icon
// pack. Every candidate is verified by decoding its bytes, so neither an HTML
// error page nor a 1x1 pixel ever becomes an icon.

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

const (
	// HTML pages bigger than this are truncated before parsing, so that one
	// remote server can't exhaust the memory of the pod
	maxPageBytes     = 1 << 20
	maxIconBytes     = 2 << 20
	maxManifestBytes = 512 << 10

	// how many downloads one item may spend looking for its icon. Only items
	// without an obvious icon use the whole budget.
	maxSiteIconFetches = 10
	maxPackIconFetches = 10
	maxPackNames       = 6

	iconRequestTimeout = 10 * time.Second
	// demo sites answer slowly and a site we do not reach at all is a lost icon
	pageRequestTimeout = 25 * time.Second

	// images smaller than this are spacers and tracking pixels, not icons
	minIconDim = 14

	// icon pack of the dashboard community, plain files, no icon service
	iconPackURL = "https://raw.githubusercontent.com/homarr-labs/dashboard-icons/main/"
)

// Where a candidate was found. Only breaks ties between icons of a similar
// size, better sources first.
const (
	fromPack = iota
	fromManifest
	fromLinkIcon
	fromAppleTouch
	fromMetaImage
	fromWellKnown
)

// packNameJunk are the characters icon pack file names never contain.
var packNameJunk = regexp.MustCompile(`[^a-z0-9]+`)

// iconCandidate is one hint about the icon of an item, before its bytes were seen.
type iconCandidate struct {
	url    string
	data   []byte // of a data: URI link, nil when the url has to be fetched
	size   int    // size announced by the page in px, 0 when unknown
	kind   string // png, svg, ... "" when unknown
	source int
}

// verifiedIcon is an icon whose bytes were seen and found to be a picture.
type verifiedIcon struct {
	url  string
	data []byte
	kind string
	size int // longest side in px, 0 for SVG and formats Go cannot read
}

func initHttpClient(tlsSkipVerify bool) {
	if tlsSkipVerify {
		log.Warn("Disabling TLS checks")
	}
	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: tlsSkipVerify},
		},
		Timeout: 30 * time.Second,
	}
}

// getLimited downloads at most limit bytes and reports the URL the response
// finally came from, because relative icon links of a page are resolved
// against that URL and not against the URL we asked for.
func getLimited(requestURL string, limit int64, timeout time.Duration) (data []byte, finalURL string, statusCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, "", 0, err
	}
	// some servers answer a browser-looking client only
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CasaVue)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,image/*,*/*;q=0.8")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(io.LimitReader(resp.Body, limit))
	return data, resp.Request.URL.String(), resp.StatusCode, err
}

// sitePage is all one request to a dashboard item tells us about it.
type sitePage struct {
	finalURL   *url.URL
	origins    []string // hosts of the page and of the URL we were asked for, final first
	title      string
	candidates []iconCandidate
	manifest   string // manifest linked by the page, "" when there is none
}

func fetchSite(itemURL string) *sitePage {
	itemURL = strings.TrimSpace(itemURL)
	if itemURL == "" {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(itemURL), "http") {
		itemURL = "https://" + itemURL
	}
	requested, err := url.Parse(itemURL)
	if err != nil {
		log.Debug("fetchSite: ", itemURL, ": ", err)
		return nil
	}

	data, finalURL, statusCode, err := getLimited(itemURL, maxPageBytes, pageRequestTimeout)
	if err != nil {
		log.Debug("fetchSite: ", itemURL, ": ", err)
		// the page did not answer, but its well known icon paths still might
		return &sitePage{finalURL: requested, origins: originsOf(requested, nil)}
	}
	log.Debug("fetchSite: ", finalURL, " status ", statusCode, ", ", len(data), " bytes")

	pageBase, err := url.Parse(finalURL)
	if err != nil {
		return nil
	}
	page := &sitePage{finalURL: pageBase, origins: originsOf(pageBase, requested)}

	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		log.Debug("fetchSite: cannot parse ", finalURL, ": ", err)
		return page
	}
	for child := doc.FirstChild; child != nil; child = child.NextSibling {
		page.walk(child)
	}
	return page
}

func (page *sitePage) walk(node *html.Node) {
	if node.Type == html.ElementNode {
		switch node.Data {
		case "title":
			if page.title == "" {
				page.title = strings.Join(strings.Fields(textOf(node)), " ")
			}
		case "link":
			page.link(node)
		case "meta":
			page.meta(node)
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		page.walk(child)
	}
}

func (page *sitePage) link(node *html.Node) {
	rel := strings.Fields(strings.ToLower(attr(node, "rel")))
	if hasToken(rel, "manifest") {
		if page.manifest == "" {
			page.manifest = resolveURL(page.finalURL, attr(node, "href"))
		}
		return
	}
	// a black silhouette for Safari browser tabs, useless on a dashboard tile
	if hasToken(rel, "mask-icon") {
		return
	}
	source := fromLinkIcon
	switch {
	case hasToken(rel, "apple-touch-icon"), hasToken(rel, "apple-touch-icon-precomposed"):
		source = fromAppleTouch
	case hasToken(rel, "icon"), hasToken(rel, "shortcut"), hasToken(rel, "fluid-icon"):
	default:
		return
	}
	page.add(attr(node, "href"), attr(node, "sizes"), attr(node, "type"), source)
}

func (page *sitePage) meta(node *html.Node) {
	name := strings.ToLower(attr(node, "property") + " " + attr(node, "name"))
	if !strings.Contains(name, "og:image") && !strings.Contains(name, "twitter:image") && !strings.Contains(name, "tileimage") {
		return
	}
	page.add(attr(node, "content"), "", "", fromMetaImage)
}

func (page *sitePage) add(href, sizes, mimeType string, source int) {
	if data, ok := inlineData(href); ok {
		kind, _, _, valid := imageKind(data)
		if valid {
			page.candidates = append(page.candidates, iconCandidate{data: data, kind: kind, source: source})
		}
		return
	}
	target := resolveURL(page.finalURL, href)
	if target == "" {
		return
	}
	page.candidates = append(page.candidates, iconCandidate{
		url:    target,
		size:   announcedSize(sizes, target),
		kind:   iconKind(target, mimeType),
		source: source,
	})
}

// manifestIcons reads the icons of a PWA manifest, which modern web apps
// maintain themselves and which are usually the biggest ones available.
func manifestIcons(manifestURL string) []iconCandidate {
	if manifestURL == "" {
		return nil
	}
	data, finalURL, statusCode, err := getLimited(manifestURL, maxManifestBytes, iconRequestTimeout)
	if err != nil || statusCode != http.StatusOK {
		log.Debug("manifest ", manifestURL, ": ", err, " status ", statusCode)
		return nil
	}
	var manifest struct {
		Icons []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
			Type  string `json:"type"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &manifest); err != nil {
		log.Debug("manifest ", finalURL, " is not JSON: ", err)
		return nil
	}
	manifestBase, err := url.Parse(finalURL)
	if err != nil {
		return nil
	}
	var candidates []iconCandidate
	for _, icon := range manifest.Icons {
		target := resolveURL(manifestBase, icon.Src)
		if target == "" {
			continue
		}
		candidates = append(candidates, iconCandidate{
			url:    target,
			size:   announcedSize(icon.Sizes, target),
			kind:   iconKind(target, icon.Type),
			source: fromManifest,
		})
	}
	return candidates
}

// Paths favicons and manifests are found under when a page does not link them.
var (
	wellKnownIcons = []string{"/favicon.ico", "/favicon.svg", "/favicon.png", "/apple-touch-icon.png"}
	//nolint:revive
	wellKnownManifests = []string{"/manifest.webmanifest", "/manifest.json", "/site.webmanifest"}
)

// originsOf are the hosts to probe: a demo runs often on another host than the
// site which advertises the application, and that one has the icon.
func originsOf(final, requested *url.URL) []string {
	var origins []string
	for _, u := range []*url.URL{final, requested} {
		if u == nil {
			continue
		}
		origin := u.Scheme + "://" + u.Host
		if !slices.Contains(origins, origin) {
			origins = append(origins, origin)
		}
	}
	return origins
}

func wellKnownCandidates(page *sitePage) []iconCandidate {
	var candidates []iconCandidate
	origins := page.origins
	if len(origins) == 0 {
		origins = originsOf(page.finalURL, nil)
	}
	if len(origins) > 0 {
		for _, manifestPath := range wellKnownManifests {
			candidates = append(candidates, manifestIcons(origins[0]+manifestPath)...)
		}
	}
	for _, origin := range origins {
		for _, iconPath := range wellKnownIcons {
			candidates = append(candidates, iconCandidate{
				url:    origin + iconPath,
				kind:   kindHint(iconPath),
				source: fromWellKnown,
			})
		}
	}
	return candidates
}

// findIcon looks for the icon of an item and reports where it came from and
// what its bytes are, so that the caller does not have to fetch it again.
func findIcon(item *DashEntry, page *sitePage) (string, *verifiedIcon) {
	names := nameCandidates(item, page)

	// a link to a code forge carries the icon of the forge, the name of the
	// repository is the better lead to the icon of the application
	repo := page != nil && isRepoLink(page.finalURL)
	if repo {
		if foundURL, icon := bestIcon(dedupe(packCandidates(names)), maxPackIconFetches); icon != nil {
			return foundURL, icon
		}
	}

	var candidates []iconCandidate
	if page != nil {
		candidates = append(candidates, page.candidates...)
		if !repo {
			candidates = append(candidates, manifestIcons(page.manifest)...)
			if len(candidates) == 0 {
				candidates = wellKnownCandidates(page)
			}
		}
	}
	if repo {
		return bestIcon(dedupe(socialCards(page, candidates)), maxSiteIconFetches)
	}
	if foundURL, icon := bestIcon(dedupe(candidates), maxSiteIconFetches); icon != nil {
		return foundURL, icon
	}
	log.Debug("findIcon: no icon of ", pageURL(page), ", trying the icon pack")
	return bestIcon(dedupe(packCandidates(names)), maxPackIconFetches)
}

// socialCards keeps the pictures a page says of itself (og:image, twitter:image).
// A repository page of a code forge has such a picture of every repository, while
// the favicon, the touch icon and the manifest of that page all belong to the
// forge and not to the application the repository is about.
func socialCards(page *sitePage, candidates []iconCandidate) []iconCandidate {
	var kept []iconCandidate
	for _, candidate := range candidates {
		if candidate.source != fromMetaImage {
			continue
		}
		if u, err := url.Parse(candidate.url); err == nil && u.Host == page.finalURL.Host {
			continue
		}
		kept = append(kept, candidate)
	}
	return kept
}

func pageURL(page *sitePage) string {
	if page == nil {
		return "<no page>"
	}
	return page.finalURL.String()
}

// packCandidates are the icons of the community icon pack named after the
// application.
func packCandidates(names []string) []iconCandidate {
	var candidates []iconCandidate
	for _, name := range names {
		candidates = append(candidates,
			iconCandidate{url: iconPackURL + "svg/" + name + ".svg", kind: "svg", source: fromPack},
			iconCandidate{url: iconPackURL + "png/" + name + ".png", kind: "png", source: fromPack},
		)
	}
	return candidates
}

// nameCandidates are the names an application is known under, the most
// specific ones first.
func nameCandidates(item *DashEntry, page *sitePage) []string {
	var names []string
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return
		}
		for _, known := range names {
			if known == name {
				return
			}
		}
		names = append(names, name)
	}

	repo := page != nil && isRepoLink(page.finalURL)
	if repo {
		add(repoName(page.finalURL))
	}
	add(item.Labels["app.kubernetes.io/name"])
	add(item.Labels["app.kubernetes.io/instance"])
	add(item.Name)
	if !repo {
		// the first word of a forge page title is the name of the forge, and the
		// icon pack has an icon of every forge in it
		add(firstWord(item.WebpageTitle))
	}
	// multi word names are written with dashes in the icon pack
	for _, name := range append([]string(nil), names...) {
		add(strings.ReplaceAll(name, " ", "-"))
	}
	// and without any other punctuation
	for _, name := range append([]string(nil), names...) {
		add(strings.Trim(packNameJunk.ReplaceAllString(name, "-"), "-"))
	}
	if len(names) > maxPackNames {
		names = names[:maxPackNames]
	}
	return names
}

// bestIcon downloads candidates in the order of their chances and stops at the
// first picture which fills a dashboard tile.
func bestIcon(candidates []iconCandidate, maxFetches int) (string, *verifiedIcon) {
	sort.SliceStable(candidates, func(i, j int) bool {
		return declaredScore(candidates[i]) > declaredScore(candidates[j])
	})

	var best *verifiedIcon
	bestScore := 0
	fetches := 0
	for _, candidate := range candidates {
		if candidate.url != "" {
			if fetches >= maxFetches {
				break
			}
			fetches++
		}
		icon := verifyIcon(candidate)
		if icon == nil {
			continue
		}
		if score := verifiedScore(icon, candidate.source); best == nil || score > bestScore {
			best, bestScore = icon, score
		}
		// 192 px or a vector fill a tile, paying for more candidates changes nothing
		if best.kind == "svg" || best.size >= 192 {
			break
		}
	}
	if best == nil {
		return "", nil
	}
	return best.url, best
}

// verifyIcon downloads a candidate and rejects everything which is not a
// picture of a usable size.
func verifyIcon(candidate iconCandidate) *verifiedIcon {
	data := candidate.data
	if data == nil {
		body, finalURL, statusCode, err := getLimited(candidate.url, maxIconBytes, iconRequestTimeout)
		if err != nil {
			log.Debug("verifyIcon: ", candidate.url, ": ", err)
			return nil
		}
		// hosts which measure pictures do not like a whole dashboard crawling them
		if statusCode == http.StatusTooManyRequests {
			time.Sleep(2 * time.Second)
			body, finalURL, statusCode, err = getLimited(candidate.url, maxIconBytes, iconRequestTimeout)
			if err != nil {
				log.Debug("verifyIcon: ", candidate.url, ": ", err)
				return nil
			}
		}
		if statusCode != http.StatusOK {
			log.Debug("verifyIcon: ", finalURL, " status ", statusCode)
			return nil
		}
		candidate.url = finalURL
		data = body
	}
	kind, width, height, ok := imageKind(data)
	if !ok || (width > 0 && (width < minIconDim || height < minIconDim)) {
		if candidate.url != "" {
			log.Debug("verifyIcon: ", candidate.url, " is not a usable picture")
		}
		return nil
	}
	size := width
	if height > size {
		size = height
	}
	return &verifiedIcon{url: candidate.url, data: data, kind: kind, size: size}
}

// imageKind tells which picture the bytes are and how big it is. SVG is text,
// so it is recognised by its tags, everything else by decoding its header.
func imageKind(data []byte) (kind string, width, height int, ok bool) {
	if len(data) == 0 {
		return "", 0, 0, false
	}
	head := data
	if len(head) > 2048 {
		head = head[:2048]
	}
	lower := bytes.ToLower(head)
	if bytes.Contains(lower, []byte("<svg")) && !bytes.Contains(lower, []byte("<html")) {
		return "svg", 0, 0, true
	}
	if icoWidth, icoHeight, isIco := icoDims(data); isIco {
		return "ico", icoWidth, icoHeight, true
	}
	if bytes.HasPrefix(head, []byte("RIFF")) && len(head) > 12 && bytes.Equal(head[8:12], []byte("WEBP")) {
		return "webp", 0, 0, true
	}
	if len(head) > 12 && bytes.Equal(head[4:8], []byte("ftyp")) && bytes.Equal(head[8:11], []byte("avi")) {
		return "avif", 0, 0, true
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0, false
	}
	return format, config.Width, config.Height, true
}

// icoDims reports the biggest picture inside an .ico file. A side of 0 in its
// directory means 256.
func icoDims(data []byte) (width int, height int, ok bool) {
	if len(data) < 22 || !bytes.Equal(data[:4], []byte{0x00, 0x00, 0x01, 0x00}) {
		return 0, 0, false
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	for i := 0; i < count; i++ {
		entry := 6 + i*16
		if entry+2 > len(data) {
			break
		}
		side := int(data[entry])
		if side == 0 {
			side = 256
		}
		if side > width {
			width = side
		}
		if side > height {
			height = side
		}
	}
	return width, height, width > 0
}

// score ranks an icon: a vector first, then the biggest picture, then the
// source which proves to be the icon of the application most often.
func score(kind string, size, source int) int {
	if kind == "svg" {
		return 4000 - source
	}
	// bigger than a dashboard tile is no better
	dim := size
	if dim > 256 {
		dim = 256
	}
	// og:image is a banner picture, its size says nothing about the icon
	if source == fromMetaImage && dim > 128 {
		dim = 128
	}
	var points int
	switch {
	case dim >= 256:
		points = 300
	case dim >= 192:
		points = 280
	case dim >= 128:
		points = 250
	case dim >= 96:
		points = 220
	case dim >= 64:
		points = 190
	case dim >= 48:
		points = 170
	case dim >= 32:
		points = 150
	case dim >= 16:
		points = 120
	case dim > 0:
		points = 60
	default:
		points = 100 // of an unknown size, browsers still show something
	}
	return points*10 - source
}

func declaredScore(candidate iconCandidate) int {
	return score(candidate.kind, candidate.size, candidate.source)
}

func verifiedScore(icon *verifiedIcon, source int) int {
	return score(icon.kind, icon.size, source)
}

// saveIcon stores the bytes of a verified icon next to the frontend files, so
// that the static dashboard does not depend on the icon still being there
// tomorrow. The file is named after the URL it came from and its extension
// comes from its bytes, so a PNG served as favicon.ico is not saved as .ico.
func saveIcon(icon *verifiedIcon) string {
	name := strToSha256(icon.id()) + "." + icon.fileExt()
	log.Debug("saveIcon: ", icon.url, " -> ", name)
	folder := compiledVuePath + "/" + downloadedAvatarsPath
	if err := os.MkdirAll(folder, os.ModePerm); err != nil {
		log.Error("creating ", folder, ": ", err)
		return icon.url
	}
	if err := os.WriteFile(folder+"/"+name, icon.data, 0644); err != nil {
		log.Error("saving ", name, ": ", err)
		return icon.url
	}
	return downloadedAvatarsPath + "/" + name
}

// downloadIcon stores a local copy of an icon whose URL was written into the
// configuration or into an ingress annotation.
func downloadIcon(iconURL string) string {
	if strings.HasPrefix(iconURL, ".") {
		return iconURL // generated or stored by us already
	}
	icon := verifyIcon(iconCandidate{url: iconURL})
	if icon == nil {
		log.Warn("configured icon is not a picture, linking it instead: ", iconURL)
		return iconURL
	}
	return saveIcon(icon)
}

// isRemoteIcon tells an icon the browser has to fetch from another host from a
// file or avatar served by CasaVue itself.
func isRemoteIcon(iconURL string) bool {
	return strings.HasPrefix(strings.ToLower(iconURL), "http")
}

// iconProxyURL is where this server serves the icon of one dashboard item from,
// so that the dashboard loads all its pictures from one host.
func iconProxyURL(key string) string {
	return "/icons/?item=" + url.QueryEscape(key)
}

// proxiedIcons replaces the icon addresses of the items an API response is
// built for, so that the browser asks this server for the picture and not the
// application the item points at. Those hosts refuse a picture loaded by a
// foreign page: Zot answers with Cross-Origin-Resource-Policy: same-origin, the
// qBittorrent WebUI answers 401 to any request carrying the Referer of another
// site, and an http:// icon is dropped as mixed content by an https://
// dashboard.
func proxiedIcons(items map[string]DashEntry) map[string]DashEntry {
	for key, item := range items {
		if isRemoteIcon(item.IconURL) {
			item.IconURL = iconProxyURL(key)
			items[key] = item
		}
	}
	return items
}

// iconProxyHandler answers with the icon of one dashboard item, fetched by this
// server, which is not sent the Referer or the Origin a browser would send.
// Only items already on the dashboard are answered, so the endpoint cannot be
// used to reach arbitrary hosts.
func iconProxyHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("item")
	item, _, found := dashboardItems.read(key)
	if !found || !isRemoteIcon(item.IconURL) {
		http.Error(w, "unknown dashboard item", http.StatusNotFound)
		return
	}

	data, finalURL, statusCode, err := getLimited(item.IconURL, maxIconBytes, iconRequestTimeout)
	if err != nil || statusCode != http.StatusOK {
		log.Warn("Icon request to ", item.IconURL, " ended with status ", statusCode, ": ", err)
		http.Error(w, "icon unavailable", http.StatusNotFound)
		return
	}
	kind, _, _, ok := imageKind(data)
	if !ok {
		log.Warn("Icon ", finalURL, " is not a picture")
		http.Error(w, "not a picture", http.StatusNotFound)
		return
	}

	// the bytes come from another host, so they must never run as a document of this one
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// shortcut: the address of a proxied icon does not change with the icon, so a
	// replaced icon is served only after this cache expires
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", contentTypeFor(kind))
	w.Write(data)
}

// contentTypeFor names the format of the verified bytes for the browser.
func contentTypeFor(kind string) string {
	switch kind {
	case "svg":
		return "image/svg+xml"
	case "ico":
		return "image/vnd.microsoft.icon"
	case "jpeg", "png", "gif", "webp", "avif", "bmp":
		return "image/" + kind
	}
	return "application/octet-stream"
}

// id identifies where the icon came from, for icons inlined by a data: URI the
// bytes are all there is.
func (icon *verifiedIcon) id() string {
	if icon.url != "" {
		return icon.url
	}
	return strToSha256(string(icon.data))
}

func (icon *verifiedIcon) fileExt() string {
	if icon.kind == "jpeg" {
		return "jpg"
	}
	return icon.kind
}

// announcedSize reads the size out of the attributes of a page and of the file
// name of the icon.
func announcedSize(sizes, iconURL string) int {
	// SVG icons announce themselves as "any", for us as big as they can be
	if strings.Contains(strings.ToLower(sizes), "any") {
		return 512
	}
	if size := getSizeFromString(sizes); size > 0 {
		return size
	}
	return getSizeFromString(iconURL)
}

// iconKind is the format a page announces for an icon: the file name first,
// then the `type` attribute of the link or of the manifest entry, which is all
// there is for an icon without a telling file name. It only orders the
// candidates, the bytes of the picture decide what an icon really is.
func iconKind(iconURL, mimeType string) string {
	if kind := kindHint(iconURL); kind != "" {
		return kind
	}
	// a vector is the icon to want, so it must not stay of an unknown kind
	if strings.Contains(strings.ToLower(mimeType), "svg") {
		return "svg"
	}
	return ""
}

// kindHint guesses the format of an icon from its file name. It only orders
// the candidates, the bytes decide what an icon really is.
func kindHint(iconURL string) string {
	name := strings.ToLower(iconURL)
	if query := strings.IndexAny(name, "?#"); query >= 0 {
		name = name[:query]
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".svg":
		return "svg"
	case ".ico", ".cur":
		return "ico"
	case ".png":
		return "png"
	case ".gif":
		return "gif"
	case ".jpg", ".jpeg", ".jfif":
		return "jpeg"
	case ".webp":
		return "webp"
	case ".bmp":
		return "bmp"
	}
	return ""
}

// resolveURL makes a link of a page absolute. Icons linked from other hosts
// are followed as they were by the crawler this code replaces.
func resolveURL(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	target, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if !target.IsAbs() {
		target = base.ResolveReference(target)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return ""
	}
	return target.String()
}

// inlineData unpacks icon links which carry the picture itself, as pages
// without any external file do.
func inlineData(href string) ([]byte, bool) {
	head, encoded, found := strings.Cut(strings.TrimSpace(href), ",")
	if !found || !strings.HasPrefix(strings.ToLower(head), "data:image/") {
		return nil, false
	}
	if data, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return data, true
	}
	if data, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
		return data, true
	}
	return nil, false
}

func dedupe(candidates []iconCandidate) []iconCandidate {
	var result []iconCandidate
	seen := map[string]bool{}
	for _, candidate := range candidates {
		key := candidate.url
		if key == "" {
			key = string(candidate.data)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, candidate)
	}
	return result
}

// isRepoLink tells apart links to a code forge, which point at the project
// instead of at a running instance of it.
func isRepoLink(pageURL *url.URL) bool {
	host := strings.ToLower(pageURL.Hostname())
	for _, forge := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
		if host == forge || strings.HasSuffix(host, "."+forge) {
			return true
		}
	}
	return false
}

// repoName is the name of the repository behind a link to a code forge.
func repoName(pageURL *url.URL) string {
	segments := strings.FieldsFunc(pageURL.Path, func(r rune) bool { return r == '/' })
	if len(segments) == 0 {
		return ""
	}
	name := segments[len(segments)-1]
	if len(segments) > 1 {
		name = segments[1]
	}
	return strings.TrimSuffix(name, ".git")
}

func hasToken(tokens []string, token string) bool {
	for _, t := range tokens {
		if t == token {
			return true
		}
	}
	return false
}

func attr(node *html.Node, key string) string {
	for _, a := range node.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func textOf(node *html.Node) string {
	var text strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			text.WriteString(child.Data)
		}
	}
	return text.String()
}
