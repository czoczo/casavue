// application starting point

package main

import (
	_ "embed"
	"flag"
	"net/http"
	"path/filepath"

	//	"time"
	"sync"

	"k8s.io/client-go/util/homedir"
)

const (
	configFilePath        = "./config/main.yaml"
	itemsFilePath         = "./config/items.yaml"
	staticFilesPath       = "./frontend"
	generatedAvatarsPath  = "./avatars"
	downloadedAvatarsPath = "./downloadedAvatars"
	staticApiPath         = "/api/v1"
	compiledVuePath       = staticFilesPath + "/dist"
	sourceVuePath         = staticFilesPath + "/src"
)

//go:embed VERSION_APP.txt
var version string

var config Config
var staticItems StaticItems
var staticMode *bool
var wg sync.WaitGroup
var httpClient *http.Client

//= &http.Client{
//	Timeout: 5 * time.Second,
//}

func main() {
	staticMode = flag.Bool("static", false, "Single shot static content dashboard generation.")
	flag.Parse()

	dashboardItems.items = make(map[string]DashEntry)

	// config.go
	loadConfig()

	// icon_crawl.go
	refreshItems()

	var kubeconfigPath string
	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&kubeconfigPath, "kubeconfig", filepath.Join(home, ".kube", "config"), "(Optional) absolute path to the kubeconfig file")
	} else {
		flag.StringVar(&kubeconfigPath, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// customization.go
	updateFrontendFiles()

	if *staticMode {
		return
	}

	// kubernetes.go
	kconfig := getKubeConfig(kubeconfigPath)
	if kconfig == nil {
		return
	}
	go getAndWatchKubernetesIngressItems(kconfig)
	go getAndWatchKubernetesGatewayRoutes(kconfig)

	// httpserver.go
	initHttpServer()
}
