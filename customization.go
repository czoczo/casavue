// logo generation and theme settings for PWA needs

package main

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	cp "github.com/otiai10/copy"
	log "github.com/sirupsen/logrus"
)

// code for updating frontend compiled static files, in order to handle customization
// (color and title) settings.

func updateFrontendFiles() {
	updateManifestJSON()
	updateIndexHTML()

	// has to be after index.html and manifest.json updates, so generated checksums match
	updateServiceWorkerJS()

	generatePWAIcons()

	// if static mode, copy files for docker volume share
	if *staticMode {
		err := cp.Copy(compiledVuePath, "./dist")
		if err != nil {
			log.Error("Error copying Vue dist files: ", err)
			return
		}
	}
}

const (
	pwaIconsFolderPath  = compiledVuePath + "/img/icons"
	sourceLogoPath      = sourceVuePath + "/assets/logo.svg"
	sourceSmallLogoPath = sourceVuePath + "/assets/logo_small.svg"
	manifestJSON        = compiledVuePath + "/manifest.json"
	indexHtmlPath       = compiledVuePath + "/index.html"
	faviconPath         = compiledVuePath + "/favicon.ico"
)

type Manifest struct {
	Name            string `json:"name"`
	ShortName       string `json:"short_name"`
	ThemeColor      string `json:"theme_color"`
	BackgroundColor string `json:"background_color"`
}

func updateManifestJSON() {
	// Read the original manifest.json file
	originalManifest, err := readJSONFile(manifestJSON)
	if err != nil {
		log.Error("Error reading original manifest:", err)
		return
	}

	// JSON object with precedence (overwriting original values)
	overrideObject := map[string]interface{}{
		"name":             config.Customization.Name,
		"short_name":       config.Customization.Name,
		"theme_color":      config.Customization.Colors.Theme,
		"background_color": config.Customization.Colors.Theme,
	}

	// Merge the overrideObject into the originalManifest
	mergedManifest := mergeJSON(originalManifest, overrideObject)

	// Write the merged manifest back to the original file
	err = writeJSONFile(manifestJSON, mergedManifest)
	if err != nil {
		log.Error("Error writing merged manifest: ", err)
		return
	}

	log.Info("manifest.json merged and written successfully!")
}

func updateIndexHTML() {
	// Read the entire content of the file
	content := readStringFile(indexHtmlPath)

	// Perform the replacements
	regexpReplaceString(&content, `<meta name="msapplication-TileColor" content="`, `#[0-9a-fA-F]+`, config.Customization.Colors.Theme, `">`)
	regexpReplaceString(&content, `<link rel="mask-icon" href="img/icons/safari-pinned-tab.svg" color="`, `#[0-9a-fA-F]+`, config.Customization.Colors.Theme, `">`)
	regexpReplaceString(&content, `<meta name="theme-color" content="`, `#[0-9a-fA-F]+`, config.Customization.Colors.Theme, `">`)
	regexpReplaceString(&content, "<title>", `.*?`, config.Customization.Name, "</title>")

	// Write the modified content back to the same file
	writeStringFile(indexHtmlPath, content)
	log.Info("index.html replacement completed successfully!")
}

func updateServiceWorkerJS() {
	inputFiles := []string{"config.json", "index.html", "manifest.json"}

	for _, inputFile := range inputFiles {
		configJsonFilePath := compiledVuePath + "/" + inputFile
		data, err := os.ReadFile(configJsonFilePath)
		if err != nil {
			log.Fatal("Error reading file:", err)
		}

		// Calculate the MD5 sum into a string
		hash := md5.Sum(data)
		md5sum := hex.EncodeToString(hash[:])

		// Define the file path
		inputFilePath := compiledVuePath + "/service-worker.js"

		// Read the entire content of the file
		content := readStringFile(inputFilePath)

		// make replacement
		regexpReplaceString(&content, `url:"`+inputFile+`",revision:"`, `[0-9a-f]{32}`, md5sum, `"`)

		// Write the modified content back to the same file
		writeStringFile(inputFilePath, content)

		log.Info("service-worker.js: replacement for file '" + inputFile + "' completed successfully!")

		// Define the file path
		inputFilePath = compiledVuePath + "/service-worker.js.map"

		// Read the entire content of the file
		content = readStringFile(inputFilePath)

		regexpReplaceString(&content, `"url\\": \\"`+inputFile+`\\",\\n    \\"revision\\": \\"`, `[0-9a-f]{32}`, md5sum, `\\"`)

		// Write the modified content back to the same file
		writeStringFile(inputFilePath, content)
		log.Info("service-worker.js.map: replacement for file '" + inputFile + "' completed successfully!")
	}
}

func extractSize(path string) (int, error) {
	_, filename := filepath.Split(path)

	// Remove extension
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))

	// Extract size
	size := strings.Split(filename, "-")
	sizeNumber := size[len(size)-1]

	// Split 'NxN' format and convert to integer
	n, err := strconv.Atoi(strings.Split(sizeNumber, "x")[0])
	if err != nil {
		return 180, err
	}

	return n, nil
}

func generatePWAIcons() {
	convertSvgToFavicons(sourceSmallLogoPath)
	paths, _ := listLogoFiles()
	for _, path := range paths {
		ext := filepath.Ext(path)
		if ext != ".png" {
			continue
		}
		size, _ := extractSize(path)
		convertSvgToPng(sourceLogoPath, path, size)
	}
}

func regexpReplaceString(s *string, prefix string, swapRegex string, swapValue string, sufix string) {
	// Define regular expressions for the replacements
	re := regexp.MustCompile(prefix + "(" + swapRegex + ")" + sufix)

	// Perform the replacements
	*s = re.ReplaceAllString(*s, prefix+swapValue+sufix)
}
