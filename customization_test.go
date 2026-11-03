package main

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestRegexpReplaceStringKeepsReplacementLiteral(t *testing.T) {
	s := "<title>Old Title</title>"
	regexpReplaceString(&s, "<title>", "[^<]*", `Home $1 $HOME`, "</title>")
	if s != "<title>Home $1 $HOME</title>" {
		t.Errorf("got %q", s)
	}
}

// The header of the dashboard draws an overriding customization.logo, so the logo has
// to be served next to index.html - reduced like the icons are not, the header keeps
// its own padding.
func TestOverriddenLogoIsServedToTheHeader(t *testing.T) {
	original := logoSvgPath
	logoSvgPath = t.TempDir() + "/logo.svg"
	defer func() { logoSvgPath = original }()

	logo := `<svg viewBox="0 0 100 100"><path/></svg>`
	writeOverriddenLogo(Customization{Logo: base64.StdEncoding.EncodeToString([]byte(logo))})
	if written, err := os.ReadFile(logoSvgPath); err != nil {
		t.Errorf("an overriding logo has to be served: %v", err)
	} else if string(written) != logo {
		t.Errorf("the header gets the logo as it was configured, got %s", written)
	}

	writeOverriddenLogo(Customization{})
	if _, err := os.Stat(logoSvgPath); !os.IsNotExist(err) {
		t.Errorf("an override which is gone again must not stay served, got %v", err)
	}
}
