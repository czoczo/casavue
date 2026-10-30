// implement order of icon sources to crawl

package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
)

// each crawl parses whole HTML pages (~10 MB heap per MB page), so they may
// not all run at once on a pod limited to 64 MB
const maxConcurrentCrawls = 8

var crawlSlots = make(chan struct{}, maxConcurrentCrawls)

func crawlItem(key string) {
	crawlSlots <- struct{}{}
	defer func() { <-crawlSlots }()

	// in static generation mode wait,
	// for all items to complete before exiting
	if *staticMode {
		defer wg.Done()
	}

	dashboardItem, version, ok := dashboardItems.read(key)
	if !ok {
		return
	}

	// uncomment for debuging single item
	//if key != "namespace/item_name" { return }

	// echo metadata
	log.Info("Starting icon crawl for '", key, "' at '", dashboardItem.URL)

	// cluster object name: the part of the "namespace/name" store key after the slash
	name := key[strings.LastIndex(key, "/")+1:]

	findHtmlTitle(name, &dashboardItem)

	findDefaultFavicon(&dashboardItem)

	for labelKey, labelVal := range dashboardItem.Labels {
		// check for app.kubernetes.io/instance label
		if labelKey == "app.kubernetes.io/instance" {
			findIconGitHub(&dashboardItem, labelVal)
			log.Debug("GitHub icon search, based on '", labelVal, "' returned: ", dashboardItem.IconURL)
		}
		// check for app.kubernetes.io/name label
		if labelKey == "app.kubernetes.io/name" {
			findIconGitHub(&dashboardItem, labelVal)
			log.Debug("GitHub icon search, based on '", labelVal, "' returned: ", dashboardItem.IconURL)
		}
	}
	checkedNames := map[string]bool{}

	// check Icons on GitHub based on Ingress name
	checkedNames[name] = true
	findIconGitHub(&dashboardItem, strings.ToLower(name))

	// check Icons on GitHub based on site title first word
	titleFirstWord := firstWord(dashboardItem.WebpageTitle)
	if !checkedNames[titleFirstWord] {
		checkedNames[titleFirstWord] = true
		findIconGitHub(&dashboardItem, titleFirstWord)
	}

	// check Icons on GitHub based on site title spaces to dashes
	titleWithDashes := strings.ToLower(strings.ReplaceAll(dashboardItem.WebpageTitle, " ", "-"))
	if !checkedNames[titleWithDashes] {
		checkedNames[titleWithDashes] = true
		findIconGitHub(&dashboardItem, titleWithDashes)
	}

	// check header for PNG
	findHtmlIcon(&dashboardItem, "png")

	// check header for SVG
	findHtmlIcon(&dashboardItem, "svg")

	// check header with https://pkg.go.dev/go.deanishe.net/favicon
	findHtmlIconDeanishe(&dashboardItem)

	// check for first level of DNS domain
	addressPrefix := strings.Split(getHostFromURL(dashboardItem.URL), ".")[0]
	if !checkedNames[addressPrefix] {
		checkedNames[addressPrefix] = true
		findIconGitHub(&dashboardItem, addressPrefix)
	}

	// last resort - generate avatar
	getGeneratedIcon(&dashboardItem, key)

	// download if static mode
	if *staticMode {
		downloadedIconFile := downloadIcon(dashboardItem.IconURL)
		log.Info("Downloaded icon file: ", downloadedIconFile, ", from: ", dashboardItem.IconURL)
		dashboardItem.IconURL = downloadedIconFile
	}

	// write result, unless the item was changed or deleted while crawling
	if !dashboardItems.writeIfVersion(key, version, dashboardItem) {
		log.Info("Discarding crawl result for '", key, "': item changed while crawling")
		return
	}
	log.Info("Icon crawl result for '", key, "': icon - ", dashboardItem.IconURL, ", title - ", dashboardItem.WebpageTitle)
}

func refreshItems() {
	for _, key := range dashboardItems.getKeys() {
		if *staticMode {
			wg.Add(1)
		}
		go crawlItem(key)
	}
	if *staticMode {
		wg.Wait()

		// create folder for static api file if not existant
		log.Debug("Creating folder if non-existent: ", filepath.Dir(compiledVuePath+"/"+staticApiPath))
		os.MkdirAll(filepath.Dir(compiledVuePath+staticApiPath), os.ModePerm)

		err := exportConfigAsJSONFile(dashboardItems.get(), compiledVuePath+staticApiPath)
		if err != nil {
			fmt.Printf("Error creating JSON file: %v\n", err)
		}
	}
}
