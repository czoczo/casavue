package main

import (
	"image/color"
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
