// functions for text and JSON file operations

package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
)

// ReadJSONFile reads a JSON file and returns its content as a map[string]interface{}
func readJSONFile(filename string) (map[string]interface{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// MergeJSON merges two JSON objects (maps) with precedence
func mergeJSON(original, override map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})
	for key, value := range original {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

// WriteJSONFile writes a map to a JSON file
func writeJSONFile(filename string, data map[string]interface{}) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, bytes, 0644)
}

func readStringFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatal("Error reading the file:", err)
	}
	return string(content)
}

func writeStringFile(path string, content string) {
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		log.Fatal("Error writing to the file:", err)
	}
}
