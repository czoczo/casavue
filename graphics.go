// logo generation for PWA needs

package main

import (
	"bufio"
	"fmt"
	"github.com/PerformLine/go-stockutil/colorutil"
	ico "github.com/biessek/golang-ico"
	"github.com/disintegration/imaging"
	log "github.com/sirupsen/logrus"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// getContrastColor takes a color in hexadecimal format (e.g. "#aabbcc") and
// returns a contrasting color in RGB hexadecimal format.
func getContrastColor(inputColor string) string {
	// Parse the input color
	baseColor, err := colorutil.Parse(inputColor)
	if err != nil {
		// Handle parsing error
		log.Fatal("Error parsing input color:", err)
		return ""
	}

	// Convert to HSL
	h, s, l := baseColor.HSL()

	// Adjust lightness based on the condition
	if l > 0.5 {
		l = 0.06
	} else {
		l = 0.94
	}

	// Convert back to RGB
	r, g, b := colorutil.HslToRgb(h, s, l)
	adjustedColor := fmt.Sprintf("#%02x%02x%02x", r, g, b)

	return adjustedColor
}

func openSvgToString(svgPath string) (string, error) {
	// Open SVG file
	svgFile, err := os.Open(svgPath)
	if err != nil {
		return "", err
	}
	defer svgFile.Close()

	// Read the SVG content
	scanner := bufio.NewScanner(svgFile)
	var svgContent string
	for scanner.Scan() {
		svgContent += scanner.Text() + "\n"
	}

	return svgContent, nil
}

func convertSvgToFavicons(svgPath string) error {
	svgContent, _ := openSvgToString(svgPath)

	// Replace 'fill="#e0f3ff"' with the value from config.Customization.Colors.Contrast
	themeColor := config.Customization.Colors.Theme
	contrastColor := getContrastColor(themeColor)
	svgContent = strings.ReplaceAll(string(svgContent), `fill="#e0f3ff"`, fmt.Sprintf(`fill="%s"`, contrastColor))

	// Save the modified SVG content to a new file at 'dist/img/icons/safari-pinned-tab.svg'
	err := ioutil.WriteFile(pwaIconsFolderPath+"/safari-pinned-tab.svg", []byte(svgContent), 0644)
	if err != nil {
		return fmt.Errorf("error saving modified SVG: %v", err)
	}

	// Replace '<path' with '<rect width="100%" height="100%" fill="red"/>\n<path'
	svgContent = strings.ReplaceAll(svgContent, `<path`, `<rect width="100%" height="100%" rx="80" ry="80" fill="`+themeColor+`"/>\n<path`)

	// Save the modified SVG content to a new file at 'dist/img/favicon.svg'
	err = ioutil.WriteFile(pwaIconsFolderPath+"/favicon.svg", []byte(svgContent), 0644)
	if err != nil {
		return fmt.Errorf("error saving modified SVG: %v", err)
	}

	img := svgToRaster(svgContent, 32)

	file, err := os.Create(faviconPath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return err
	}
	defer file.Close() // Close the file when done

	ico.Encode(file, img)

	return nil
}

func convertSvgToPng(svgPath string, pngPath string, sideLength int) error {
	svgContent, _ := openSvgToString(svgPath)

	// Replace color occurrences
	replacedContent := strings.Replace(svgContent, "#e0f3ff", getContrastColor(config.Customization.Colors.Theme), -1)
	img := svgToRaster(replacedContent, sideLength)

	// Resize the image to add padding (15% of side length)
	resizedImg := imaging.Resize(img, sideLength*70/100, sideLength*70/100, imaging.Lanczos)

	// Set the background color
	canvas := image.NewRGBA(image.Rect(0, 0, sideLength, sideLength))
	bgColor := colorutil.MustParse(config.Customization.Colors.Theme)
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Center the image on a new square canvas (with the original side length)
	square := imaging.PasteCenter(canvas, resizedImg)

	// Create the output file
	outputFile, err := os.Create(pngPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Encode the image to PNG
	return png.Encode(outputFile, square)
}

func svgToRaster(svgContent string, sideLength int) image.Image {
	// Decode SVG
	icon, _ := oksvg.ReadIconStream(strings.NewReader(svgContent))
	icon.SetTarget(0, 0, float64(sideLength), float64(sideLength))

	// Create a new RGBA image
	img := image.NewRGBA(image.Rect(0, 0, sideLength, sideLength))

	// Set the background color
	bgColor := colorutil.MustParse(config.Customization.Colors.Theme)
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// draw icon
	icon.Draw(rasterx.NewDasher(sideLength, sideLength, rasterx.NewScannerGV(sideLength, sideLength, img, img.Bounds())), 1)
	return img
}

func listLogoFiles() ([]string, error) {
	var logoFiles []string

	// Walk through the directory
	err := filepath.Walk(pwaIconsFolderPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the file is a regular file and has a .png or .svg extension
		if !info.IsDir() && (strings.HasSuffix(info.Name(), ".png") || strings.HasSuffix(info.Name(), ".svg")) {
			logoFiles = append(logoFiles, filePath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return logoFiles, nil
}
