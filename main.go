// application starting point

package main

import (
	_ "embed"
	"flag"
	"net/http"
	"path/filepath"

	//	"time"
	"sync"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
)

// the configuration files, variables so that a test can point them at its own files
var (
	configFilePath = "./config/main.yaml"
	itemsFilePath  = "./config/items.yaml"
)

const (
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
	dashboardItems.versions = make(map[string]int64)

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
	// without any Kubernetes access the items of the config files are still
	// served, otherwise the container would exit and show nothing
	kconfig := getKubeConfig(kubeconfigPath)
	if kconfig == nil {
		log.Warn("No Kubernetes configuration, serving only the items from the config files")
	} else {
		go getAndWatchKubernetesIngressItems(kconfig)
		go getAndWatchKubernetesGatewayRoutes(kconfig)
	}

	// a new ConfigMap of the main config or of the items is applied without restarting
	// the pod
	startConfigurationReload(kconfig)

	// httpserver.go
	initHttpServer()
}

// startConfigurationReload runs the one source of a new configuration: the watch of the
// configuration ConfigMaps when they can be watched, the polling of the mounted
// configuration files otherwise. Never both, because the mounted files hold the old
// ConfigMap until the kubelet has synced the new one into the pod (up to a minute), so a
// poller next to the watch applies the replaced configuration again every 5 seconds.
func startConfigurationReload(kconfig *rest.Config) {
	if kconfig != nil && watchConfigMaps(kconfig) {
		return
	}
	go watchConfigFiles()
}
