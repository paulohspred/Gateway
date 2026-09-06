package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpointIsLocalReadOnlyPrometheusText(t *testing.T) {
	server := newTestServer(t, newFakeProvider())

	for _, path := range []string{"/healthz", "/api/v1/generators"} {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, recorder.Code)
		}
	}

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("unexpected content type %q", got)
	}
	body := recorder.Body.String()
	for _, metric := range []string{"rc_monitor_uptime_seconds", "rc_monitor_http_requests_total 3"} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metrics output missing %q: %s", metric, body)
		}
	}

	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/metrics", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected POST /metrics to fail with 405, got %d", recorder.Code)
	}
}
