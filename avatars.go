// handling avatars generation for entries with no icon found

package main

import (
	"bytes"
	"github.com/aofei/cameron"
	log "github.com/sirupsen/logrus"
	"image/png"
	"net/http"
	"os"
	"strings"
)

func getGeneratedIcon(entry *DashEntry, name string) {
	log.Debug("Generating avatar for: ", name)
	if entry.IconURL != "" {
		return
	}

	if *staticMode {
		avatarsDir := compiledVuePath + "/" + generatedAvatarsPath
		if err := os.MkdirAll(avatarsDir, os.ModePerm); err != nil {
			log.Error("Error creating avatars folder '", avatarsDir, "': ", err)
			return
		}
		filename := strToSha256(name) + ".png"
		file, err := os.Create(avatarsDir + "/" + filename)
		if err != nil {
			log.Error("Error creating avatar file: ", err)
			return
		}
		defer file.Close()
		if err := png.Encode(file, cameron.Identicon([]byte(name), 540, 60)); err != nil {
			log.Error("Error encoding avatar: ", err)
			return
		}

		entry.IconURL = "./avatars/" + filename
		return
	}

	entry.IconURL = "/avatars/" + strings.TrimSpace(name)
	return
}

func avatarHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("Avatar request: ", r.Method, " ", r.URL.Path)
	name := strings.TrimPrefix(r.URL.Path, "/avatars/")

	// Use Cameron to generate avatars
	buf := bytes.Buffer{}
	png.Encode(&buf, cameron.Identicon([]byte(name), 540, 60))
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	buf.WriteTo(w)
}
