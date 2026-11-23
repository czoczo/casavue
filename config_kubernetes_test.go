package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// The two configuration ConfigMaps arrive as separate events, so each event has to be
// applied together with the content of the other file: an event of one of them must
// never drop the configuration of the second one.
func TestConfigurationConfigMapsAreAlwaysAppliedAsAPair(t *testing.T) {
	isolateConfig(t, "")
	savedMain, savedItems := lastMainYAML, lastItemsYAML
	t.Cleanup(func() { lastMainYAML, lastItemsYAML = savedMain, savedItems })

	// what the mounted files hold, which is where the ConfigMap watch starts
	lastMainYAML, lastItemsYAML = nil, nil
	if !reloadIfChanged(nil, []byte("items:\n- name: fromfile\n  namespace: static\n  url: http://127.0.0.1:1/\n")) {
		t.Fatal("configuration of the files not applied")
	}
	content := newConfigMapContent()

	itemsEvents := configMapEventHandlers(content, itemsConfigKey)
	mainEvents := configMapEventHandlers(content, mainConfigKey)

	// an items ConfigMap which is watched, a main ConfigMap which is not there at all
	itemsEvents.OnAdd(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: itemsConfigMapName, Namespace: "home"},
		Data:       map[string]string{itemsConfigKey: "items:\n- name: fromcm\n  namespace: static\n  url: http://127.0.0.1:2/\n"},
	}, false)
	if _, _, ok := dashboardItems.read("static/fromcm"); !ok {
		t.Error("static item of the items ConfigMap not applied")
	}
	if _, _, ok := dashboardItems.read("static/fromfile"); ok {
		t.Error("static item of the files kept after the items ConfigMap replaced it")
	}
	// unchanged content of both files applies nothing
	if reloadIfChanged(lastMainYAML, lastItemsYAML) {
		t.Error("unchanged configuration applied again")
	}

	// the main ConfigMap event must keep the items of the other ConfigMap
	mainEvents.OnAdd(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: mainConfigMapName, Namespace: "home"},
		Data:       map[string]string{mainConfigKey: "logging:\n  level: \"warn\"\n"},
	}, false)
	if config.Logging.Level != "warn" {
		t.Errorf("main ConfigMap not applied, level is %q", config.Logging.Level)
	}
	if _, _, ok := dashboardItems.read("static/fromcm"); !ok {
		t.Error("items of the items ConfigMap lost on a main ConfigMap event")
	}
	// the values main.yaml says nothing about stay at their defaults
	if config.Content_filters.Item.Mode != "include" {
		t.Errorf("main.yaml defaults lost, item filter mode is %q", config.Content_filters.Item.Mode)
	}
}

// A ConfigMap of the watched name which does not hold the configuration file it stands
// for is ignored: reading it empty would take the items away from the dashboard.
func TestConfigurationConfigMapWithoutItsFileIsIgnored(t *testing.T) {
	isolateConfig(t, "")
	savedMain, savedItems := lastMainYAML, lastItemsYAML
	t.Cleanup(func() { lastMainYAML, lastItemsYAML = savedMain, savedItems })

	reloadIfChanged(nil, []byte("items:\n- name: kept\n  namespace: static\n  url: http://127.0.0.1:1/\n"))
	content := newConfigMapContent()

	configMapEventHandlers(content, itemsConfigKey).OnAdd(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: itemsConfigMapName, Namespace: "home"},
		Data:       map[string]string{"something_else.yaml": "items: []"},
	}, false)
	if _, _, ok := dashboardItems.read("static/kept"); !ok {
		t.Error("items of an unrelated ConfigMap replaced by nothing")
	}
}

// Out of the cluster there is no namespace to look for configuration ConfigMaps in, so
// nothing is tried and the configuration files keep being polled.
func TestConfigMapsAreNotWatchedWithoutANamespace(t *testing.T) {
	if namespace := serviceAccountNamespace(); namespace != "" {
		t.Skipf("running in the namespace %q, the out of cluster case does not happen here", namespace)
	}
	watchConfigMaps(&rest.Config{})
}
