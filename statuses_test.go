package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The endpoint used to fetch whatever URL was put into the query, which turned
// the dashboard into an open proxy for whoever could reach it.
func TestStatusCheckAcceptsOnlyDashboardItems(t *testing.T) {
	httpClient = newHttpClient(false)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()

	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer forbidden.Close()

	secret := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer secret.Close()

	// httptest serves a self signed certificate, which the client must not accept
	badCert := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer badCert.Close()

	dashboardItems = DashboardItemsStore{
		items: map[string]DashEntry{
			"default/up":        {Namespace: "default", Name: "up", URL: upstream.URL},
			"default/down":      {Namespace: "default", Name: "down", URL: "http://127.0.0.1:9/"},
			"default/forbidden": {Namespace: "default", Name: "forbidden", URL: forbidden.URL},
			"default/secret":    {Namespace: "default", Name: "secret", URL: secret.URL},
			"default/bad-cert":  {Namespace: "default", Name: "bad-cert", URL: badCert.URL},
		},
		versions: map[string]int64{"default/up": 1, "default/down": 1},
	}

	for key, expected := range map[string]int{
		"default/up":        http.StatusOK,
		"default/forbidden": http.StatusOK,
		"default/secret":    http.StatusOK,
		"default/bad-cert":  http.StatusFailedDependency,
		"default/down":      http.StatusInternalServerError,
		"other/not-an-item": http.StatusNotFound,
		"http://169.254.169.254/latest/meta-data/": http.StatusNotFound,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/statusCheck/?item="+key, nil)
		statusCheckHandler(recorder, request)
		if recorder.Code != expected {
			t.Errorf("item %q: got status %d, expected %d", key, recorder.Code, expected)
		}
	}
}
