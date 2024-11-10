// active HTTP checks for listed items

package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	// "time"
)

func checkUrlStatus(url string) (int, error) {
	//client := http.Client{
	//	Timeout: 3 * time.Second,
	//}

	log.Debug("Checking URL ", url, " for status.")
	for {
		resp, err := httpClient.Get(url)
		if err != nil {
			return -1, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 || resp.StatusCode == 401 {
			return resp.StatusCode, nil
		}

		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			return resp.StatusCode, fmt.Errorf("failed to retrieve the content. Status code: %d", resp.StatusCode)
		}

		url = resp.Header.Get("Location")
		if url == "" {
			return -1, fmt.Errorf("redirect location not found")
		}
	}
}

func statusCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the URL from the request path
	extractedUrl := ""

	queryParams, err := url.ParseQuery(r.URL.RawQuery)
	for key, values := range queryParams {
		for _, value := range values {
			if key == "url" {
				extractedUrl = value
			}
		}
	}

	if extractedUrl == "" {
		fmt.Fprintf(w, "%d", 0)
	}

	// Make an HTTP request to the specified URL
	statusCode, err := checkUrlStatus(extractedUrl)
	if err != nil {
		log.Warn("Request to ", extractedUrl, ", ended with status: ", err)
		http.Error(w, fmt.Sprintf("Error making request to %s: %s", extractedUrl, err), http.StatusInternalServerError)
		return
	}

	// Return the status code as the response
	log.Debug("Request to ", extractedUrl, ", ended with status: ", statusCode)
	fmt.Fprintf(w, "%d", statusCode)
}
