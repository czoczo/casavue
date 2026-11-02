package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func testPNG(t *testing.T, side int) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, image.NewRGBA(image.Rect(0, 0, side, side))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// testICO is an .ico file of two pictures, the size 0 of the second one means
// 256 px.
func testICO(t *testing.T) []byte {
	t.Helper()
	picture := testPNG(t, 32)
	data := []byte{0, 0, 1, 0, 2, 0}
	for _, entry := range [][2]int{{32, len(picture)}, {0, len(picture)}} {
		header := make([]byte, 16)
		header[0], header[1] = byte(entry[0]), byte(entry[0])
		binary.LittleEndian.PutUint16(header[4:6], 1)
		binary.LittleEndian.PutUint16(header[6:8], 32)
		binary.LittleEndian.PutUint32(header[8:12], uint32(entry[1]))
		binary.LittleEndian.PutUint32(header[12:16], uint32(6+32+len(picture)-entry[1]))
		data = append(data, header...)
	}
	return append(data, picture...)
}

func TestImageKind(t *testing.T) {
	kind, width, height, ok := imageKind(testPNG(t, 64))
	if !ok || kind != "png" || width != 64 || height != 64 {
		t.Errorf("png: %q %dx%d ok=%v", kind, width, height, ok)
	}

	if kind, _, _, ok := imageKind([]byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`)); !ok || kind != "svg" {
		t.Errorf("svg: %q ok=%v", kind, ok)
	}

	// an HTML error page is no icon, not even one served as favicon.ico
	if _, _, _, ok = imageKind([]byte("<!DOCTYPE html><html><body><svg>x</svg></body></html>")); ok {
		t.Error("HTML must be rejected")
	}

	if _, _, _, ok = imageKind(nil); ok {
		t.Error("no bytes are no icon")
	}
}

func TestIcoDims(t *testing.T) {
	if width, height, ok := icoDims(testICO(t)); !ok || width != 256 || height != 256 {
		t.Errorf("got %dx%d ok=%v", width, height, ok)
	}
	if _, _, ok := icoDims([]byte("not an ico file at all")); ok {
		t.Error("must reject files without an ico header")
	}
}

func TestAnnouncedSize(t *testing.T) {
	for _, test := range []struct {
		sizes string
		url   string
		want  int
	}{
		{"32x32", "/favicon.png", 32},
		{"512x512 192x192", "/icon.png", 512},
		{"any", "/icon.svg", 512},
		{"", "/img/icon-180x180.png", 180},
		{"", "/img/icon.png", 0},
	} {
		if got := announcedSize(test.sizes, test.url); got != test.want {
			t.Errorf("announcedSize(%q, %q) = %d, expected %d", test.sizes, test.url, got, test.want)
		}
	}
}

func TestResolveURL(t *testing.T) {
	base, err := url.Parse("https://example.com/app/page.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ href, want string }{
		{"/favicon.ico", "https://example.com/favicon.ico"},
		{"img/icon.png", "https://example.com/app/img/icon.png"},
		{"https://cdn.example.com/icon.svg", "https://cdn.example.com/icon.svg"},
		{"javascript:void(0)", ""},
		{"", ""},
	} {
		if got := resolveURL(base, test.href); got != test.want {
			t.Errorf("resolveURL(%q) = %q, expected %q", test.href, got, test.want)
		}
	}
}

// A page announcing a big broken icon next to a smaller whole one has to give
// us the whole one, and the icon of a manifest has to win over the banner
// picture of social media.
func TestBestIconOfAPage(t *testing.T) {
	initHttpClient(false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><head><title>Some App</title>
			<link rel="icon" sizes="256x256" href="/broken.ico">
			<meta property="og:image" content="/banner.png">
			<link rel="manifest" href="/manifest.webmanifest">
			<link rel="mask-icon" href="/silhouette.svg">
			</head><body>x</body></html>`))
		case "/broken.ico":
			w.Write([]byte("<!DOCTYPE html><html>Not found</html>"))
		case "/manifest.webmanifest":
			w.Header().Set("Content-Type", "application/manifest+json")
			w.Write([]byte(`{"icons":[{"src":"/icon-48x48.png","sizes":"48x48"},{"src":"/touch-180x180.png","sizes":"180x180"}]}`))
		default:
			w.Write(testPNG(t, 180))
		}
	}))
	defer server.Close()

	page := fetchSite(server.URL)
	if page.title != "Some App" {
		t.Errorf("title %q", page.title)
	}
	if len(page.candidates) != 2 {
		t.Errorf("candidates %+v", page.candidates)
	}

	candidates := append(page.candidates, manifestIcons(page.manifest)...)
	iconURL, icon := bestIcon(dedupe(candidates), maxSiteIconFetches)
	if icon == nil {
		t.Fatalf("no icon found in %+v", candidates)
	}
	if icon.size != 180 || icon.kind != "png" {
		t.Errorf("got %s (%d px) from %s", icon.kind, icon.size, iconURL)
	}
	if iconURL != server.URL+"/touch-180x180.png" {
		t.Errorf("unexpected source %s", iconURL)
	}
}

// The file has to land next to the frontend files, not in a folder of the same
// name below it, and it has to be named after the bytes it holds.
func TestSaveIconFile(t *testing.T) {
	icon := &verifiedIcon{url: "https://example.com/favicon.ico", data: testPNG(t, 32), kind: "png", size: 32}
	saved := saveIcon(icon)
	if !strings.HasPrefix(saved, downloadedAvatarsPath+"/") {
		t.Errorf("unexpected path %q", saved)
	}
	if !strings.HasSuffix(saved, ".png") {
		t.Errorf("a PNG saved as %q", saved)
	}
	if _, err := os.Stat(compiledVuePath + "/" + saved); err != nil {
		t.Errorf("icon file is missing: %v", err)
	}
	t.Cleanup(func() { os.Remove(compiledVuePath + "/" + saved) })
}

// A browser loads the icons of the dashboard as pictures of another site, and
// many applications refuse those: the qBittorrent WebUI answers 401 to a request
// carrying the Referer of a foreign page. The dashboard has to be served the
// icons by CasaVue, which is not sending a Referer nor an Origin.
func TestIconProxyServesTheIconOfADashboardItem(t *testing.T) {
	initHttpClient(false)

	served := map[string]bool{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Referer() != "" || r.Header.Get("Origin") != "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/svg":
			served["svg"] = true
			w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"/>`))
		case "/broken":
			w.Write([]byte("not a picture at all"))
		case "/logo":
			w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"/>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	dashboardItems = DashboardItemsStore{
		items: map[string]DashEntry{
			"media/qbittorrent": {Namespace: "media", Name: "qbittorrent", IconURL: upstream.URL + "/svg"},
			"media/broken":      {Namespace: "media", Name: "broken", IconURL: upstream.URL + "/broken"},
			"media/missing":     {Namespace: "media", Name: "missing", IconURL: upstream.URL + "/nothing-here"},
			"media/local":       {Namespace: "media", Name: "local", IconURL: "/avatars/media/local"},
			"media/anonymous":   {Namespace: "media", Name: "anonymous", IconURL: upstream.URL + "/logo?v=1"},
		},
		versions: map[string]int64{"media/qbittorrent": 1},
	}

	for _, tc := range []struct {
		key         string
		status      int
		contentType string
	}{
		{"media/qbittorrent", http.StatusOK, "image/svg+xml"},
		{"media/broken", http.StatusNotFound, ""},
		{"media/missing", http.StatusNotFound, ""},
		{"media/local", http.StatusNotFound, ""},
		{"other/not-an-item", http.StatusNotFound, ""},
		// a name without a telling file name, announced as a vector by the page
		{"media/anonymous", http.StatusOK, "image/svg+xml"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, iconProxyURL(tc.key), nil)
		request.Header.Set("Referer", "https://dashboard.example.com/")
		request.Header.Set("Origin", "https://dashboard.example.com")
		iconProxyHandler(recorder, request)

		if recorder.Code != tc.status {
			t.Errorf("item %q: got status %d, expected %d", tc.key, recorder.Code, tc.status)
		}
		if ct := recorder.Header().Get("Content-Type"); tc.status == http.StatusOK && ct != tc.contentType {
			t.Errorf("item %q: content type %q, expected %q", tc.key, ct, tc.contentType)
		}
	}
	if !served["svg"] {
		t.Error("the icon of the item was not fetched from its own host")
	}

	// an SVG is served as a picture only, never as a document of this server
	recorder := httptest.NewRecorder()
	iconProxyHandler(recorder, httptest.NewRequest(http.MethodGet, iconProxyURL("media/qbittorrent"), nil))
	if csp := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src") {
		t.Errorf("served SVG without a content security policy: %q", csp)
	}
}

func TestProxiedIconsKeepsLocalIconAddresses(t *testing.T) {
	items := proxiedIcons(map[string]DashEntry{
		"media/qbittorrent": {IconURL: "https://qbittorrent.example.com/images/icon.svg"},
		"media/casavue":     {IconURL: "/avatars/media/casavue"},
		"searching/Google":  {IconURL: "http://www.google.com/favicon.ico"},
	})
	if items["media/qbittorrent"].IconURL != iconProxyURL("media/qbittorrent") {
		t.Errorf("remote icon not proxied: %q", items["media/qbittorrent"].IconURL)
	}
	if items["searching/Google"].IconURL != iconProxyURL("searching/Google") {
		t.Errorf("http icon not proxied: %q", items["searching/Google"].IconURL)
	}
	if items["media/casavue"].IconURL != "/avatars/media/casavue" {
		t.Errorf("own icon changed: %q", items["media/casavue"].IconURL)
	}
}

// A vector is the icon to want from a page, whatever the page announces next to
// it and whatever the file name of the vector is called.
func TestVectorIconWinsOverAnnouncedRasterOnes(t *testing.T) {
	initHttpClient(false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><head><title>Some App</title>
			<link rel="icon" type="image/png" sizes="32x32" href="images/app32.png">
			<link rel="icon" type="image/png" sizes="512x512" href="/big.png">
			<link rel="icon" type="image/svg+xml" href="/logo?v=1">
			</head><body>x</body></html>`))
		case "/logo":
			w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"/>`))
		default:
			w.Write(testPNG(t, 512))
		}
	}))
	defer server.Close()

	iconURL, icon := findIcon(&DashEntry{Name: "app"}, fetchSite(server.URL))
	if icon == nil {
		t.Fatal("no icon found")
	}
	if icon.kind != "svg" || iconURL != server.URL+"/logo?v=1" {
		t.Errorf("got %s (%d px) from %s", icon.kind, icon.size, iconURL)
	}
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestNameCandidates(t *testing.T) {
	item := DashEntry{Name: "Home Assistant", Labels: map[string]string{"app.kubernetes.io/name": "homeassistant"}}
	repoURL, err := url.Parse("https://github.com/immich-app/immich#-demo")
	if err != nil {
		t.Fatal(err)
	}
	names := nameCandidates(&item, &sitePage{finalURL: repoURL})
	if names[0] != "immich" {
		t.Errorf("the repository name must come first, got %v", names)
	}
	if !hasToken(names, "homeassistant") || !hasToken(names, "home-assistant") {
		t.Errorf("missing name variants: %v", names)
	}
	if !hasToken(nameCandidates(&DashEntry{Name: "Open Source Routing Machine (OSRM)"}, nil), "open-source-routing-machine-osrm") {
		t.Error("punctuation has to disappear from icon pack names")
	}
	forge := nameCandidates(&DashEntry{Name: "Wallos", WebpageTitle: "GitHub - ellite/Wallos: x"}, &sitePage{finalURL: mustParse(t, "https://github.com/ellite/wallos")})
	if hasToken(forge, "github") {
		t.Errorf("a link to a forge must not ask for the icon of the forge: %v", forge)
	}
	if len(names) > maxPackNames {
		t.Errorf("%v exceed the limit of %d names", names, maxPackNames)
	}
}

// A link to a repository must not end up with the favicon of the code forge.
func TestSocialCardsOfARepositoryPage(t *testing.T) {
	page := &sitePage{finalURL: mustParse(t, "https://github.com/immich-app/immich")}
	kept := socialCards(page, []iconCandidate{
		{url: "https://github.githubassets.com/favicons/favicon.svg", source: fromLinkIcon},
		{url: "https://github.com/apple-touch-icon.png", source: fromMetaImage},
		{url: "https://opengraph.githubassets.com/12ab/immich-app/immich", source: fromMetaImage},
	})
	if len(kept) != 1 || kept[0].url != "https://opengraph.githubassets.com/12ab/immich-app/immich" {
		t.Errorf("unexpected candidates kept: %v", kept)
	}
}

func TestOriginsOf(t *testing.T) {
	page, err := url.Parse("https://demo.wondercms.com/demo")
	if err != nil {
		t.Fatal(err)
	}
	item, err := url.Parse("https://www.wondercms.com/demo")
	if err != nil {
		t.Fatal(err)
	}
	origins := originsOf(page, item)
	if len(origins) != 2 || origins[0] != "https://demo.wondercms.com" || origins[1] != "https://www.wondercms.com" {
		t.Errorf("unexpected origins %v", origins)
	}
	if origins := originsOf(page, page); len(origins) != 1 {
		t.Errorf("%v of the same host must be probed once", origins)
	}
}

func TestIsRepoLink(t *testing.T) {
	for _, test := range []struct {
		url  string
		want bool
	}{
		{"https://github.com/foo/bar", true},
		{"https://demo.example.com/", false},
	} {
		parsed, err := url.Parse(test.url)
		if err != nil {
			t.Fatal(err)
		}
		if got := isRepoLink(parsed); got != test.want {
			t.Errorf("isRepoLink(%q) = %v", test.url, got)
		}
	}
}
