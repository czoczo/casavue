package main

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func immichIngress() *v1.Ingress {
	return &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "immich-public-proxy",
			Namespace:   "public",
			Annotations: map[string]string{"traefik.ingress.kubernetes.io/router.tls": "true"},
			Labels:      map[string]string{"app.kubernetes.io/name": "public-proxy"},
		},
		Spec: v1.IngressSpec{
			TLS:   []v1.IngressTLS{{Hosts: []string{"foty2.lab.dom.cz0.cz"}}},
			Rules: []v1.IngressRule{{Host: "foty2.lab.dom.cz0.cz"}},
		},
	}
}

// The informer calls UpdateFunc for every watch event. Events which do not change
// anything shown on the dashboard (observed in practice: two Traefik controllers
// ping-ponging status.loadBalancer.ingress[].ip) must be recognised as equal.
func TestDashEntryIgnoresIrrelevantIngressChanges(t *testing.T) {
	oldObj := immichIngress()
	newObj := immichIngress()
	newObj.ResourceVersion = "9999"
	oldObj.Status.LoadBalancer.Ingress = []v1.IngressLoadBalancerIngress{{IP: "10.9.0.41"}}
	newObj.Status.LoadBalancer.Ingress = []v1.IngressLoadBalancerIngress{{IP: "10.7.0.42"}}

	oldName, oldEntry := createDashEntryFromIngress(oldObj)
	newName, newEntry := createDashEntryFromIngress(newObj)
	if oldName != newName || !reflect.DeepEqual(oldEntry, newEntry) {
		t.Errorf("status only change must not alter the entry:\nold=%+v\nnew=%+v", oldEntry, newEntry)
	}

	// a change that is displayed must not be swallowed by the comparison
	newObj.Annotations["casavue.app/icon"] = "https://example.com/icon.svg"
	if _, changedEntry := createDashEntryFromIngress(newObj); reflect.DeepEqual(oldEntry, changedEntry) {
		t.Error("annotation change must alter the entry")
	}
}
