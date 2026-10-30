// Kubernetes integration reading Ingress and Gateway API HTTPRoute resources

package main

import (
	"os"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/networking/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayversioned "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

func getKubeConfig(kubeconfigPath string) *rest.Config {
	// creates the in-cluster config
	kconfig, err := rest.InClusterConfig()
	if err != nil {
		log.Warn("Error creating K8s in-cluster config: ", err)
	}

	if _, err := os.Stat(kubeconfigPath); err == nil {
		kconfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			log.Warn("Error building K8s config form flags: ", err)
		}
	}
	return kconfig
}

func processAnnotations(annotations map[string]string) (description, nameOverride, iconURL, urlOverride string) {
	if val, ok := annotations["casavue.app/description"]; ok {
		log.Debug("Found description: ", val)
		description = val
	}
	if val, ok := annotations["casavue.app/name"]; ok {
		log.Debug("Found name override: ", val)
		nameOverride = val
	}
	if val, ok := annotations["casavue.app/icon"]; ok {
		log.Debug("Found icon URL override: ", val)
		iconURL = val
	}
	if val, ok := annotations["casavue.app/url"]; ok {
		log.Debug("Found URL override: ", val)
		urlOverride = val
	}
	return
}

// storeKey is "namespace/name" of the cluster object. It never changes, so
// overriding the displayed name or moving an annotation cannot leave an item
// behind under the old key, and identically named Ingresses in different
// namespaces do not overwrite each other.
func storeKey(obj metav1.Object) string {
	return obj.GetNamespace() + "/" + obj.GetName()
}

// displayName is what the dashboard shows for an item.
func displayName(obj metav1.Object, nameOverride string) string {
	if nameOverride != "" {
		return nameOverride
	}
	if label, ok := obj.GetLabels()["app.kubernetes.io/name"]; ok && label != "" {
		return label
	}
	return obj.GetName()
}

// buildEntry turns a cluster object into a dashboard entry. It reports false
// when no usable URL can be built out of it.
func buildEntry(obj metav1.Object, host, protocol string) (DashEntry, bool) {
	description, nameOverride, iconURL, urlOverride := processAnnotations(obj.GetAnnotations())

	URL := urlOverride
	if URL == "" {
		if host == "" {
			log.Debug("Skipping '", obj.GetName(), "': no host and no URL override.")
			return DashEntry{}, false
		}
		URL = protocol + host
	}

	return DashEntry{
		Namespace:   obj.GetNamespace(),
		Name:        displayName(obj, nameOverride),
		Description: description,
		URL:         URL,
		IconURL:     iconURL,
		Labels:      obj.GetLabels(),
	}, true
}

func dashEntryFromIngress(obj interface{}) (DashEntry, bool) {
	it, ok := obj.(*v1.Ingress)
	if !ok {
		return DashEntry{}, false
	}

	protocol := "http://"
	if len(it.Spec.TLS) > 0 {
		protocol = "https://"
	}
	// an Ingress with only a defaultBackend has no rules at all
	host := ""
	if len(it.Spec.Rules) > 0 {
		host = it.Spec.Rules[0].Host
	}

	return buildEntry(it, host, protocol)
}

func dashEntryFromHTTPRoute(obj interface{}) (DashEntry, bool) {
	it, ok := obj.(*gatewayv1.HTTPRoute)
	if !ok {
		return DashEntry{}, false
	}

	host := ""
	if len(it.Spec.Hostnames) > 0 {
		host = string(it.Spec.Hostnames[0])
	}
	// shortcut: CasaVue never reads Gateway listeners, so listener TLS is unknown
	// and links default to http://. Upgrade by resolving Spec.ParentRefs.
	return buildEntry(it, host, "http://")
}

// filteredOut reports whether a cluster object must not be shown on the dashboard.
func filteredOut(obj metav1.Object) bool {
	if applyFilter(config.Content_filters.Namespace.Pattern, config.Content_filters.Namespace.Mode, obj.GetNamespace()) {
		log.Debug("Skipping namespace '" + obj.GetNamespace() + "' due to pattern")
		return true
	}
	if applyFilter(config.Content_filters.Item.Pattern, config.Content_filters.Item.Mode, obj.GetName()) {
		log.Debug("Skipping item '" + obj.GetName() + "' due to pattern")
		return true
	}
	if obj.GetLabels()["app.kubernetes.io/name"] == "casavue" {
		log.Debug("Skipping self: ", obj.GetName())
		return true
	}
	if config.Content_filters.Item.Mode == "ingressAnnotation" {
		_, annotationPresent := obj.GetAnnotations()["casavue.app/enable"]
		if !annotationPresent {
			log.Debug("Skipping item '" + obj.GetName() + "' due to Ingress Annotation mode and lack of annotation.")
			return true
		}
	}
	return false
}

// dashEntryEventHandlers returns informer callbacks shared by every watched
// resource type; entryFunc converts an object into a dashboard entry.
func dashEntryEventHandlers(entryFunc func(interface{}) (DashEntry, bool)) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			meta, ok := obj.(metav1.Object)
			if !ok || filteredOut(meta) {
				return
			}
			key := storeKey(meta)
			entry, ok := entryFunc(obj)
			if !ok {
				return
			}
			log.Info("Dashboard item added: ", key)
			dashboardItems.write(key, entry)
			go crawlItem(key)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			meta, ok := newObj.(metav1.Object)
			if !ok {
				return
			}
			key := storeKey(meta)
			newEntry, entryOk := entryFunc(newObj)
			if !entryOk || filteredOut(meta) {
				// the object is not to be shown anymore
				dashboardItems.delete(key)
				return
			}
			oldEntry, oldOk := entryFunc(oldObj)
			if oldOk && reflect.DeepEqual(oldEntry, newEntry) {
				// Watch events also carry changes irrelevant to the dashboard, e.g. two
				// controllers fighting over status.loadBalancer. Without this check such
				// an object restarts the whole icon crawl over and over.
				log.Debug("Ignoring update without dashboard relevant changes: ", key)
				return
			}
			log.Info("Dashboard item updated: ", key)
			dashboardItems.write(key, newEntry)
			go crawlItem(key)
		},

		DeleteFunc: func(obj interface{}) {
			// when the watch connection dropped, the object arrives as a tombstone
			// whose Key is already "namespace/name"
			if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				log.Info("Dashboard item deleted: ", tomb.Key)
				dashboardItems.delete(tomb.Key)
				return
			}
			if meta, ok := obj.(metav1.Object); ok {
				log.Info("Dashboard item deleted: ", storeKey(meta))
				dashboardItems.delete(storeKey(meta))
			}
		},
	}
}

func getAndWatchKubernetesIngressItems(kconfig *rest.Config) {
	log.Info("Getting Kubernetes Ingress items")

	clientset, err := kubernetes.NewForConfig(kconfig)
	if err != nil {
		log.Warn("Error creating K8s config: ", err)
		return
	}

	watchlist := cache.NewListWatchFromClient(clientset.NetworkingV1().RESTClient(), "ingresses", metav1.NamespaceAll, fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Ingress{},
		time.Second*0,
		dashEntryEventHandlers(dashEntryFromIngress),
	)

	// blocks for the lifetime of the process, the function already runs in its own goroutine
	controller.Run(make(chan struct{}))
}

func getAndWatchKubernetesGatewayRoutes(kconfig *rest.Config) {
	log.Info("Getting Kubernetes Gateway API HTTPRoutes")

	clientset, err := gatewayversioned.NewForConfig(kconfig)
	if err != nil {
		log.Warn("Error creating Gateway API config: ", err)
		return
	}

	// Check if HTTPRoute resource is available
	resources, err := clientset.Discovery().ServerResourcesForGroupVersion("gateway.networking.k8s.io/v1")
	if err != nil {
		log.Info("Could not query for server resources in gateway.networking.k8s.io/v1, skipping Gateway API watch: ", err)
		return
	}
	httpRouteSupported := false
	for _, resource := range resources.APIResources {
		if resource.Name == "httproutes" && resource.Kind == "HTTPRoute" {
			httpRouteSupported = true
			break
		}
	}
	if !httpRouteSupported {
		log.Info("HTTPRoute resource not available on the cluster, skipping Gateway API watch.")
		return
	}

	watchlist := cache.NewListWatchFromClient(clientset.GatewayV1().RESTClient(), "httproutes", metav1.NamespaceAll, fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&gatewayv1.HTTPRoute{},
		time.Second*0,
		dashEntryEventHandlers(dashEntryFromHTTPRoute),
	)

	// blocks for the lifetime of the process, the function already runs in its own goroutine
	controller.Run(make(chan struct{}))
}
