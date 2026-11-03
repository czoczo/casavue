// logo generation for PWA needs

package main

import (
	"encoding/base64"
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
	"regexp"
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

// logoSvgColor is the fill color the shipped logo SVGs are drawn with; the icon
// generation swaps it for the contrast color of the theme. An overriding logo has to
// use it as well to get recolored.
const logoSvgColor = "#000000"

// logoSvg decodes an SVG logo override from the configuration. An empty override
// means "nothing configured". The wrapped base64 which terminals and YAML block
// scalars produce is accepted.
func logoSvg(override string) (string, error) {
	if strings.TrimSpace(override) == "" {
		return "", nil
	}
	svg, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(override), ""))
	if err != nil {
		return "", fmt.Errorf("not valid base64: %w", err)
	}
	if !strings.Contains(string(svg), "<svg") {
		return "", fmt.Errorf("decoded content is not an SVG document")
	}
	return string(svg), nil
}

// logoScale is how much of its own canvas an overriding logo keeps of it before the
// icons are generated out of it: an own SVG usually fills the canvas edge to edge.
const logoScale = 0.7

var (
	viewBoxRegex = regexp.MustCompile(`viewBox="([^"]*)"`)
	svgTagRegex  = regexp.MustCompile(`(?s)<svg\b[^>]*>`)
)

// shrinkLogoSvg reduces an overriding logo by 30% before the icons are generated out
// of it. Its content gets wrapped in one scaled <g> instead of the paths being
// rewritten, so colors, gradients and the repainting of the icon generation keep
// working. The viewBox is not used for the reduction either: an icon drawn in a
// viewBox with a negative origin ends up off the canvas of the raster graphics.
// shortcut: an SVG without a viewBox (or with a viewBox which is not four numbers)
// keeps its size, as there is no canvas to keep the logo centered in; upgrade to
// transforming the paths if such logos have to be reduced as well.
func shrinkLogoSvg(svg string) string {
	dims := viewBoxRegex.FindStringSubmatchIndex(svg)
	if dims == nil {
		return svg
	}

	canvas := make([]float64, 4)
	numbers := strings.Fields(strings.ReplaceAll(svg[dims[2]:dims[3]], ",", " "))
	if len(numbers) != 4 {
		return svg
	}
	for idx, number := range numbers {
		value, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return svg
		}
		canvas[idx] = value
	}
	if canvas[2] <= 0 || canvas[3] <= 0 {
		return svg
	}

	// the root tag has to stay outside the scaled group, so the background of the
	// favicons is not scaled along with the logo
	start := svgTagRegex.FindStringIndex(svg)
	closing := strings.LastIndex(svg, "</svg>")
	if start == nil || closing < start[1] {
		return svg // a self-closing root tag: nothing to wrap
	}

	// the center of the canvas is the one point which stays where it is while scaling
	scaled := fmt.Sprintf(`<g transform="translate(%v %v) scale(%v %v)">`,
		svgNumber((canvas[0]+canvas[2]/2)*(1-logoScale)), svgNumber((canvas[1]+canvas[3]/2)*(1-logoScale)),
		svgNumber(logoScale), svgNumber(logoScale))

	return svg[:start[1]] + scaled + svg[start[1]:closing] + "</g>" + svg[closing:]
}

// svgNumber writes a value of a generated SVG without exponent and without trailing
// zeros, so the SVG stays readable.
func svgNumber(value float64) string {
	return strconv.FormatFloat(roundToPlaces(value, 3), 'f', -1, 64)
}

func configuredLogoSvg(override string) string {
	svg, err := logoSvg(override)
	if err != nil {
		log.Error("Ignoring logo override: ", err)
	}
	return svg
}

func readSvgFile(svgPath string) string {
	svg, err := os.ReadFile(svgPath)
	if err != nil {
		log.Error("Error reading ", svgPath, ": ", err)
	}
	return string(svg)
}

// logoSvgs returns the SVG content the PWA icons are generated from: a configured
// base64 override if there is one, the SVG file of CasaVue elsewhere. Only an
// override gets reduced by 30% first, the logo of CasaVue keeps the size it always
// had.
func logoSvgs(customization Customization) (mainLogo string, smallLogo string) {
	mainLogo = shrinkLogoSvg(configuredLogoSvg(customization.Logo))
	smallLogo = shrinkLogoSvg(configuredLogoSvg(customization.LogoSmall))

	// only the main logo overridden: the favicons are generated from it as well
	if smallLogo == "" && mainLogo != "" {
		smallLogo = mainLogo
	}
	if mainLogo == "" {
		mainLogo = readSvgFile(sourceLogoPath)
	}
	if smallLogo == "" {
		smallLogo = readSvgFile(sourceSmallLogoPath)
	}
	return mainLogo, smallLogo
}

func convertSvgToFavicons(svgContent string) error {
	themeColor := config.Customization.Colors.Theme
	contrastColor := getContrastColor(themeColor)
	svgContent = strings.ReplaceAll(svgContent, `fill="`+logoSvgColor+`"`, fmt.Sprintf(`fill="%s"`, contrastColor))

	// Save the modified SVG content to a new file at 'dist/img/icons/safari-pinned-tab.svg'
	err := ioutil.WriteFile(pwaIconsFolderPath+"/safari-pinned-tab.svg", []byte(svgContent), 0644)
	if err != nil {
		return fmt.Errorf("error saving modified SVG: %v", err)
	}

	// The background of the favicon goes right inside the root tag, so that it keeps
	// covering the whole icon also for a logo which is scaled down in a <g>
	if start := svgTagRegex.FindStringIndex(svgContent); start != nil {
		svgContent = svgContent[:start[1]] + `<rect width="100%" height="100%" rx="80" ry="80" fill="` + themeColor + `"/>\n` + svgContent[start[1]:]
	}

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

func convertSvgToPng(svgContent string, pngPath string, sideLength int) error {
	// Replace color occurrences
	replacedContent := strings.Replace(svgContent, logoSvgColor, getContrastColor(config.Customization.Colors.Theme), -1)
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
