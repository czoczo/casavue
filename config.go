// handling configuration files

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

//go:embed config/main.yaml
var default_config string

//go:embed config/items.yaml
var default_static_items string

// how often the configuration files are looked at for changes made outside of
// the process (a new ConfigMap in Kubernetes)
const configReloadCheckInterval = 5 * time.Second

type Customization struct {
	Name string `json:"name"`
	// base64 encoded SVG replacing the logo of CasaVue, not passed to the frontend
	Logo   string `json:"-" yaml:"logo"`
	Colors struct {
		Theme string `json:"theme"`
		Items struct {
			Saturation int `json:"saturation"`
			Lightness  int `json:"lightness"`
		} `json:"items"`
	} `json:"colors"`
}

type Data struct {
	Version       string        `json:"version"`
	StaticMode    bool          `json:"staticMode"`
	Customization Customization `json:"customization"`
}

type Filter struct {
	Mode    string `yaml:"mode"`
	Pattern string `yaml:"pattern"`
}

type ContentFilters struct {
	Namespace Filter `yaml:"namespace"`
	Item      Filter `yaml:"item"`
}

type Logging struct {
	Level string `yaml:"level"`
}

// Config represents the overall structure of the YAML file
type Config struct {
	Customization         Customization  `yaml:"customization"`
	Content_filters       ContentFilters `yaml:"content_filters"`
	Allow_skip_tls_verify bool           `yaml:"allow_skip_tls_verify"`
	Logging               Logging        `yaml:"logging"`
}

// Item represents a single item in the YAML structure
type Item struct {
	Name        string `yaml:"name"`
	Namespace   string `yaml:"namespace"`
	Description string `yaml:"description"`
	URL         string `yaml:"url"`
	Icon        string `yaml:"icon"`
}

type StaticItems struct {
	Items []Item `yaml:"items"`
}

// configMu guards the live configuration and the HTTP client built from it: the
// informer and crawl goroutines read them while a configuration reload replaces
// them.
var configMu sync.RWMutex

// contentFilters returns the filters of the live configuration.
func contentFilters() ContentFilters {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.Content_filters
}

// liveHttpClient returns the HTTP client of the live configuration.
func liveHttpClient() *http.Client {
	configMu.RLock()
	defer configMu.RUnlock()
	return httpClient
}

func loadConfig() {
	log.Info("Loading CasaVue configuration")
	// create config folder if not existant
	os.MkdirAll(filepath.Dir(configFilePath), os.ModePerm)

	// create config if not found (first run)
	initConfig()

	mainYAML, itemsYAML, err := readConfigFiles()
	if err != nil {
		log.Warning("Error reading configuration files:", err)
	}
	// what the watcher compares the files against, so that it does not reload what
	// has just been loaded here
	lastMainYAML, lastItemsYAML = mainYAML, itemsYAML

	cfg, err := parseMainConfig(mainYAML)
	if err != nil {
		log.Fatal("Error in '", configFilePath, "': ", err)
	}

	staticItems, err := parseStaticItems(itemsYAML)
	if err != nil {
		log.Fatal("Error in '", itemsFilePath, "': ", err)
	}

	applyConfig(cfg, staticItems)
}

// readConfigFiles reads both configuration files as one unit, so that a change of
// either reloads the configuration. A main.yaml which is not there is legitimate: the
// Helm chart mounts only the items ConfigMap unless a main configuration is set.
func readConfigFiles() (mainYAML []byte, itemsYAML []byte, err error) {
	mainYAML, err = os.ReadFile(configFilePath)
	if os.IsNotExist(err) {
		mainYAML, err = nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	itemsYAML, err = os.ReadFile(itemsFilePath)
	if err != nil {
		return nil, nil, err
	}
	return mainYAML, itemsYAML, nil
}

// parseMainConfig merges a main.yaml over the defaults built into the binary and
// refuses a configuration CasaVue could not run on.
func parseMainConfig(mainYAML []byte) (Config, error) {
	cfg := Config{}
	if err := yaml.Unmarshal([]byte(default_config), &cfg); err != nil {
		return cfg, fmt.Errorf("defaults unpacking error: %w", err)
	}
	if err := yaml.Unmarshal(mainYAML, &cfg); err != nil {
		return cfg, err
	}
	if err := validateMainConfig(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func parseStaticItems(itemsYAML []byte) (StaticItems, error) {
	staticItems := StaticItems{}
	if err := yaml.Unmarshal(itemsYAML, &staticItems); err != nil {
		return staticItems, err
	}
	return staticItems, nil
}

// validateMainConfig rejects what the running app would either crash on or ignore
// silently, so that a bad configuration file never becomes the live one.
func validateMainConfig(cfg Config) error {
	if _, err := parseThemeColor(cfg.Customization.Colors.Theme); err != nil {
		return err
	}
	if _, err := logoSvg(cfg.Customization.Logo); err != nil {
		return fmt.Errorf("customization.logo: %w", err)
	}
	for _, filter := range []Filter{cfg.Content_filters.Namespace, cfg.Content_filters.Item} {
		switch filter.Mode {
		case "include", "exclude", "ingressAnnotation":
		default:
			return fmt.Errorf("unknown filter mode %q, only include, exclude and ingressAnnotation are allowed", filter.Mode)
		}
		if _, err := regexp.Compile(filter.Pattern); err != nil {
			return fmt.Errorf("invalid filter pattern %q: %w", filter.Pattern, err)
		}
	}
	switch cfg.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level %q, only debug, info, warn and error are allowed", cfg.Logging.Level)
	}
	return nil
}

// applyConfig makes a parsed configuration the one the app works with and returns
// the static items which were added or changed. At startup those are crawled later
// on by refreshItems(), which crawls everything.
func applyConfig(cfg Config, staticItems StaticItems) (changedKeys []string) {
	configMu.Lock()
	config = cfg
	httpClient = newHttpClient(cfg.Allow_skip_tls_verify)
	configMu.Unlock()

	applyLogging(cfg.Logging)
	writeFrontendConfig(cfg)

	return readStaticItems(staticItems)
}

func applyLogging(logging Logging) {
	log.Info("Setting log level to: ", logging.Level)
	switch logging.Level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	}
}

// writeFrontendConfig creates the config file the Vue frontend reads.
func writeFrontendConfig(cfg Config) {
	data := Data{
		Version:       version,
		StaticMode:    *staticMode,
		Customization: cfg.Customization,
	}

	jsonFile, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		log.Error("Error creating Vue config file: ", err)
		return
	}
	if err := atomicReplace(compiledVuePath+"/config.json", jsonFile); err != nil {
		log.Error("Error writing Vue config file: ", err)
	}
}

// contents of the configuration files as the watcher last saw them
var lastMainYAML, lastItemsYAML []byte

// watchConfigFiles reloads the configuration when the content of the files changes,
// so a ConfigMap rolled out to Kubernetes is applied without restarting the pod.
// shortcut: the files are polled, because the kubelet updates a mounted ConfigMap by
// swapping a symlink; upgrade to inotify or to a ConfigMap watch if seconds matter.
func watchConfigFiles() {
	for {
		time.Sleep(configReloadCheckInterval)
		reloadChangedConfig()
	}
}

// reloadChangedConfig reloads the configuration when the files differ from what was
// applied last time and reports whether it did.
func reloadChangedConfig() bool {
	mainYAML, itemsYAML, err := readConfigFiles()
	if err != nil {
		// keeping what works is better than dropping the dashboard over an
		// unreadable file
		log.Warning("Configuration files unreadable: ", err)
		return false
	}

	if bytes.Equal(mainYAML, lastMainYAML) && bytes.Equal(itemsYAML, lastItemsYAML) {
		return false
	}
	lastMainYAML, lastItemsYAML = mainYAML, itemsYAML

	log.Info("Configuration change detected, reloading")
	reloadConfig(mainYAML, itemsYAML)
	return true
}

// reloadConfig applies new content of the configuration files to the running app.
// A configuration which does not pass validation is dropped, the one in use stays.
func reloadConfig(mainYAML []byte, itemsYAML []byte) {
	cfg, err := parseMainConfig(mainYAML)
	if err != nil {
		log.Error("'", configFilePath, "' not applied: ", err)
		return
	}
	staticItems, err := parseStaticItems(itemsYAML)
	if err != nil {
		log.Error("'", itemsFilePath, "' not applied: ", err)
		return
	}

	// title and colors are baked into the served frontend files, so they have to be
	// regenerated; everything else is read from the configuration at runtime
	customizationChanged := config.Customization != cfg.Customization

	changedKeys := applyConfig(cfg, staticItems)

	if customizationChanged {
		updateFrontendFiles()
	}

	for _, key := range changedKeys {
		go crawlItem(key)
	}

	// items which only the changed content filters allow or hide
	applyWatchedObjects()
}

// static items applied last time, so a reload writes only the changed ones and
// forgets the ones which disappeared
var appliedStaticItems = make(map[string]Item)

// readStaticItems applies the static entries of the configuration and returns the
// keys which were added or changed, so that their icons get crawled.
func readStaticItems(staticItems StaticItems) (changedKeys []string) {
	filters := contentFilters()
	applied := make(map[string]Item, len(staticItems.Items))

	for _, staticItem := range staticItems.Items {

		// apply filter on namespaces
		if applyFilter(filters.Namespace.Pattern, filters.Namespace.Mode, staticItem.Namespace) {
			log.Debug("Skipping namespace '" + staticItem.Namespace + "' due to pattern")
			continue
		}

		// apply filter on item names
		if applyFilter(filters.Item.Pattern, filters.Item.Mode, staticItem.Name) {
			log.Debug("Skipping item '" + staticItem.Name + "' due to pattern")
			continue
		}

		key := staticItem.Namespace + "/" + staticItem.Name
		applied[key] = staticItem

		// an unchanged entry keeps what the icon crawl found for it
		if previous, ok := appliedStaticItems[key]; ok && previous == staticItem {
			continue
		}

		dashboardItems.write(key, DashEntry{
			Namespace:   staticItem.Namespace,
			Name:        staticItem.Name,
			Description: staticItem.Description,
			URL:         staticItem.URL,
			IconURL:     staticItem.Icon,
			Labels:      make(map[string]string),
		})
		changedKeys = append(changedKeys, key)
		log.Debug("Added static entry: ", staticItem.Name)
	}

	// entries which are gone from items.yaml must not stay on the dashboard.
	// shortcut: an entry of the same namespace/name served by the cluster is
	// removed with it; applyWatchedObjects adds it back right away.
	for key := range appliedStaticItems {
		if _, ok := applied[key]; !ok {
			log.Info("Removing static entry: ", key)
			dashboardItems.delete(key)
		}
	}
	appliedStaticItems = applied

	log.Info("Loaded static entries from configuration.")
	return changedKeys
}

func initConfig() {
	configFiles := []string{configFilePath, itemsFilePath}
	filesContent := []string{default_config, default_static_items}

	for idx, _ := range configFiles {
		_, err := os.Stat(configFiles[idx])
		if !os.IsNotExist(err) {
			log.Info("File '", configFiles[idx], "' exists, skipping file init.")
			continue
		}

		// create config file
		tempConfigFile, err := os.Create(configFiles[idx])
		if err != nil {
			// a mounted ConfigMap is read-only and holds only what it defines;
			// the defaults built into the binary are used for the rest
			log.Warning("Error creating config file:", err)
			continue
		}
		defer tempConfigFile.Close()

		// write default config to file
		_, err2 := tempConfigFile.WriteString(filesContent[idx])
		if err2 != nil {
			log.Fatal("Error writing config to file:", err2)
		}
	}
}
