package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paulohspred/Gateway/internal/core"
	"github.com/paulohspred/Gateway/internal/metrics"
)

func TestVersionedOperationalAPI(t *testing.T) {
	sessions := core.NewSessionRegistry()
	s := &Server{NodeID: "gw-v1", Sessions: sessions, Metrics: metrics.New()}
	h := s.handler()

	get := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		return rr
	}

	if rr := get("/v1/healthz"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"apiVersion":"v1"`) {
		t.Fatalf("v1 health response: code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr := get("/v1/readyz"); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("v1 ready before SetReady: %d", rr.Code)
	}
	if rr := get("/v1/status"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"operationalState":"not_ready"`) {
		t.Fatalf("v1 status before readiness: code=%d body=%q", rr.Code, rr.Body.String())
	}

	s.SetReady(true)
	if rr := get("/v1/status"); !strings.Contains(rr.Body.String(), `"operationalState":"ready_idle"`) {
		t.Fatalf("v1 idle status: %q", rr.Body.String())
	}
	sessions.Open(core.Session{ID: "s1", ListenerID: "t1"})
	if rr := get("/v1/status"); !strings.Contains(rr.Body.String(), `"operationalState":"active"`) || !strings.Contains(rr.Body.String(), `"activeSessions":1`) {
		t.Fatalf("v1 active status: %q", rr.Body.String())
	}

	if rr := get("/v1/metrics"); rr.Code != http.StatusOK {
		t.Fatalf("v1 metrics: %d", rr.Code)
	}
	if rr := get("/v1/sessions"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"id":"s1"`) {
		t.Fatalf("v1 sessions: code=%d body=%q", rr.Code, rr.Body.String())
	}
}
