// Kubernetes integration reading Ingress resources

package main

import (
	"os"
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

func createDashEntryFromIngress(it *v1.Ingress) (string, DashEntry) {
	protocol := "http://"
	name := it.Name
	description := ""
	iconURL := ""

	if len(it.Spec.TLS) > 0 {
		protocol = "https://"
	}
	URL := protocol + it.Spec.Rules[0].Host

	desc, nameOverride, iconOverride, urlOverride := processAnnotations(it.Annotations)

	if desc != "" {
		description = desc
	}
	if nameOverride != "" {
		name = nameOverride
	}
	if iconOverride != "" {
		iconURL = iconOverride
	}
	if urlOverride != "" {
		URL = urlOverride
	}

	log.Info("Adding Dashboard Item based on ingress '", it.Name, "', with key '", name, "'.")
	return name, DashEntry{it.Namespace, description, URL, "", iconURL, it.Labels}
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
		cache.ResourceEventHandlerFuncs{

			AddFunc: func(obj interface{}) {
				ingress := obj.(*v1.Ingress)
				if applyFilter(config.Content_filters.Namespace.Pattern, config.Content_filters.Namespace.Mode, ingress.Namespace) {
					log.Debug("Skipping namespace '" + ingress.Namespace + "' due to pattern")
					return
				}
				if applyFilter(config.Content_filters.Item.Pattern, config.Content_filters.Item.Mode, ingress.Name) {
					log.Debug("Skipping item '" + ingress.Name + "' due to pattern")
					return
				}

				// skip self
				if val, ok := ingress.Labels["app.kubernetes.io/name"]; ok {
					if val == "casavue" {
						log.Debug("Skipping self: ", ingress.Name)
						return
					}
				}

				_, annotationPresent := ingress.Annotations["casavue.app/enable"]
				if config.Content_filters.Item.Mode == "ingressAnnotation" && !annotationPresent {
					log.Debug("Skipping item '" + ingress.Name + "' due to Ingress Annotation mode and lack of annotation.")
					return
				}
				log.Info("Ingress added: ", ingress.Name)
				name, dashboardItem := createDashEntryFromIngress(ingress)
				dashboardItems.write(name, dashboardItem)
				go crawlItem(name)
			},

			DeleteFunc: func(obj interface{}) {
				ingress := obj.(*v1.Ingress)
				log.Info("Ingress deleted: ", ingress.Name)
				dashboardItems.delete(ingress.Name)
			},

			UpdateFunc: func(oldObj, newObj interface{}) {
				oldIngress := oldObj.(*v1.Ingress)
				newIngress := newObj.(*v1.Ingress)

				dashboardItems.delete(oldIngress.Name)

				if applyFilter(config.Content_filters.Namespace.Pattern, config.Content_filters.Namespace.Mode, newIngress.Namespace) {
					log.Debug("Skipping namespace '" + newIngress.Namespace + "' due to pattern")
					return
				}
				if applyFilter(config.Content_filters.Item.Pattern, config.Content_filters.Item.Mode, newIngress.Name) {
					log.Debug("Skipping name '" + newIngress.Name + "' due to pattern")
					return
				}
				_, annotationPresent := newIngress.Annotations["casavue.app/enable"]
				if config.Content_filters.Item.Mode == "ingressAnnotation" && !annotationPresent {
					log.Debug("Skipping item '" + newIngress.Name + "' due to Ingress Annotation mode and lack of annotation.")
					return
				}
				log.Info("Ingress updated: ", oldIngress.Name, " -> ", newIngress.Name)
				name, dashboardItem := createDashEntryFromIngress(newIngress)
				dashboardItems.write(name, dashboardItem)
				go crawlItem(name)
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	for {
		time.Sleep(time.Second)
	}

}

func createDashEntryFromHTTPRoute(it *gatewayv1.HTTPRoute) (string, DashEntry) {
	protocol := "http://"
	name := it.Name
	description := ""
	iconURL := ""

	// Check for TLS - simplified logic or annotations?

	URL := ""
	if len(it.Spec.Hostnames) > 0 {
		URL = protocol + string(it.Spec.Hostnames[0])
	}

	desc, nameOverride, iconOverride, urlOverride := processAnnotations(it.Annotations)

	if desc != "" {
		description = desc
	}
	if nameOverride != "" {
		name = nameOverride
	}
	if iconOverride != "" {
		iconURL = iconOverride
	}
	if urlOverride != "" {
		URL = urlOverride
	}

	log.Info("Adding Dashboard Item based on httproute '", it.Name, "', with key '", name, "'.")
	return name, DashEntry{it.Namespace, description, URL, "", iconURL, it.Labels}
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
		cache.ResourceEventHandlerFuncs{

			AddFunc: func(obj interface{}) {
				route := obj.(*gatewayv1.HTTPRoute)
				if applyFilter(config.Content_filters.Namespace.Pattern, config.Content_filters.Namespace.Mode, route.Namespace) {
					log.Debug("Skipping namespace '" + route.Namespace + "' due to pattern")
					return
				}
				if applyFilter(config.Content_filters.Item.Pattern, config.Content_filters.Item.Mode, route.Name) {
					log.Debug("Skipping item '" + route.Name + "' due to pattern")
					return
				}

				// skip self
				if val, ok := route.Labels["app.kubernetes.io/name"]; ok {
					if val == "casavue" {
						log.Debug("Skipping self: ", route.Name)
						return
					}
				}

				_, annotationPresent := route.Annotations["casavue.app/enable"]
				if config.Content_filters.Item.Mode == "ingressAnnotation" && !annotationPresent {
					log.Debug("Skipping item '" + route.Name + "' due to Ingress Annotation mode and lack of annotation.")
					return
				}
				log.Info("HTTPRoute added: ", route.Name)
				name, dashboardItem := createDashEntryFromHTTPRoute(route)
				dashboardItems.write(name, dashboardItem)
				go crawlItem(name)
			},

			DeleteFunc: func(obj interface{}) {
				route := obj.(*gatewayv1.HTTPRoute)
				log.Info("HTTPRoute deleted: ", route.Name)
				dashboardItems.delete(route.Name)
			},

			UpdateFunc: func(oldObj, newObj interface{}) {
				oldRoute := oldObj.(*gatewayv1.HTTPRoute)
				newRoute := newObj.(*gatewayv1.HTTPRoute)

				dashboardItems.delete(oldRoute.Name)

				if applyFilter(config.Content_filters.Namespace.Pattern, config.Content_filters.Namespace.Mode, newRoute.Namespace) {
					log.Debug("Skipping namespace '" + newRoute.Namespace + "' due to pattern")
					return
				}
				if applyFilter(config.Content_filters.Item.Pattern, config.Content_filters.Item.Mode, newRoute.Name) {
					log.Debug("Skipping name '" + newRoute.Name + "' due to pattern")
					return
				}
				_, annotationPresent := newRoute.Annotations["casavue.app/enable"]
				if config.Content_filters.Item.Mode == "ingressAnnotation" && !annotationPresent {
					log.Debug("Skipping item '" + newRoute.Name + "' due to Ingress Annotation mode and lack of annotation.")
					return
				}
				log.Info("HTTPRoute updated: ", oldRoute.Name, " -> ", newRoute.Name)
				name, dashboardItem := createDashEntryFromHTTPRoute(newRoute)
				dashboardItems.write(name, dashboardItem)
				go crawlItem(name)
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	for {
		time.Sleep(time.Second)
	}

}
