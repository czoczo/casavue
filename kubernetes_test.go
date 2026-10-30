package main

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
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

	oldEntry, _ := dashEntryFromIngress(oldObj)
	newEntry, _ := dashEntryFromIngress(newObj)
	if !reflect.DeepEqual(oldEntry, newEntry) {
		t.Errorf("status only change must not alter the entry:\nold=%+v\nnew=%+v", oldEntry, newEntry)
	}

	// a change that is displayed must not be swallowed by the comparison
	newObj.Annotations["casavue.app/icon"] = "https://example.com/icon.svg"
	if changedEntry, _ := dashEntryFromIngress(newObj); reflect.DeepEqual(oldEntry, changedEntry) {
		t.Error("annotation change must alter the entry")
	}
}

// An Ingress with only a defaultBackend has no rules; reading Rules[0] used to
// crash the whole process.
func TestIngressWithoutRulesIsRejected(t *testing.T) {
	it := &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-only", Namespace: "default"},
		Spec:       v1.IngressSpec{DefaultBackend: &v1.IngressBackend{}},
	}
	if _, ok := dashEntryFromIngress(it); ok {
		t.Error("ingress without rules and without URL annotation must not become an entry")
	}

	it.Annotations = map[string]string{"casavue.app/url": "https://example.com/"}
	entry, ok := dashEntryFromIngress(it)
	if !ok || entry.URL != "https://example.com/" {
		t.Errorf("URL annotation must be usable even without rules, got %+v (ok=%v)", entry, ok)
	}
}

func TestHTTPRouteWithoutHostnamesIsRejected(t *testing.T) {
	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-route", Namespace: "ingress"},
	}
	if _, ok := dashEntryFromHTTPRoute(route); ok {
		t.Error("route without hostnames and without URL annotation must not become an entry")
	}

	route.Spec.Hostnames = []gatewayv1.Hostname{"app.example.com"}
	entry, ok := dashEntryFromHTTPRoute(route)
	if !ok || entry.URL != "http://app.example.com" {
		t.Errorf("route hostname must become the URL, got %+v (ok=%v)", entry, ok)
	}
}

func TestStoreKeyKeepsSameNamedIngressesApart(t *testing.T) {
	first := immichIngress()
	second := immichIngress()
	second.Namespace = "media"

	firstKey, secondKey := storeKey(first), storeKey(second)
	if firstKey != "public/immich-public-proxy" {
		t.Errorf("unexpected key %q", firstKey)
	}
	if firstKey == secondKey {
		t.Error("same object name in two namespaces must not share a store key")
	}

	store := newTestStore()
	for _, it := range []*v1.Ingress{first, second} {
		entry, ok := dashEntryFromIngress(it)
		if !ok {
			t.Fatal("valid ingress rejected")
		}
		store.write(storeKey(it), entry)
	}
	if len(store.get()) != 2 {
		t.Errorf("both namespaces must be visible, got %d items", len(store.get()))
	}
}

func TestEnableAnnotation(t *testing.T) {
	it := immichIngress()
	if enabledByAnnotation(it.Annotations) {
		t.Error("in ingressAnnotation mode an object without the annotation must stay hidden")
	}

	for value, expected := range map[string]bool{
		// empty value keeps meaning "enabled", that is what bare annotations did before
		"true": true, "True": true, "": true, "false": false, "no": false, "off": false,
	} {
		it.Annotations = map[string]string{"casavue.app/enable": value}
		if got := enabledByAnnotation(it.Annotations); got != expected {
			t.Errorf("casavue.app/enable=%q: got %v, expected %v", value, got, expected)
		}
	}
}

func TestDisplayNamePrecedence(t *testing.T) {
	it := immichIngress()
	if got := displayName(it, ""); got != "public-proxy" {
		t.Errorf("app.kubernetes.io/name label must be used, got %q", got)
	}
	if got := displayName(it, "Immich"); got != "Immich" {
		t.Errorf("name override must win, got %q", got)
	}

	it.Labels = nil
	if got := displayName(it, ""); got != "immich-public-proxy" {
		t.Errorf("object name must be the fallback, got %q", got)
	}
}
