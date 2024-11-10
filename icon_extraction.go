// obtaining artifacts for dashboard items

package main

import (
	log "github.com/sirupsen/logrus"
	"go.deanishe.net/favicon"
	"golang.org/x/net/html"
	"regexp"
	"strings"
)

func findIconGitHub(entry *DashEntry, nameValue string) {
	log.Debug("findIconGitHub looking up: ", nameValue)
	if entry.IconURL != "" {
		log.Debug("findIconGitHub function: IconURL already found: ", entry.IconURL)
		return
	}

	if nameValue == "" {
		log.Debug("findIconGitHub function: no input passed to function.")
		return
	}

	name := nameValue
	ghUrl := "https://raw.githubusercontent.com/walkxcode/dashboard-icons/main/"
	svgUrl := ghUrl + "svg/" + name + ".svg"
	_, err := checkUrlStatus(svgUrl)
	if err == nil {
		log.Debug("findIconGitHub function: found icon: ", svgUrl)
		entry.IconURL = svgUrl
		return
	}
	pngUrl := ghUrl + "png/" + name + ".png"
	_, err = checkUrlStatus(pngUrl)
	if err == nil {
		log.Debug("findIconGitHub function: found icon: ", pngUrl)
		entry.IconURL = pngUrl
	}
	return
}

func findHtmlTitle(name string, entry *DashEntry) {
	log.Debug("findHtmlTitle looking up item:", name)
	title := ""
	if entry.WebpageTitle != "" {
		log.Debug("findHtmlTitle function: WebpageTitle already found.")
		return
	}

	resp, err := httpClient.Get(strings.TrimSpace(entry.URL))
	if err != nil {
		log.Error("findHtmlTitle function error: ", err)
		return
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Error("findHtmlTitle function error: ", err)
		return
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.TrimSpace(n.Data) == "title" {
			if n.FirstChild != nil {
				title = n.FirstChild.Data
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)
	if title == "" {
		title = name
	}
	log.Debug("findHtmlTitle function: returning:" + title)
	entry.WebpageTitle = title
}

func findHtmlIcon(entry *DashEntry, format string) {
	log.Debug("findHtmlIcon looking up: ", entry.URL)
	if entry.IconURL != "" {
		log.Debug("findHtmlIcon function: IconURL already found.")
		return
	}

	resp, err := httpClient.Get(strings.TrimSpace(entry.URL))
	if err != nil {
		log.Error("findHtmlIcon function: http.Get error: ", err)
		return
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Error("findHtmlIcon function: html.Parse error: ", err)
		return
	}

	pattern := ".*icon.*"
	re := regexp.MustCompile(pattern)
	finalPngUrl := ""
	finalSvgUrl := ""
	size := -1

	var f func(*html.Node)
	f = func(n *html.Node) {
		if strings.TrimSpace(n.Data) == "link" {
			href := false
			sizes := false
			icon := false
			url := ""
			sizesVal := ""
			for _, a := range n.Attr {
				if a.Key == "rel" && re.MatchString(a.Val) {
					icon = true
				}
				if a.Key == "href" {
					href = true
					url = a.Val
				}
				if a.Key == "sizes" {
					sizes = true
					sizesVal = a.Val
				}

				// if looking for png format and required attributes present
				if format == "png" && href && (sizes || icon) {

					var biggestSize int
					// get size from either 'sizes' or 'href' attributes
					if sizesVal != "" {
						biggestSize = getSizeFromString(sizesVal)
					} else {
						biggestSize = getSizeFromString(url)
					}

					// if larger than any other found size then update
					if biggestSize > size {
						size = biggestSize
						finalPngUrl = url
					}
				}

				// if looking for svg format and required attributes present
				if format == "svg" && href && (sizes || icon) {
					if strings.Contains(strings.ToLower(url), ".svg") {
						finalSvgUrl = url
					}
				}
			}

		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)
	if format == "svg" && finalSvgUrl != "" {
		prefixedFinalSvgUrl := addPrefix(entry.URL, finalSvgUrl)
		_, err = checkUrlStatus(prefixedFinalSvgUrl)
		if err != nil {
			log.Warn("findHtmlIcon function: error reading icon URL for: ", prefixedFinalSvgUrl)
			return
		}
		entry.IconURL = prefixedFinalSvgUrl
		log.Debug("findHtmlIcon function, found icon: ", entry.IconURL)
	}
	if format == "png" && finalPngUrl != "" {
		prefixedFinalPngUrl := addPrefix(entry.URL, finalPngUrl)
		_, err = checkUrlStatus(prefixedFinalPngUrl)
		if err != nil {
			log.Warn("findHtmlIcon function: error reading icon URL for: ", prefixedFinalPngUrl)
			return
		}
		entry.IconURL = prefixedFinalPngUrl
		log.Debug("findHtmlIcon function, found icon: ", entry.IconURL)
	}
	log.Debug("findHtmlIcon function, icon not found")
}

func findHtmlIconDeanishe(entry *DashEntry) {
	log.Debug("fingHtmlIconDeanishe looking up: ", entry.URL)
	if entry.IconURL != "" {
		log.Debug("findHtmlIconDeanishe function: IconURL already found.")
		return
	}

	// return, if endpoint unreachable
	_, err := httpClient.Get(strings.TrimSpace(entry.URL))
	if err != nil {
		log.Error("findHtmlIconDeanishe function: http.Get error: ", err)
		return
	}

	f := favicon.New(
		favicon.OnlySquare,
	)

	icons, err := f.Find(entry.URL)
	if err != nil {
		log.Warn(err)
	}

	if len(icons) < 1 {
		log.Debug("findHtmlIconDeanishe function, did not find any icons for: ", entry.URL)
		return
	}

	// prefair SVG
	for _, i := range icons {
		if strings.HasPrefix(i.MimeType, "image/svg") {
			entry.IconURL = i.URL
			log.Debug("findHtmlIconDeanishe function: found svg icon: ", i.URL)
			return
		}
	}

	potentialIcon := icons[0].URL
	_, err = checkUrlStatus(potentialIcon)
	if err != nil {
		log.Warn("findHtmlIconDeanishe function: error reading icon URL for: ", entry.URL)
		return
	}
	entry.IconURL = potentialIcon
	log.Debug("findHtmlIconDeanishe function: found icon: ", entry.IconURL)
}
