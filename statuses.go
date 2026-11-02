// active HTTP checks for listed items

package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
)

// checkUrlStatus follows redirects on its own (http.Client, up to 10 of them)
// and reports the final status code. Anything but 200 and 401 is an error.
func checkUrlStatus(url string) (int, error) {
	log.Debug("Checking URL ", url, " for status.")
	resp, err := liveHttpClient().Get(url)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 401 {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, fmt.Errorf("failed to retrieve the content. Status code: %d", resp.StatusCode)
}

// statusCheckHandler reports the status code of one dashboard item. Only URLs
// already known to the dashboard are fetched, so the endpoint cannot be used to
// reach arbitrary hosts.
func statusCheckHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("item")

	dashboardItem, _, found := dashboardItems.read(key)
	if !found || dashboardItem.URL == "" {
		// not a dashboard item, nothing to check
		http.Error(w, "unknown dashboard item", http.StatusNotFound)
		return
	}

	statusCode, err := checkUrlStatus(dashboardItem.URL)
	if err != nil {
		log.Warn("Request to ", dashboardItem.URL, ", ended with status: ", err)
		http.Error(w, fmt.Sprintf("Error making request to %s: %s", dashboardItem.URL, err), http.StatusInternalServerError)
		return
	}

	log.Debug("Request to ", dashboardItem.URL, ", ended with status: ", statusCode)
	fmt.Fprintf(w, "%d", statusCode)
}
