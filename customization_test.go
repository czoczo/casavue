package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func stampOf(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])[:8]
}

func TestRegexpReplaceStringKeepsReplacementLiteral(t *testing.T) {
	s := "<title>Old Title</title>"
	regexpReplaceString(&s, "<title>", "[^<]*", `Home $1 $HOME`, "</title>")
	if s != "<title>Home $1 $HOME</title>" {
		t.Errorf("got %q", s)
	}
}

// A browser keeps the favicon it loaded first under its URL and does not drop it when
// CasaVue rewrites the file, so a regenerated icon has to be asked for under a new URL.
func TestIconURLsAreStampedWithTheirContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/img/icons", 0o755); err != nil {
		t.Fatal(err)
	}
	originalIcons, originalFavicon := pwaIconsFolderPath, faviconPath
	originalIndex, originalManifest := indexHtmlPath, manifestJSON
	defer func() {
		pwaIconsFolderPath, faviconPath = originalIcons, originalFavicon
		indexHtmlPath, manifestJSON = originalIndex, originalManifest
	}()
	pwaIconsFolderPath = dir + "/img/icons"
	faviconPath = dir + "/favicon.ico"
	indexHtmlPath = dir + "/index.html"
	manifestJSON = dir + "/manifest.json"

	writeStringFile(faviconPath, "ico")
	writeStringFile(indexHtmlPath, `<link rel="icon" href="favicon.ico">`+
		`<link rel="icon" type="image/svg+xml" href="img/icons/favicon.svg">`+
		`<link rel="icon" sizes="32x32" href="img/icons/favicon-32x32.png">`)
	writeStringFile(manifestJSON, `{"icons":[{"src":"./img/icons/favicon-32x32.png","sizes":"32x32"}]}`)
	writeStringFile(pwaIconsFolderPath+"/favicon.svg", "svg")
	writeStringFile(pwaIconsFolderPath+"/favicon-32x32.png", "old")

	stampIconURLs()
	index := readStringFile(indexHtmlPath)
	want := `<link rel="icon" href="favicon.ico?v=` + stampOf("ico") + `">` +
		`<link rel="icon" type="image/svg+xml" href="img/icons/favicon.svg?v=` + stampOf("svg") + `">` +
		`<link rel="icon" sizes="32x32" href="img/icons/favicon-32x32.png?v=` + stampOf("old") + `">`
	if index != want {
		t.Errorf("every icon URL has to carry the stamp of its content, got %s", index)
	}
	if manifest := readStringFile(manifestJSON); !strings.Contains(manifest,
		`"./img/icons/favicon-32x32.png?v=`+stampOf("old")+`"`) {
		t.Errorf("the PWA icons of manifest.json have to be stamped too, got %s", manifest)
	}

	// a configuration reload regenerates the icons and stamps the already stamped files again
	writeStringFile(pwaIconsFolderPath+"/favicon-32x32.png", "new")
	stampIconURLs()
	index = readStringFile(indexHtmlPath)
	if !strings.Contains(index, `favicon-32x32.png?v=`+stampOf("new")+`"`) {
		t.Errorf("a regenerated icon has to get a new URL, got %s", index)
	}
	if count := strings.Count(index, "?v="); count != 3 {
		t.Errorf("re-stamping must replace the stamp, not stack one, got %d in %s", count, index)
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
