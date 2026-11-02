// crawling the icon of one dashboard item and refreshing all of them

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// each crawl parses a whole HTML page, so they may not all run at once on a pod
// limited to 64 MB
const maxConcurrentCrawls = 8

var crawlSlots = make(chan struct{}, maxConcurrentCrawls)

func crawlItem(key string) {
	crawlSlots <- struct{}{}
	defer func() { <-crawlSlots }()

	// in static generation mode wait
	// for all items to complete before exiting
	if *staticMode {
		defer wg.Done()
	}

	item, version, ok := dashboardItems.read(key)
	if !ok {
		return
	}

	// uncomment for debuging single item
	//if key != "namespace/item_name" { return }

	// cluster object name: the part of the "namespace/name" store key after the slash
	name := key[strings.LastIndex(key, "/")+1:]

	log.Info("Starting icon crawl for '", key, "' at '", item.URL)

	page := fetchSite(item.URL)
	if item.WebpageTitle == "" {
		item.WebpageTitle = pageTitle(page, name)
	}

	if item.IconURL != "" {
		// the icon was chosen in the configuration or by an annotation,
		// in static mode only a local copy of it is made
		if *staticMode {
			item.IconURL = downloadIcon(item.IconURL)
		}
	} else {
		iconURL, icon := findIcon(&item, page)
		switch {
		case icon == nil:
			// last resort - generate avatar
			getGeneratedIcon(&item, key)
		case *staticMode:
			item.IconURL = saveIcon(icon)
		default:
			item.IconURL = iconURL
		}
	}

	// write result, unless the item was changed or deleted while crawling
	if !dashboardItems.writeIfVersion(key, version, item) {
		log.Info("Discarding crawl result for '", key, "': item changed while crawling")
		return
	}
	log.Info("Icon crawl result for '", key, "': icon - ", item.IconURL, ", title - ", item.WebpageTitle)
}

func pageTitle(page *sitePage, name string) string {
	if page != nil && page.title != "" {
		return page.title
	}
	return name
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
