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
	Name         string            `json:"name"`
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
	// per item version, bumped by every write; lets crawls notice they became stale
	versions map[string]int64
	sequence int64
}

var dashboardItems DashboardItemsStore

// clients store concurrency methods. A copy is returned: the API handler walks
// the map while informer and crawl goroutines write into it.
func (cs *DashboardItemsStore) get() map[string]DashEntry {
	cs.RLock()
	defer cs.RUnlock()
	result := make(map[string]DashEntry, len(cs.items))
	for key, value := range cs.items {
		result[key] = value
	}
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

func (cs *DashboardItemsStore) read(key string) (DashEntry, int64, bool) {
	cs.RLock()
	defer cs.RUnlock()
	result, ok := cs.items[key]
	return result, cs.versions[key], ok
}

func (cs *DashboardItemsStore) write(key string, value DashEntry) {
	cs.Lock()
	defer cs.Unlock()
	cs.sequence++
	cs.items[key] = value
	cs.versions[key] = cs.sequence
}

// writeIfVersion stores the value only when nothing touched the item since it
// was read with the returned version. Icon crawls outlive the cluster events
// which started them, so without this a finished crawl would bring back an
// item which was deleted or renamed in the meantime.
func (cs *DashboardItemsStore) writeIfVersion(key string, version int64, value DashEntry) bool {
	cs.Lock()
	defer cs.Unlock()
	if cs.versions[key] != version {
		return false
	}
	cs.sequence++
	cs.items[key] = value
	cs.versions[key] = cs.sequence
	return true
}

func (cs *DashboardItemsStore) delete(key string) {
	cs.Lock()
	defer cs.Unlock()
	delete(cs.items, key)
	delete(cs.versions, key)
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
