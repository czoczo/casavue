package main

import (
	"encoding/json"
	"sync"
	"testing"
)

func newTestStore() *DashboardItemsStore {
	return &DashboardItemsStore{
		items:    map[string]DashEntry{},
		versions: map[string]int64{},
	}
}

func entry(url string) DashEntry {
	return DashEntry{Namespace: "default", Name: "app", URL: url}
}

// The API handler marshals the whole store while informer and crawl goroutines
// write into it. Handing out the live map used to kill the process with
// "concurrent map iteration and map write".
func TestStoreSurvivesConcurrentReadAndWrite(t *testing.T) {
	store := newTestStore()
	for i := 0; i < 20; i++ {
		store.write("default/app"+string(rune('a'+i)), entry("http://example.com"))
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if _, err := json.Marshal(store.get()); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := "default/app" + string(rune('a'+i))
				store.write(key, entry("http://changed.example.com"))
				store.delete(key)
			}
		}(i)
	}
	wg.Wait()
}

// A finished crawl may not write back an item which was changed or deleted
// while it was running, otherwise renamed and deleted items come back to life.
func TestStaleCrawlResultIsDropped(t *testing.T) {
	store := newTestStore()
	store.write("default/app", entry("http://example.com"))

	staleEntry, version, ok := store.read("default/app")
	if !ok {
		t.Fatal("item not found")
	}

	store.delete("default/app")
	staleEntry.IconURL = "/avatars/default/app"
	if store.writeIfVersion("default/app", version, staleEntry) {
		t.Error("crawl result of a deleted item must be dropped")
	}
	if _, _, ok := store.read("default/app"); ok {
		t.Error("deleted item must stay deleted")
	}

	// same for an item whose URL was changed while the old crawl was running
	store.write("default/app", entry("http://new.example.com"))
	newEntry, newVersion, _ := store.read("default/app")
	if store.writeIfVersion("default/app", version, staleEntry) {
		t.Error("crawl result based on an outdated version must be dropped")
	}
	if found, _, _ := store.read("default/app"); found.URL != newEntry.URL {
		t.Errorf("stale write must not overwrite the current entry, got %q", found.URL)
	}

	if !store.writeIfVersion("default/app", newVersion, newEntry) {
		t.Error("write with the current version must succeed")
	}
}
