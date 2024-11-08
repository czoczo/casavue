// Kubernetes integration reading Ingress resources

package main

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/networking/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func createDashEntryFromIngress(it *v1.Ingress) (string, DashEntry) {
	protocol := "http://"
	name := it.Name
	description := ""
	iconURL := ""

	if len(it.Spec.TLS) > 0 {
		protocol = "https://"
	}
	URL := protocol + it.Spec.Rules[0].Host

	if extractedDescription, ok := it.Annotations["casavue.app/description"]; ok {
		log.Debug("Found description: ", extractedDescription)
		description = extractedDescription
	}

	if extractedName, ok := it.Annotations["casavue.app/name"]; ok {
		log.Debug("Found name override: ", extractedName)
		name = extractedName
	}

	if extractedName, ok := it.Annotations["casavue.app/icon"]; ok {
		log.Debug("Found icon URL override: ", extractedName)
		iconURL = extractedName
	}

	if extractedName, ok := it.Annotations["casavue.app/url"]; ok {
		log.Debug("Found URL override: ", extractedName)
		URL = extractedName
	}

	log.Info("Adding Dashboard Item based on ingress '", it.Name, "', with key '", name, "'.")
	return name, DashEntry{it.Namespace, description, URL, "", iconURL, it.Labels}
}

func getAndWatchKubernetesIngressItems() {
	log.Info("Getting Kubernetes Ingress items")
	// initiate variables
	var kubeconfig *string

	// obtain kubeconfig file
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(Optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// creates the in-cluster config
	kconfig, err := rest.InClusterConfig()
	if err != nil {
		log.Warn("Error creating K8s in-cluster config: ", err)
	}

	if _, err := os.Stat(*kubeconfig); err == nil {
		kconfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			log.Warn("Error building K8s config form flags: ", err)
		}
	}

	if kconfig == nil {
		return
	}

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
