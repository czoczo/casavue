// watching of the configuration ConfigMaps over the Kubernetes API

package main

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// The ConfigMaps the Helm chart builds the /app/config volume from, and the data keys
// inside them, which are the names of the configuration files. They have to match the
// chart, because the mounted files are what CasaVue runs on when the ConfigMaps cannot
// be watched.
const (
	mainConfigMapName  = "casavue-cfg-main"
	itemsConfigMapName = "casavue-cfg-items"
	mainConfigKey      = "main.yaml"
	itemsConfigKey     = "items.yaml"
)

// namespace file of the mounted ServiceAccount; rest.InClusterConfig does not read it,
// so the namespace of the pod has to be taken from here
const serviceAccountNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// configMapContent keeps the configuration as the ConfigMap watch sees it. Both files
// are always handed on together: the two ConfigMaps arrive as separate events, and half
// a configuration must never become the live one. It starts from the mounted files,
// which is what a ConfigMap this pod does not mount looks like anyway.
type configMapContent struct {
	sync.Mutex
	files map[string][]byte
}

func newConfigMapContent() *configMapContent {
	configReloadMu.Lock()
	defer configReloadMu.Unlock()
	return &configMapContent{files: map[string][]byte{
		mainConfigKey:  lastMainYAML,
		itemsConfigKey: lastItemsYAML,
	}}
}

func (c *configMapContent) set(key string, content []byte) {
	c.Lock()
	c.files[key] = content
	mainYAML, itemsYAML := c.files[mainConfigKey], c.files[itemsConfigKey]
	c.Unlock()

	reloadIfChanged(mainYAML, itemsYAML)
}

// configMapEventHandlers hands the content of one configuration file of a ConfigMap to
// the content of both files.
func configMapEventHandlers(content *configMapContent, key string) cache.ResourceEventHandlerFuncs {
	remember := func(obj interface{}) {
		cfgMap, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return
		}
		// a ConfigMap of the right name without that key is not read empty, which would
		// take the configuration it stands for away from the dashboard
		if file, found := cfgMap.Data[key]; found {
			content.set(key, []byte(file))
		}
	}
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    remember,
		UpdateFunc: func(_, newObj interface{}) { remember(newObj) },
		// a deleted configuration ConfigMap changes nothing: the dashboard keeps running
		// on the configuration it has, as it does for unreadable files
		DeleteFunc: func(interface{}) {},
	}
}

// watchConfigMaps watches the ConfigMaps the /app/config volume is built from, so an
// applied or upgraded configuration arrives in about a second instead of after the
// kubelet volume sync of up to a minute, and reports whether it does. It does not when
// there is no Kubernetes access, no namespace to look in or no right to read ConfigMaps;
// the polling of the configuration files is what carries a change then.
//
// Only one of the two may run: until the kubelet has synced a new ConfigMap into the pod
// (up to a minute), the mounted files hold the configuration which was just replaced, and
// applying them afterwards flips the dashboard back to the old one.
func watchConfigMaps(kconfig *rest.Config) bool {
	namespace := serviceAccountNamespace()
	if namespace == "" {
		log.Debug("Configuration ConfigMaps not watched, no namespace to look in")
		return false
	}

	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		log.Warn("Error creating K8s config for the configuration watch: ", err)
		return false
	}
	// one probe before the watch: without the right to list ConfigMaps the watch would
	// be retried and logged forever, while the file polling works anyway
	if _, err := clientset.CoreV1().ConfigMaps(namespace).List(context.Background(), metav1.ListOptions{Limit: 1}); err != nil {
		log.Info("Configuration ConfigMaps in namespace ", namespace, " cannot be read (", err, "), configuration files are polled instead")
		return false
	}

	content := newConfigMapContent()
	for _, cfgMap := range []struct{ name, key string }{
		{name: mainConfigMapName, key: mainConfigKey},
		{name: itemsConfigMapName, key: itemsConfigKey},
	} {
		watchlist := cache.NewFilteredListWatchFromClient(clientset.CoreV1().RESTClient(), "configmaps", namespace, func(options *metav1.ListOptions) {
			// the two configuration ConfigMaps of this pod, not every ConfigMap
			options.FieldSelector = "metadata.name=" + cfgMap.name
		})
		_, controller := cache.NewInformer(watchlist, &corev1.ConfigMap{}, time.Second*0, configMapEventHandlers(content, cfgMap.key))
		go controller.Run(make(chan struct{}))
	}

	log.Info("Watching the configuration ConfigMaps in namespace ", namespace)
	return true
}

// serviceAccountNamespace is the namespace of the running pod, empty when not running
// in a cluster (a -kubeconfig run keeps polling its configuration files).
func serviceAccountNamespace() string {
	namespace, err := os.ReadFile(serviceAccountNamespaceFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(namespace))
}
