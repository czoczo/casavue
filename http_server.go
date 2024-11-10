// HTTP server code

package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func initHttpServer() {
	// Define a handler function to handle HTTP requests

	http.HandleFunc("/api/v1", entriesApiHandler)

	// avatars.go
	http.HandleFunc("/avatars/", avatarHandler)

	// statuses.go
	http.HandleFunc("/statusCheck/", statusCheckHandler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve the file using the file server
		log.Info("Filesystem request: ", r.Method, " ", r.URL.Path)
		http.FileServer(http.Dir(compiledVuePath)).ServeHTTP(w, r)
	})

	// Specify the port to listen on
	port := 8080
	addr := fmt.Sprintf(":%d", port)

	// Start the HTTP server
	log.Info("Server is running on ", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("Error starting server: ", err)
	}
}

func entriesApiHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("Api request: ", r.Method, " ", r.URL.Path)
	// Set the content type to JSON
	w.Header().Set("Content-Type", "application/json")

	// Marshal the dashboardItems to JSON
	jsonData, err := json.Marshal(dashboardItems.get())
	if err != nil {
		http.Error(w, "Error marshalling JSON", http.StatusInternalServerError)
		return
	}

	// Write the JSON data to the response writer
	w.Write(jsonData)
}
