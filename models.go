// data structures for dashboard items

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
)

// DashEntry represents the structure of each entry in the dashboard.

type DashEntry struct {
	Namespace    string            `json:"namespace"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	WebpageTitle string            `json:"title"`
	IconURL      string            `json:"iconURL"`
	Labels       map[string]string `json:"labels"`
}

// thread safe store for items
type DashboardItemsStore struct {
	sync.RWMutex
	items map[string]DashEntry
}

var dashboardItems DashboardItemsStore

// clients store concurrency methods
func (cs *DashboardItemsStore) get() map[string]DashEntry {
	cs.RLock()
	result := cs.items
	cs.RUnlock()
	return result
}

func (cs *DashboardItemsStore) getKeys() []string {
	var result []string
	cs.RLock()
	for k := range cs.items {
		result = append(result, k)
	}
	cs.RUnlock()
	return result
}

func (cs *DashboardItemsStore) read(key string) (DashEntry, bool) {
	cs.RLock()
	result, ok := cs.items[key]
	cs.RUnlock()
	return result, ok
}

func (cs *DashboardItemsStore) write(key string, value DashEntry) {
	cs.Lock()
	cs.items[key] = value
	cs.Unlock()
}

func (cs *DashboardItemsStore) delete(key string) {
	cs.Lock()
	delete(cs.items, key)
	cs.Unlock()
}

func exportConfigAsJSONFile(data map[string]DashEntry, filePath string) error {
	// Convert the map to a JSON string
	jsonString, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Write the JSON string to a file
	err = ioutil.WriteFile(filePath, jsonString, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("JSON data saved to file: %s\n", filePath)
	return nil
}
