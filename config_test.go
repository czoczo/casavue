package main

import (
	"os"
	"path/filepath"
	"testing"

	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mustParseMainConfig parses over the defaults built into the binary.
func mustParseMainConfig(t *testing.T, mainYAML string) Config {
	t.Helper()
	cfg, err := parseMainConfig([]byte(mainYAML))
	if err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
	return cfg
}

// isolateConfig puts a fresh configuration and empty item store in place for one
// test and puts back what was there before it.
func isolateConfig(t *testing.T, mainYAML string) {
	t.Helper()
	saved := config
	appliedStaticItems = map[string]Item{}
	watchedStores.stores = nil
	httpClient = newHttpClient(false)
	dashboardItems.items = map[string]DashEntry{}
	dashboardItems.versions = map[string]int64{}
	t.Cleanup(func() {
		config = saved
		appliedStaticItems = map[string]Item{}
		watchedStores.stores = nil
		dashboardItems.items, dashboardItems.versions = map[string]DashEntry{}, map[string]int64{}
	})

	config = mustParseMainConfig(t, mainYAML)
}

func staticItem(name, url string) Item {
	return Item{Name: name, Namespace: "static", URL: url}
}

// A configuration which the app cannot work with must not become the live one: it is
// refused with an error instead of crashing the process later on.
func TestUnusableConfigurationsAreRefused(t *testing.T) {
	isolateConfig(t, "")

	for name, mainYAML := range map[string]string{
		"unknown filter mode":    "content_filters:\n  item:\n    mode: \"whatever\"\n",
		"invalid filter pattern": "content_filters:\n  item:\n    mode: \"exclude\"\n    pattern: \"[\"\n",
		"unknown log level":      "logging:\n  level: \"everything\"\n",
		"invalid theme color":    "customization:\n  colors:\n    theme: \"purple\"\n",
		"logo not base64":        "customization:\n  logo: \"###\"\n",
		"logo not an svg":        "customization:\n  logo: \"PGh0bWwvPg==\"\n", // <html/>
	} {
		if _, err := parseMainConfig([]byte(mainYAML)); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}

	cfg := mustParseMainConfig(t, "customization:\n  name: MyHome\n  colors:\n    theme: \"#bada55\"\n")
	if cfg.Customization.Name != "MyHome" {
		t.Error("customization not applied over the defaults")
	}
	// untouched values have to stay at their defaults
	if cfg.Content_filters.Item.Mode != "include" || cfg.Logging.Level != "info" {
		t.Errorf("defaults lost: %+v", cfg)
	}
}

// A broken file which lands on the cluster later may only ever mean the previous
// configuration stays in use.
func TestBrokenConfigIsNotApplied(t *testing.T) {
	isolateConfig(t, "")
	readStaticItems(StaticItems{Items: []Item{staticItem("app1", "http://127.0.0.1:1/")}})

	reloadConfig([]byte("logging:\n  level: \"everything\"\n"), []byte("items: [\n"))

	if config.Logging.Level != "info" {
		t.Errorf("broken main.yaml was applied, level is %q", config.Logging.Level)
	}
	if _, _, ok := dashboardItems.read("static/app1"); !ok {
		t.Error("rejected configuration must not change the dashboard")
	}
}

// A reloaded items.yaml has to add, update and remove entries, and an entry which
// did not change must keep what its icon crawl found for it.
func TestStaticItemsReload(t *testing.T) {
	isolateConfig(t, "")

	readStaticItems(StaticItems{Items: []Item{
		staticItem("same", "http://127.0.0.1:1/"),
		staticItem("gone", "http://127.0.0.1:1/"),
	}})
	dashboardItems.write("static/same", DashEntry{Namespace: "static", Name: "same",
		URL: "http://127.0.0.1:1/", WebpageTitle: "Crawled Title", IconURL: "/icons/static/same"})
	unchangedVersion := func() int64 {
		_, version, ok := dashboardItems.read("static/same")
		if !ok {
			t.Fatal("item disappeared")
		}
		return version
	}
	beforeVersion := unchangedVersion()

	changed := readStaticItems(StaticItems{Items: []Item{
		staticItem("same", "http://127.0.0.1:1/"),
		staticItem("new", "http://127.0.0.1:2/"),
	}})

	if _, _, ok := dashboardItems.read("static/gone"); ok {
		t.Error("item removed from items.yaml is still on the dashboard")
	}
	if new, _, ok := dashboardItems.read("static/new"); !ok || new.URL != "http://127.0.0.1:2/" {
		t.Errorf("new item not added: %+v (found=%v)", new, ok)
	}
	if unchangedVersion() != beforeVersion {
		t.Error("unchanged item was rewritten, its crawled title and icon are lost")
	}
	if stored, _, _ := dashboardItems.read("static/same"); stored.WebpageTitle != "Crawled Title" {
		t.Errorf("crawled title lost: %+v", stored)
	}

	if len(changed) != 1 || changed[0] != "static/new" {
		t.Errorf("only the new item has to be crawled, got %v", changed)
	}
}

// fakeStore stands in for an informer store.
type fakeStore struct{ objects []interface{} }

func (f *fakeStore) List() []interface{} { return f.objects }

func watchedIngress(namespace, name string) *v1.Ingress {
	return &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: namespace,
			// a URL of the local test host, so that the icon crawl of a newly added
			// item fails immediately instead of going over the network
			Annotations: map[string]string{"casavue.app/url": "http://127.0.0.1:1/"},
		},
	}
}

// The content filters of a new configuration apply to the cluster objects which are
// already known, not only to the ones changing from now on.
func TestContentFiltersReappliedToWatchedObjects(t *testing.T) {
	isolateConfig(t, "")
	store := &fakeStore{objects: []interface{}{
		watchedIngress("public", "shown"),
		watchedIngress("private", "hidden"),
	}}
	watchedStores.add(store, dashEntryFromIngress)

	config = mustParseMainConfig(t, "content_filters:\n  namespace:\n    mode: \"include\"\n    pattern: \"^public$\"\n")
	applyWatchedObjects()
	if _, _, ok := dashboardItems.read("public/shown"); !ok {
		t.Error("item of an allowed namespace not added")
	}
	if _, _, ok := dashboardItems.read("private/hidden"); ok {
		t.Error("item of a filtered out namespace added")
	}

	// the other way round, without a single cluster event in between
	config = mustParseMainConfig(t, "content_filters:\n  namespace:\n    mode: \"exclude\"\n    pattern: \"^public$\"\n")
	applyWatchedObjects()
	if _, _, ok := dashboardItems.read("public/shown"); ok {
		t.Error("item of a namespace filtered out by the new configuration is still on the dashboard")
	}
	if _, _, ok := dashboardItems.read("private/hidden"); !ok {
		t.Error("item of a namespace allowed by the new configuration not added")
	}
}

// The Helm chart mounts only the items ConfigMap by default, so CasaVue runs on a
// read-only directory without a main.yaml and has to serve the items of the file which
// is there.
func TestMissingMainConfigFileIsTolerated(t *testing.T) {
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "config")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatal(err)
	}
	itemsPath := filepath.Join(dir, "items.yaml")
	if err := os.WriteFile(itemsPath, []byte("items:\n- name: app1\n  namespace: static\n  url: http://127.0.0.1:1/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	savedMainPath, savedItemsPath, savedConfig, savedStatic := configFilePath, itemsFilePath, config, appliedStaticItems
	t.Cleanup(func() {
		configFilePath, itemsFilePath, config, appliedStaticItems = savedMainPath, savedItemsPath, savedConfig, savedStatic
		dashboardItems.items, dashboardItems.versions = map[string]DashEntry{}, map[string]int64{}
	})
	httpClient = newHttpClient(false)
	configFilePath, itemsFilePath = filepath.Join(readOnlyDir, "main.yaml"), itemsPath
	dashboardItems.items, dashboardItems.versions = map[string]DashEntry{}, map[string]int64{}

	loadConfig()
	if _, _, ok := dashboardItems.read("static/app1"); !ok {
		t.Error("items.yaml not loaded although only main.yaml is missing")
	}
	if config.Logging.Level != "info" {
		t.Errorf("main.yaml defaults not used, level is %q", config.Logging.Level)
	}
}

// The whole reload path: files on disk, unchanged files, a changed items.yaml and a
// broken main.yaml.
func TestChangedConfigFilesAreReloaded(t *testing.T) {
	dir := t.TempDir()
	mainPath, itemsPath := filepath.Join(dir, "main.yaml"), filepath.Join(dir, "items.yaml")
	write := func(path string, content string) {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(mainPath, "")
	write(itemsPath, "items:\n- name: app1\n  namespace: static\n  url: http://127.0.0.1:1/\n")

	savedMainPath, savedItemsPath, savedConfig, savedStatic := configFilePath, itemsFilePath, config, appliedStaticItems
	t.Cleanup(func() {
		configFilePath, itemsFilePath, config, appliedStaticItems = savedMainPath, savedItemsPath, savedConfig, savedStatic
		dashboardItems.items, dashboardItems.versions = map[string]DashEntry{}, map[string]int64{}
	})
	httpClient = newHttpClient(false)
	configFilePath, itemsFilePath = mainPath, itemsPath
	dashboardItems.items, dashboardItems.versions = map[string]DashEntry{}, map[string]int64{}

	loadConfig()
	if _, _, ok := dashboardItems.read("static/app1"); !ok {
		t.Fatal("configuration files not loaded at startup")
	}

	if reloadChangedConfig() {
		t.Error("unchanged configuration files must not reload anything")
	}

	write(itemsPath, "items:\n- name: app2\n  namespace: static\n  url: http://127.0.0.1:2/\n")
	if !reloadChangedConfig() {
		t.Fatal("changed items.yaml did not reload the configuration")
	}
	if _, _, ok := dashboardItems.read("static/app2"); !ok {
		t.Error("new static item not added")
	}
	if _, _, ok := dashboardItems.read("static/app1"); ok {
		t.Error("removed static item still on the dashboard")
	}

	// a broken main.yaml, even together with a good items.yaml, changes nothing
	write(mainPath, "logging:\n  level: \"everything\"\n")
	write(itemsPath, "items:\n- name: app3\n  namespace: static\n  url: http://127.0.0.1:3/\n")
	if !reloadChangedConfig() {
		t.Fatal("changed files must be noticed even when they are refused")
	}
	if config.Logging.Level != "info" {
		t.Errorf("broken main.yaml was applied, level is %q", config.Logging.Level)
	}
	if _, _, ok := dashboardItems.read("static/app2"); !ok {
		t.Error("refused configuration changed the dashboard")
	}
	if _, _, ok := dashboardItems.read("static/app3"); ok {
		t.Error("refused configuration changed the dashboard")
	}
}
