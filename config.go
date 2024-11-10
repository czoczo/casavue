// handling configuration files

package main

import (
	_ "embed"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

//go:embed config/main.yaml
var default_config string

//go:embed config/items.yaml
var default_static_items string

type Customization struct {
	Name   string `json:"name"`
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
}

type StaticItems struct {
	Items []Item `yaml:"items"`
}

func loadConfig() {

	log.Info("Loading CasaVue configuration")
	// create config folder if not existant
	os.MkdirAll(filepath.Dir(configFilePath), os.ModePerm)

	// create config if not found (first run)
	initConfig()

        // load default config values
        err := yaml.Unmarshal([]byte(default_config), &config)
        if err != nil {
                log.Fatal("Error unpacking default config values:", err)
        }

	// read config file
	yfile, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		log.Warning("Error reading config file:", err)
	}

	// unpack config YAML
	err = yaml.Unmarshal(yfile, &config)
	if err != nil {
		log.Fatal("Error unpacking config file:", err)
	}

	// set log level according to config
	log.Info("Setting log level to: ", config.Logging.Level)
	switch config.Logging.Level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.Fatal("Invalid log level")
	}

	// set HTTP TLS verify mode
	initHttpClient(config.Allow_skip_tls_verify)

	// read staticItems file
	yfile, err = ioutil.ReadFile(itemsFilePath)
	if err != nil {
		log.Warning("Error reading staticItems file:", err)
	}

	// unpack staticItems YAML
	err = yaml.Unmarshal(yfile, &staticItems)
	if err != nil {
		log.Fatal("Error unpacking staticFiles file:", err)
	}

	data := Data{
		Version:       version,
		StaticMode:    *staticMode,
		Customization: config.Customization,
	}

	// create config file for Vue frontend
	jsonFile, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		log.Fatal("Error creating Vue config file: ", err)
	}
	_ = ioutil.WriteFile(compiledVuePath+"/config.json", jsonFile, 0644)

	readStaticItems()
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
			log.Fatal("Error creating config file:", err)
		}
		defer tempConfigFile.Close()

		// write default config to file
		_, err2 := tempConfigFile.WriteString(filesContent[idx])
		if err2 != nil {
			log.Fatal("Error writing config to file:", err2)
		}
	}
}

func readStaticItems() {
	// add static entries from config file
	for _, staticItem := range staticItems.Items {

		// apply filter on namespaces
		if applyFilter(config.Content_filters.Namespace.Pattern, config.Content_filters.Namespace.Mode, staticItem.Namespace) {
			log.Debug("Skipping namespace '" + staticItem.Namespace + "' due to pattern")
			continue
		}

		// apply filter on item names
		if applyFilter(config.Content_filters.Item.Pattern, config.Content_filters.Item.Mode, staticItem.Name) {
			log.Debug("Skipping item '" + staticItem.Name + "' due to pattern")
			continue
		}

		dashboardItems.write(staticItem.Name, DashEntry{staticItem.Namespace, staticItem.Description, staticItem.URL, "", "", make(map[string]string)})
		log.Debug("Added static entry: ", staticItem.Name)
	}
	log.Info("Loaded static entries from configuration.")
}
