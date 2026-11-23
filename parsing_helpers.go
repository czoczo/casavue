// helper functions for manipulating URLs

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// getSizeFromString reads the size out of an icon attribute like "32x32".
// Only the number before "x" is the size, "32x32" is not 3232.
func getSizeFromString(str string) int {
	match := regexp.MustCompile("[0-9]+x[0-9]+").FindString(str)
	if match == "" {
		return 0
	}
	size, _ := strconv.Atoi(strings.SplitN(match, "x", 2)[0])
	return size
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
