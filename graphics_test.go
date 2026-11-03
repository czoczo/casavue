package main

import (
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestParseThemeColor(t *testing.T) {
	for value, want := range map[string]color.NRGBA{
		"#e0f3ff": {R: 0xe0, G: 0xf3, B: 0xff, A: 255},
		"#f0a":    {R: 0xff, G: 0x00, B: 0xaa, A: 255},
		"#000000": {R: 0, G: 0, B: 0, A: 255},
	} {
		got, err := parseThemeColor(value)
		if err != nil {
			t.Errorf("%q: %v", value, err)
			continue
		}
		if got != want {
			t.Errorf("%q: parsed to %+v, expected %+v", value, got, want)
		}
	}

	for _, value := range []string{"", "not a color", "#12345", "rgb(1,2,3)"} {
		if _, err := parseThemeColor(value); err == nil {
			t.Errorf("%q must be rejected", value)
		}
	}
}

// The values are what the generated PWA icons are painted with, so changing the
// conversion maths has to be a deliberate decision.
func TestGetContrastColor(t *testing.T) {
	for theme, want := range map[string]string{
		"#e0f3ff": "#00111e",
		"#ffffff": "#0f0f0f",
		"#000000": "#efefef",
		"#aabbcc": "#0c0f14",
		"#123456": "#e5eff9",
		"#808080": "#efefef",
	} {
		if got := getContrastColor(theme); got != want {
			t.Errorf("theme %q: got %q, expected %q", theme, got, want)
		}
	}
}

// The base64 overrides of customization.logo and customization.logo_small.
func TestLogoSvg(t *testing.T) {
	for _, override := range []string{
		base64.StdEncoding.EncodeToString([]byte("<svg/>")),
		// the wrapping a terminal or a YAML block scalar produces
		base64.StdEncoding.EncodeToString([]byte("<svg/>")) + "\n",
	} {
		svg, err := logoSvg(override)
		if err != nil || svg != "<svg/>" {
			t.Errorf("%q: got %q, %v", override, svg, err)
		}
	}

	if svg, err := logoSvg(""); svg != "" || err != nil {
		t.Errorf("empty override means 'not set': got %q, %v", svg, err)
	}

	for _, override := range []string{
		base64.StdEncoding.EncodeToString([]byte("<html/>")), // no SVG document
		"####", // not base64
	} {
		if _, err := logoSvg(override); err == nil {
			t.Errorf("%q must be refused", override)
		}
	}
}

// Which logo the PWA icons are generated from: nothing configured, only the main
// logo, only the small one. The shipped SVGs have to be repainted by the icon
// generation, so their fill color is checked along the way.
func TestLogoSvgs(t *testing.T) {
	b64 := func(svg string) string { return base64.StdEncoding.EncodeToString([]byte(svg)) }

	mainLogo, smallLogo := logoSvgs(Customization{})
	for _, svg := range []string{mainLogo, smallLogo} {
		if !strings.Contains(svg, `fill="`+logoSvgColor+`"`) {
			t.Errorf("shipped logo is not painted with %s, the icons would stay uncolored", logoSvgColor)
		}
	}
	if mainLogo == smallLogo {
		t.Error("logo.svg and logo_small.svg must be different files")
	}

	mainLogo, smallLogo = logoSvgs(Customization{Logo: b64("<svg>main</svg>")})
	if mainLogo != "<svg>main</svg>" || smallLogo != "<svg>main</svg>" {
		t.Errorf("favicons have to be generated from an overriding main logo: %q, %q", mainLogo, smallLogo)
	}

	mainLogo, smallLogo = logoSvgs(Customization{LogoSmall: b64("<svg>small</svg>")})
	if smallLogo != "<svg>small</svg>" {
		t.Errorf("logo_small override ignored: %q", smallLogo)
	}
	if mainLogo == smallLogo || mainLogo == "" {
		t.Errorf("only logo_small is overridden, the main logo has to stay the shipped one: %q", mainLogo)
	}
}

// The base64 blobs of the configuration are only for the icon generation and must
// not end up in the config file the frontend gets.
func TestLogoOverridesAreNotSentToFrontend(t *testing.T) {
	cfg := mustParseMainConfig(t, "customization:\n  logo: \""+base64.StdEncoding.EncodeToString([]byte("<svg/>"))+"\"\n")

	frontendConfig, err := json.Marshal(Data{Customization: cfg.Customization})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(frontendConfig), "logo") {
		t.Errorf("logo override leaked into the frontend config: %s", frontendConfig)
	}
}

// An overriding logo is reduced by 30% before its icons are generated out of it, the
// logo of CasaVue is not.
func TestShrinkLogoSvg(t *testing.T) {
	for _, tc := range []struct{ name, svg, expected string }{
		{"viewBox of the shipped logo", `<svg viewBox="0.00 0.00 637.00 637.00"><path/></svg>`, `<svg viewBox="0.00 0.00 637.00 637.00"><g transform="translate(95.55 95.55) scale(0.7 0.7)"><path/></g></svg>`},
		{"origin not at zero", `<svg viewBox="10 20 100 50"><path/></svg>`, `<svg viewBox="10 20 100 50"><g transform="translate(18 13.5) scale(0.7 0.7)"><path/></g></svg>`},
		{"comma separated", `<svg viewBox="0,0,100,100"><path/></svg>`, `<svg viewBox="0,0,100,100"><g transform="translate(15 15) scale(0.7 0.7)"><path/></g></svg>`},
		{"no viewBox to keep the logo centered with", `<svg><path/></svg>`, `<svg><path/></svg>`},
		{"viewBox which is not four numbers", `<svg viewBox="wide"><path/></svg>`, `<svg viewBox="wide"><path/></svg>`},
		{"empty canvas", `<svg viewBox="0 0 0 0"><path/></svg>`, `<svg viewBox="0 0 0 0"><path/></svg>`},
		{"self-closing root tag", `<svg viewBox="0 0 100 100"/>`, `<svg viewBox="0 0 100 100"/>`},
	} {
		if got := shrinkLogoSvg(tc.svg); got != tc.expected {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.expected)
		}
	}

	// the scaled group has to reach the rasterization too
	mainLogo, shrunk := readSvgFile(sourceLogoPath), shrinkLogoSvg(readSvgFile(sourceLogoPath))
	config.Customization.Colors.Theme = "#deaded"
	full, reduced := drawnWidth(svgToRaster(mainLogo, 200)), drawnWidth(svgToRaster(shrunk, 200))
	if ratio := float64(reduced) / float64(full); ratio < 0.63 || ratio > 0.77 {
		t.Errorf("an overriding logo is not drawn 30 percent smaller: %d pixels instead of %d", reduced, full)
	}
}

// drawnWidth is the width of what an image has drawn on the transparent background of
// an SVG, measured on the dark logo of CasaVue.
func drawnWidth(img image.Image) int {
	side := img.Bounds().Dx()
	first, last := side, -1
	for x := 0; x < side; x++ {
		for y := 0; y < side; y++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 6553 {
				first, last = min(first, x), max(last, x)
				break
			}
		}
	}
	return last - first + 1
}
