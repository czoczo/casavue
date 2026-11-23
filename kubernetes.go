// Kubernetes integration reading Ingress and Gateway API HTTPRoute resources

package main

import (
	"os"
	"reflect"
	"strings"
	"sync"
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

	host, protocol := routeHostAndProtocol(it)
	return buildEntry(it, host, protocol)
}

// listenerStore keeps the listeners of every watched Gateway by "namespace/name".
// An HTTPRoute has no TLS of its own, it is terminated by the listeners of the
// Gateway it attaches to, so those have to be known to link to the route with the
// right protocol. Written by the Gateway informer, read while an entry is built.
type listenerStore struct {
	sync.RWMutex
	byKey map[string][]gatewayv1.Listener
}

var gatewayListeners = listenerStore{byKey: make(map[string][]gatewayv1.Listener)}

func (ls *listenerStore) write(key string, listeners []gatewayv1.Listener) {
	ls.Lock()
	defer ls.Unlock()
	ls.byKey[key] = listeners
}

func (ls *listenerStore) forget(key string) {
	ls.Lock()
	defer ls.Unlock()
	delete(ls.byKey, key)
}

func (ls *listenerStore) get(key string) []gatewayv1.Listener {
	ls.RLock()
	defer ls.RUnlock()
	return ls.byKey[key]
}

// httpRoutes is the store of the HTTPRoute informer, so a changed Gateway can
// re-check the routes attached to it. Set before the Gateway informer starts.
var httpRoutes cache.Store

// clusterObjectStore is what an informer store is used for here: the objects it
// happens to hold.
type clusterObjectStore interface {
	List() []interface{}
}

// watchedInformer is one running informer: its store of cluster objects and the
// conversion of those objects into dashboard entries.
type watchedInformer struct {
	store     clusterObjectStore
	entryFunc func(interface{}) (DashEntry, bool)
}

// informerStores are the running informers, registered by the informer goroutines as
// they start and read by a configuration reload.
type informerStores struct {
	sync.Mutex
	stores []watchedInformer
}

var watchedStores informerStores

func (ws *informerStores) add(store clusterObjectStore, entryFunc func(interface{}) (DashEntry, bool)) {
	ws.Lock()
	defer ws.Unlock()
	ws.stores = append(ws.stores, watchedInformer{store: store, entryFunc: entryFunc})
}

func (ws *informerStores) all() []watchedInformer {
	ws.Lock()
	defer ws.Unlock()
	result := make([]watchedInformer, len(ws.stores))
	copy(result, ws.stores)
	return result
}

// applyWatchedObjects applies the content filters of the current configuration to
// every watched cluster object, so a configuration change is applied at once and not
// only with the next event of a cluster object. shortcut: an item already on the
// dashboard keeps its entry, a cluster object changing in a way which matters for it
// is taken care of by its own informer event.
func applyWatchedObjects() {
	for _, watched := range watchedStores.all() {
		for _, obj := range watched.store.List() {
			meta, ok := obj.(metav1.Object)
			if !ok {
				continue
			}
			key := storeKey(meta)
			if filteredOut(meta) {
				if _, _, found := dashboardItems.read(key); found {
					log.Info("Dashboard item hidden (configuration change): ", key)
					dashboardItems.delete(key)
				}
				continue
			}
			if _, _, found := dashboardItems.read(key); found {
				continue
			}
			entry, ok := watched.entryFunc(obj)
			if !ok {
				continue
			}
			log.Info("Dashboard item added (configuration change): ", key)
			dashboardItems.write(key, entry)
			go crawlItem(key)
		}
	}
}

// terminatesTLS reports whether the listener ends TLS, which makes the routes
// behind it reachable over https://. "TLS" protocol listeners pass TLS through to
// a TLSRoute instead and never serve an HTTPRoute.
func terminatesTLS(listener gatewayv1.Listener) bool {
	return listener.Protocol == gatewayv1.HTTPSProtocolType &&
		(listener.TLS == nil || listener.TLS.Mode == nil || *listener.TLS.Mode == gatewayv1.TLSModeTerminate)
}

// listenerServesHost reports whether the listener answers for a route hostname: a
// listener without hostname answers for every one, a "*." prefix is a suffix match.
func listenerServesHost(listener gatewayv1.Listener, host string) bool {
	if listener.Hostname == nil || *listener.Hostname == "" {
		return true
	}
	hostname := string(*listener.Hostname)
	if strings.HasPrefix(hostname, "*.") {
		return strings.HasSuffix(host, hostname[1:])
	}
	return hostname == host
}

// routeListeners are the listeners of the Gateways a route attaches to. A parentRef
// naming no listener means every listener of that Gateway.
func routeListeners(route *gatewayv1.HTTPRoute) []gatewayv1.Listener {
	var listeners []gatewayv1.Listener
	for _, ref := range route.Spec.ParentRefs {
		if ref.Kind != nil && *ref.Kind != "Gateway" {
			continue
		}
		namespace := route.Namespace
		if ref.Namespace != nil {
			namespace = string(*ref.Namespace)
		}
		for _, listener := range gatewayListeners.get(namespace + "/" + string(ref.Name)) {
			if ref.SectionName == nil || listener.Name == *ref.SectionName {
				listeners = append(listeners, listener)
			}
		}
	}
	return listeners
}

// routeHostAndProtocol decides which hostname of a route to link to and how. A route
// is served only by the listeners of its Gateways and only with the hostnames those
// listeners serve, so a hostname none of them serves cannot be reached at all. Prefer
// a hostname served over TLS, then one served plainly, and finally, when no Gateway
// is known or none of the hostnames matches, the first hostname with the http://
// fallback. shortcut: the listener port is ignored, a route served on https only on a
// non standard port gets a portless link; upgrade by adding the port to the URL.
func routeHostAndProtocol(route *gatewayv1.HTTPRoute) (host, protocol string) {
	protocol = "http://"
	if len(route.Spec.Hostnames) == 0 {
		return
	}
	listeners := routeListeners(route)
	if len(listeners) == 0 {
		return string(route.Spec.Hostnames[0]), protocol
	}
	for _, wantTLS := range []bool{true, false} {
		for _, hostname := range route.Spec.Hostnames {
			for _, listener := range listeners {
				if listenerServesHost(listener, string(hostname)) &&
					terminatesTLS(listener) == wantTLS {
					if wantTLS {
						protocol = "https://"
					}
					return string(hostname), protocol
				}
			}
		}
	}
	return string(route.Spec.Hostnames[0]), protocol
}

// refreshRouteTLS re-checks the items of every known HTTPRoute after a Gateway
// appeared, changed or was deleted - that is when it becomes known that a route is
// served over TLS, or stops being served that way. Items whose link does not change
// are left alone, so the usual Gateway status churn costs nothing.
func refreshRouteTLS() {
	for _, obj := range httpRoutes.List() {
		route, ok := obj.(*gatewayv1.HTTPRoute)
		if !ok {
			continue
		}
		key := storeKey(route)
		oldEntry, _, found := dashboardItems.read(key)
		if !found {
			continue
		}
		newEntry, ok := dashEntryFromHTTPRoute(route)
		if !ok || newEntry.URL == oldEntry.URL {
			continue
		}
		log.Info("Dashboard item updated (Gateway TLS): ", key, " ", oldEntry.URL, " -> ", newEntry.URL)
		dashboardItems.write(key, newEntry)
		go crawlItem(key)
	}
}

func gatewayTLSEventHandlers() cache.ResourceEventHandlerFuncs {
	remember := func(obj interface{}) {
		if gw, ok := obj.(*gatewayv1.Gateway); ok {
			gatewayListeners.write(storeKey(gw), gw.Spec.Listeners)
		}
		refreshRouteTLS()
	}
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    remember,
		UpdateFunc: func(_, newObj interface{}) { remember(newObj) },
		DeleteFunc: func(obj interface{}) {
			if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				gatewayListeners.forget(tomb.Key)
			} else if gw, ok := obj.(*gatewayv1.Gateway); ok {
				gatewayListeners.forget(storeKey(gw))
			}
			refreshRouteTLS()
		},
	}
}

// filteredOut reports whether a cluster object must not be shown on the dashboard.
func filteredOut(obj metav1.Object) bool {
	filters := contentFilters()
	if applyFilter(filters.Namespace.Pattern, filters.Namespace.Mode, obj.GetNamespace()) {
		log.Debug("Skipping namespace '" + obj.GetNamespace() + "' due to pattern")
		return true
	}
	if applyFilter(filters.Item.Pattern, filters.Item.Mode, obj.GetName()) {
		log.Debug("Skipping item '" + obj.GetName() + "' due to pattern")
		return true
	}
	if obj.GetLabels()["app.kubernetes.io/name"] == "casavue" {
		log.Debug("Skipping self: ", obj.GetName())
		return true
	}
	if filters.Item.Mode == "ingressAnnotation" && !enabledByAnnotation(obj.GetAnnotations()) {
		log.Debug("Skipping item '" + obj.GetName() + "' due to Ingress Annotation mode and no enabled annotation.")
		return true
	}
	return false
}

// enabledByAnnotation reports whether the casavue.app/enable annotation allows
// the object: in "ingressAnnotation" mode the annotation is required, and its
// value can turn a single item off again.
func enabledByAnnotation(annotations map[string]string) bool {
	value, present := annotations["casavue.app/enable"]
	if !present {
		return false
	}
	switch strings.ToLower(value) {
	case "false", "no", "off":
		return false
	}
	return true
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
	ingressStore, controller := cache.NewInformer(
		watchlist,
		&v1.Ingress{},
		time.Second*0,
		dashEntryEventHandlers(dashEntryFromIngress),
	)
	watchedStores.add(ingressStore, dashEntryFromIngress)

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
	httpRouteSupported, gatewaySupported := false, false
	for _, resource := range resources.APIResources {
		switch {
		case resource.Name == "httproutes" && resource.Kind == "HTTPRoute":
			httpRouteSupported = true
		case resource.Name == "gateways" && resource.Kind == "Gateway":
			gatewaySupported = true
		}
	}
	if !httpRouteSupported {
		log.Info("HTTPRoute resource not available on the cluster, skipping Gateway API watch.")
		return
	}

	watchlist := cache.NewListWatchFromClient(clientset.GatewayV1().RESTClient(), "httproutes", metav1.NamespaceAll, fields.Everything())
	routeStore, controller := cache.NewInformer(
		watchlist,
		&gatewayv1.HTTPRoute{},
		time.Second*0,
		dashEntryEventHandlers(dashEntryFromHTTPRoute),
	)
	httpRoutes = routeStore
	watchedStores.add(routeStore, dashEntryFromHTTPRoute)

	// Gateways are only watched to learn which listeners terminate TLS. Not waited
	// for: without Gateways (missing resource or RBAC) the routes are still shown,
	// with the http:// fallback, instead of not at all.
	if gatewaySupported {
		gatewayWatchlist := cache.NewListWatchFromClient(clientset.GatewayV1().RESTClient(), "gateways", metav1.NamespaceAll, fields.Everything())
		_, gatewayController := cache.NewInformer(
			gatewayWatchlist,
			&gatewayv1.Gateway{},
			time.Second*0,
			gatewayTLSEventHandlers(),
		)
		go gatewayController.Run(make(chan struct{}))
	} else {
		log.Info("Gateway resource not available on the cluster, HTTPRoute links cannot detect TLS.")
	}

	// blocks for the lifetime of the process, the function already runs in its own goroutine
	controller.Run(make(chan struct{}))
}
