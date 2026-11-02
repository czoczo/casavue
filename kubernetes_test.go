package main

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
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

// tlsListener is a Gateway listener; an empty hostname means "every hostname".
func tlsListener(name string, protocol gatewayv1.ProtocolType, mode *gatewayv1.TLSModeType, hostname string) gatewayv1.Listener {
	listener := gatewayv1.Listener{
		Name:     gatewayv1.SectionName(name),
		Protocol: protocol,
		Port:     443,
	}
	if hostname != "" {
		h := gatewayv1.Hostname(hostname)
		listener.Hostname = &h
	}
	if mode != nil {
		listener.TLS = &gatewayv1.ListenerTLSConfig{Mode: mode}
	}
	return listener
}

const routeHost = "foto.lab.dom"

const tlsProtocol = gatewayv1.HTTPSProtocolType

// immichRoute is an HTTPRoute attached to the "home" Gateway of its own namespace.
func immichRoute(refs ...gatewayv1.ParentReference) *gatewayv1.HTTPRoute {
	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "immich", Namespace: "public"},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: refs},
			Hostnames:       []gatewayv1.Hostname{routeHost},
		},
	}
}

// TestHTTPRoutePrefersGatewayTLS: an HTTPRoute has no TLS itself, it is terminated
// by the listeners of the Gateway it attaches to. The link has to be https://
// whenever such a listener serves the route hostname, http:// only as a fallback.
func TestHTTPRoutePrefersGatewayTLS(t *testing.T) {
	namespace := func(s string) *gatewayv1.Namespace { n := gatewayv1.Namespace(s); return &n }
	section := func(s string) *gatewayv1.SectionName { n := gatewayv1.SectionName(s); return &n }
	kind := func(s string) *gatewayv1.Kind { n := gatewayv1.Kind(s); return &n }

	terminate, passthrough := gatewayv1.TLSModeTerminate, gatewayv1.TLSModePassthrough

	tests := []struct {
		name      string
		gateways  map[string][]gatewayv1.Listener
		refs      []gatewayv1.ParentReference
		hostnames []string
		want      string
	}{
		{
			name:     "https listener",
			gateways: map[string][]gatewayv1.Listener{"public/home": {tlsListener("web", tlsProtocol, &terminate, "")}},
			refs:     []gatewayv1.ParentReference{{Name: "home"}},
			want:     "https://foto.lab.dom",
		},
		{
			name:     "no gateway watched at all is the http fallback",
			gateways: map[string][]gatewayv1.Listener{},
			refs:     []gatewayv1.ParentReference{{Name: "home"}},
			want:     "http://foto.lab.dom",
		},
		{
			name:     "http listener only",
			gateways: map[string][]gatewayv1.Listener{"public/home": {tlsListener("web", gatewayv1.HTTPProtocolType, nil, "")}},
			refs:     []gatewayv1.ParentReference{{Name: "home"}},
			want:     "http://foto.lab.dom",
		},
		{
			// both protocols side by side: https wins
			name: "http and https listener",
			gateways: map[string][]gatewayv1.Listener{"public/home": {
				tlsListener("plain", gatewayv1.HTTPProtocolType, nil, ""),
				tlsListener("secure", tlsProtocol, nil, ""),
			}},
			refs: []gatewayv1.ParentReference{{Name: "home"}},
			want: "https://foto.lab.dom",
		},
		{
			name:     "tls passthrough is not https for a route",
			gateways: map[string][]gatewayv1.Listener{"public/home": {tlsListener("raw", gatewayv1.TLSProtocolType, &passthrough, "")}},
			refs:     []gatewayv1.ParentReference{{Name: "home"}},
			want:     "http://foto.lab.dom",
		},
		{
			name:     "listener hostname wildcard serves the route",
			gateways: map[string][]gatewayv1.Listener{"public/home": {tlsListener("web", tlsProtocol, nil, "*.lab.dom")}},
			refs:     []gatewayv1.ParentReference{{Name: "home"}},
			want:     "https://foto.lab.dom",
		},
		{
			name:     "https listener for another hostname does not",
			gateways: map[string][]gatewayv1.Listener{"public/home": {tlsListener("web", tlsProtocol, nil, "other.lab.dom")}},
			refs:     []gatewayv1.ParentReference{{Name: "home"}},
			want:     "http://foto.lab.dom",
		},
		{
			name: "sectionName picks one listener",
			gateways: map[string][]gatewayv1.Listener{"public/home": {
				tlsListener("plain", gatewayv1.HTTPProtocolType, nil, ""),
				tlsListener("secure", tlsProtocol, nil, ""),
			}},
			refs: []gatewayv1.ParentReference{{Name: "home", SectionName: section("plain")}},
			want: "http://foto.lab.dom",
		},
		{
			name: "sectionName picks the https listener",
			gateways: map[string][]gatewayv1.Listener{"public/home": {
				tlsListener("plain", gatewayv1.HTTPProtocolType, nil, ""),
				tlsListener("secure", tlsProtocol, nil, ""),
			}},
			refs: []gatewayv1.ParentReference{{Name: "home", SectionName: section("secure")}},
			want: "https://foto.lab.dom",
		},
		{
			name:     "gateway of another namespace",
			gateways: map[string][]gatewayv1.Listener{"ingress/edge": {tlsListener("web", tlsProtocol, nil, "")}},
			refs:     []gatewayv1.ParentReference{{Name: "edge", Namespace: namespace("ingress")}},
			want:     "https://foto.lab.dom",
		},
		{
			name:     "gateway name of another namespace is not the local one",
			gateways: map[string][]gatewayv1.Listener{"ingress/home": {tlsListener("web", tlsProtocol, nil, "")}},
			refs:     []gatewayv1.ParentReference{{Name: "home"}},
			want:     "http://foto.lab.dom",
		},
		{
			name:     "a parentRef which is not a Gateway",
			gateways: map[string][]gatewayv1.Listener{"public/home": {tlsListener("web", tlsProtocol, nil, "")}},
			refs:     []gatewayv1.ParentReference{{Name: "home", Kind: kind("Service")}},
			want:     "http://foto.lab.dom",
		},
		{
			name:     "one of several parentRefs terminates TLS",
			gateways: map[string][]gatewayv1.Listener{"public/old": {tlsListener("web", tlsProtocol, nil, "")}},
			refs:     []gatewayv1.ParentReference{{Name: "missing"}, {Name: "old"}},
			want:     "https://foto.lab.dom",
		},
		{
			name:     "no parentRefs",
			gateways: map[string][]gatewayv1.Listener{"public/home": {tlsListener("web", tlsProtocol, nil, "")}},
			want:     "http://foto.lab.dom",
		},
		{
			// as observed on a real cluster: a route is served only with the hostnames
			// its listeners serve, the first one of the route need not be one of them
			name:      "first hostname is served by no listener",
			gateways:  map[string][]gatewayv1.Listener{"mgmt/home": {tlsListener("https", tlsProtocol, nil, "*.lab.dom")}},
			refs:      []gatewayv1.ParentReference{{Name: "home", Namespace: namespace("mgmt"), SectionName: section("https")}},
			hostnames: []string{"lab.dom", "foto.lab.dom"},
			want:      "https://foto.lab.dom",
		},
		{
			name:      "hostname served plainly wins over an unserved first one",
			gateways:  map[string][]gatewayv1.Listener{"public/home": {tlsListener("plain", gatewayv1.HTTPProtocolType, nil, "foto.lab.dom")}},
			refs:      []gatewayv1.ParentReference{{Name: "home"}},
			hostnames: []string{"unserved.lab.dom", "foto.lab.dom"},
			want:      "http://foto.lab.dom",
		},
		{
			name:      "no listener serves any hostname",
			gateways:  map[string][]gatewayv1.Listener{"public/home": {tlsListener("web", tlsProtocol, nil, "other.lab.dom")}},
			refs:      []gatewayv1.ParentReference{{Name: "home"}},
			hostnames: []string{"first.lab.dom", "second.lab.dom"},
			want:      "http://first.lab.dom",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for key, listeners := range tc.gateways {
				gatewayListeners.write(key, listeners)
				defer gatewayListeners.forget(key)
			}
			route := immichRoute(tc.refs...)
			if len(tc.hostnames) > 0 {
				route.Spec.Hostnames = nil
				for _, hostname := range tc.hostnames {
					route.Spec.Hostnames = append(route.Spec.Hostnames, gatewayv1.Hostname(hostname))
				}
			}
			entry, ok := dashEntryFromHTTPRoute(route)
			if !ok || entry.URL != tc.want {
				t.Errorf("got %+v (ok=%v), want %s", entry, ok, tc.want)
			}
		})
	}

	// the URL annotation still decides, as with an Ingress
	route := immichRoute(gatewayv1.ParentReference{Name: "home"})
	route.Annotations = map[string]string{"casavue.app/url": "http://foto.internal:8080"}
	gatewayListeners.write("public/home", []gatewayv1.Listener{tlsListener("web", tlsProtocol, nil, "")})
	defer gatewayListeners.forget("public/home")
	if entry, ok := dashEntryFromHTTPRoute(route); !ok || entry.URL != "http://foto.internal:8080" {
		t.Errorf("URL annotation must win over listener TLS, got %+v (ok=%v)", entry, ok)
	}
}

// A Gateway appears, changes or disappears after its routes were read, so the
// protocol of already shown items has to be re-checked. The refresh starts icon
// crawls of the fake hosts of this test: they are parked on the crawl slots and
// released as finished no-ops, because the items are out of the dashboard by then.
func TestRefreshRouteTLSOnGatewayChange(t *testing.T) {
	httpRoutesWas := httpRoutes
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	httpRoutes = store

	for i := 0; i < maxConcurrentCrawls; i++ {
		crawlSlots <- struct{}{}
	}
	defer func() {
		httpRoutes = httpRoutesWas
		// out of the dashboard first, then release the parked crawls: finding no
		// item, they end without fetching anything
		for _, item := range store.List() {
			if route, ok := item.(*gatewayv1.HTTPRoute); ok {
				dashboardItems.delete(storeKey(route))
			}
		}
		for len(crawlSlots) > 0 {
			<-crawlSlots
		}
	}()

	route := immichRoute(gatewayv1.ParentReference{Name: "home"})
	if err := store.Add(route); err != nil {
		t.Fatal(err)
	}
	key := storeKey(route)
	dashboardItems.write(key, DashEntry{Namespace: "public", Name: "immich", URL: "http://" + routeHost})

	// a route which is not shown stays out of the dashboard
	hidden := immichRoute(gatewayv1.ParentReference{Name: "home"})
	hidden.Name = "hidden"
	hidden.Annotations = map[string]string{"casavue.app/url": "https://hidden.example"}
	if err := store.Add(hidden); err != nil {
		t.Fatal(err)
	}

	gatewayListeners.write("public/home", []gatewayv1.Listener{tlsListener("web", tlsProtocol, nil, "")})
	defer gatewayListeners.forget("public/home")

	refreshRouteTLS()

	entry, _, ok := dashboardItems.read(key)
	if !ok || entry.URL != "https://"+routeHost {
		t.Errorf("item must follow the Gateway TLS, got %+v (ok=%v)", entry, ok)
	}
	if _, _, found := dashboardItems.read(storeKey(hidden)); found {
		t.Error("refresh must not add items which are not shown")
	}

	// Gateway status churn must not write the item again
	_, version, _ := dashboardItems.read(key)
	refreshRouteTLS()
	if _, after, _ := dashboardItems.read(key); after != version {
		t.Errorf("unchanged item must not be rewritten, version %d -> %d", version, after)
	}

	// and the other way round, when the Gateway stops terminating TLS
	gatewayListeners.write("public/home", []gatewayv1.Listener{tlsListener("plain", gatewayv1.HTTPProtocolType, nil, "")})
	refreshRouteTLS()
	if entry, _, _ := dashboardItems.read(key); entry.URL != "http://"+routeHost {
		t.Errorf("item must fall back to http, got %q", entry.URL)
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
