// logo generation for PWA needs

package main

import (
	"bufio"
	"fmt"
	ico "github.com/biessek/golang-ico"
	"github.com/disintegration/imaging"
	log "github.com/sirupsen/logrus"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// parseThemeColor reads the configured theme color, "#rrggbb" or "#rgb".
// shortcut: hex only, which is what the configuration documents; upgrade to a
// full CSS color parser if named or rgb() values are to be accepted.
func parseThemeColor(value string) (color.NRGBA, error) {
	hexColor := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(hexColor) == 3 {
		// "#f0a" means "#ff00aa"
		hexColor = strings.Repeat(hexColor[0:1], 2) + strings.Repeat(hexColor[1:2], 2) + strings.Repeat(hexColor[2:3], 2)
	}
	if len(hexColor) != 6 {
		return color.NRGBA{}, fmt.Errorf("not a color: %q", value)
	}
	rgb, err := strconv.ParseUint(hexColor, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("not a color: %q", value)
	}
	return color.NRGBA{
		R: uint8(rgb >> 16),
		G: uint8(rgb >> 8),
		B: uint8(rgb),
		A: 255,
	}, nil
}

func roundToPlaces(x float64, places int) float64 {
	multi := math.Pow(10, float64(places))
	return math.Round(x*multi) / multi
}

// rgbToHsl converts red, green and blue in [0,1] to hue in degrees and
// saturation and lightness in [0,1]. Values are rounded to 2 places, as the
// generated logo colors depend on it.
func rgbToHsl(r, g, b float64) (float64, float64, float64) {
	max := math.Max(math.Max(r, g), b)
	min := math.Min(math.Min(r, g), b)
	chroma := max - min

	hue := 0.0
	if chroma != 0 {
		switch {
		case r == max:
			hue = math.Mod((g-b)/chroma, 6)
		case g == max:
			hue = (b-r)/chroma + 2
		default:
			hue = (r-g)/chroma + 4
		}
		hue *= 60
	}

	lightness := (max + min) / 2
	saturation := 0.0
	if lightness != 1 {
		saturation = chroma / (1 - math.Abs(2*lightness-1))
	}
	if math.IsNaN(saturation) {
		// black and white: chroma 0 over 0
		saturation = 0
	}

	const rounding = 2
	hue = roundToPlaces(hue, rounding)
	saturation = roundToPlaces(saturation, rounding)
	lightness = roundToPlaces(lightness, rounding)
	if hue < 0 {
		hue = 360 + hue
	}
	return hue, saturation, lightness
}

// hslToRgb converts hue in degrees plus saturation and lightness in [0,1] back
// to red, green and blue.
func hslToRgb(hue, saturation, lightness float64) (uint8, uint8, uint8) {
	hue = math.Mod(hue, 360)
	var r, g, b float64
	if saturation == 0 {
		r, g, b = lightness, lightness, lightness
	} else {
		chroma := (1 - math.Abs(2*lightness-1)) * saturation
		sector := hue / 60
		intermediate := chroma * (1 - math.Abs(math.Mod(sector, 2)-1))
		switch {
		case sector <= 1:
			r, g, b = chroma, intermediate, 0
		case sector <= 2:
			r, g, b = intermediate, chroma, 0
		case sector <= 3:
			r, g, b = 0, chroma, intermediate
		case sector <= 4:
			r, g, b = 0, intermediate, chroma
		case sector <= 5:
			r, g, b = intermediate, 0, chroma
		default:
			r, g, b = chroma, 0, intermediate
		}
		offset := lightness - chroma/2
		r += offset
		g += offset
		b += offset
	}
	return uint8(roundToPlaces(r, 2) * 255), uint8(roundToPlaces(g, 2) * 255), uint8(roundToPlaces(b, 2) * 255)
}

// getContrastColor takes a color in hexadecimal format (e.g. "#aabbcc") and
// returns a contrasting color in RGB hexadecimal format.
func getContrastColor(inputColor string) string {
	// Parse the input color
	baseColor, err := parseThemeColor(inputColor)
	if err != nil {
		// Handle parsing error
		log.Fatal("Error parsing input color:", err)
		return ""
	}

	// Convert to HSL
	h, s, l := rgbToHsl(float64(baseColor.R)/255, float64(baseColor.G)/255, float64(baseColor.B)/255)

	// Adjust lightness based on the condition
	if l > 0.5 {
		l = 0.06
	} else {
		l = 0.94
	}

	// Convert back to RGB
	r, g, b := hslToRgb(h, s, l)
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
	bgColor, err := parseThemeColor(config.Customization.Colors.Theme)
	if err != nil {
		return err
	}
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
	bgColor, err := parseThemeColor(config.Customization.Colors.Theme)
	if err != nil {
		log.Error(err)
		return img
	}
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
