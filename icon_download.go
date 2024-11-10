// implements algorithm for obtaining icons from found icon URLs
// currently used only in static mode
// https://casavue.app/deployment/deploy_docker/#static-mode

package main

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"crypto/tls"
	"os"
	"path/filepath"
	"regexp"
	"time"
	"strings"
)

func initHttpClient(tlsSkipVerify bool) {
	if tlsSkipVerify {
        log.Warn("Disabling TLS checks")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: tlsSkipVerify},
	}
	httpClient = &http.Client{
		Transport: tr,
		Timeout: 30 * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
}

func downloadIcon(fullURLFile string) string {

	// create avatars folder if not existant
	log.Debug("Creating folder if non-existent: ", compiledVuePath+"/"+generatedAvatarsPath)
	os.MkdirAll(compiledVuePath+"/"+generatedAvatarsPath, os.ModePerm)

	// return if avatar generated locally
	if strings.HasPrefix(fullURLFile, generatedAvatarsPath) {
		return fullURLFile
	}

	// save to file if SVF data in string
	if strings.HasPrefix(fullURLFile, "data:image/svg+xml,") {
		// build avatar filename and path
		fileName := downloadedAvatarsPath + "/" + strToSha256(fullURLFile) + ".svg"
		writeStringFile(compiledVuePath+"/"+fileName, fullURLFile[20:])
		return fileName
	}

	// Validate URL
	_, err := url.Parse(fullURLFile)
	if err != nil {
		log.Error("Error validating URL: ", err)
	}

	// Build filename
	extension := filepath.Ext(fullURLFile)
	extension = strings.Split(extension, "?")[0]

	// create downloadedAvatars folder if not existant
	log.Debug("Creating folder if non-existent: ", compiledVuePath+"/"+downloadedAvatarsPath)
	os.MkdirAll(compiledVuePath+"/"+downloadedAvatarsPath, os.ModePerm)

	// build avatar filename and path
	fileName := downloadedAvatarsPath + "/" + strToSha256(fullURLFile) + extension

	// Create a blank file
	file, err := os.Create(compiledVuePath + "/" + fileName)
	if err != nil {
		log.Fatal("Error creating file: ", err)
	}
	defer file.Close()

	//client := http.Client{
	//	CheckRedirect: func(r *http.Request, via []*http.Request) error {
	//		r.URL.Opaque = r.URL.Path
	//		return nil
	//	},
	//}

	// sanitize URL
	domain := strings.Join(strings.Split(fullURLFile, "/")[:3], "/")
	endpoint := strings.Join(strings.Split(fullURLFile, "/")[3:], "/")
	re := regexp.MustCompile(`^(\.+/)+`)
	endpoint = re.ReplaceAllString(endpoint, "")
	downloadURL := domain + "/" + endpoint

	// Download content and save to file
	log.Debug("Downloading icon from URL: ", downloadURL)
	resp, err := httpClient.Get(downloadURL)
	if err != nil {
		log.Error("Error downloading content: ", err)
	}
	defer resp.Body.Close()

	size, err := io.Copy(file, resp.Body)
	log.Debug("Downloaded file %s with size %d\n", compiledVuePath+"/"+fileName, size)
	return fileName
}
