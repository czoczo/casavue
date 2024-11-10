// helper functions for manipulating URLs

package main

import (
	"crypto/sha256"
	"encoding/hex"
	log "github.com/sirupsen/logrus"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var nonAlphanumericRegex = regexp.MustCompile(`[^0-9]+`)

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func getSizeFromString(str string) int {
	re := regexp.MustCompile("[0-9]+x[0-9]+")

	// Extracting numbers from the first string
	match := clearString(re.FindString(str))
	size, _ := strconv.Atoi(match)
	return size
}

func getHostFromURL(inputUrl string) string {
	url, err := url.Parse(inputUrl)
	if err != nil {
		log.Error(err)
		return ""
	}

	return url.Host
}

func extractContent(input string) string {
	// Split the input string by slashes
	parts := strings.Split(input, "/")

	// return if there are no additional slashes in endpoint
	if len(parts) < 4 {
		return input
	}

	// Join the parts up to the last slash (inclusive)
	result := strings.Join(parts[:len(parts)-1], "/") + "/"

	return result
}

func extractBeforeHash(input string) string {
	if idx := strings.IndexByte(input, '#'); idx >= 0 {
		return input[:idx]
	}
	return input
}

func addPrefix(fqdn string, endpoint string) string {
	fqdn = extractContent(fqdn)
	fqdn = extractBeforeHash(fqdn)

	if strings.HasPrefix(endpoint, "https") {
		return endpoint
	}

	// get hostname from FQDN
	u, err := url.Parse(fqdn)
	if err != nil {
		log.Fatal(err)
	}

	if !strings.HasPrefix(endpoint, "/") {
		return strings.TrimRight(fqdn, "/") + "/" + endpoint
	}
	return u.Scheme + "://" + u.Hostname() + "/" + strings.TrimLeft(endpoint, "/")
}

func strToSha256(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	checksum := hasher.Sum(nil)

	// Convert the checksum to a hexadecimal string
	return hex.EncodeToString(checksum)
}

func applyFilter(pattern string, mode string, str string) bool {
	var exclude bool
	if mode == "exclude" {
		exclude = true
	} else if mode == "include" {
		exclude = false
	} else if mode == "ingressAnnotation" {
		return false
	} else {
		log.Fatal("Error parsing configuration. Only 'exclude', 'include' and 'ingressAnnotation' values are allowed as filter mode.")
	}

	matched, err := regexp.MatchString(pattern, str)
	if err != nil {
		log.Fatal("Parsing regular expression error:", err)
	}

	if exclude {
		// when excluding, return true on match
		return matched
	} else {
		// when including, return false on match
		return !matched
	}
}

// IsDigit returns true if the rune is a digit.
func IsDigit(r rune) bool {
	return unicode.IsDigit(r)
}

// RemoveTrailingDigits removes trailing digits from the input string.
func RemoveTrailingDigits(s string) string {
	return strings.TrimRightFunc(s, IsDigit)
}

func firstWord(input string) string {
	// Split the input string by spaces
	input = strings.ToLower(input)
	words := strings.Fields(input)

	// Check if there are no spaces
	if len(words) <= 1 {
		// If no spaces, return the original string
		return input
	}

	// If there are spaces, return only the first word
	return words[0]
}
