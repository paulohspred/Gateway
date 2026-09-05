package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/core"
	"github.com/paulohspred/Gateway/internal/metrics"
)

func TestHandlerOperationalEndpoints(t *testing.T) {
	sessions := core.NewSessionRegistry()
	sessions.Open(core.Session{ID: "session-b", ListenerID: "b"})
	sessions.Open(core.Session{ID: "session-a", ListenerID: "a"})
	m := metrics.New()
	m.Inc("rc_gateway_test_total")
	s := &Server{NodeID: "gw-test", Sessions: sessions, Metrics: m}
	h := s.handler()

	request := func(method, path string) *httptest.ResponseRecorder {
		t.Helper()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		h.ServeHTTP(rr, req)
		if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s missing nosniff header", path)
		}
		return rr
	}

	if rr := request(http.MethodGet, "/healthz"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthz unexpected response: code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr := request(http.MethodGet, "/readyz"); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz before readiness: got %d", rr.Code)
	}
	s.SetReady(true)
	if rr := request(http.MethodGet, "/readyz"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"status":"ready"`) {
		t.Fatalf("readyz after readiness: code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr := request(http.MethodGet, "/status"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"nodeId":"gw-test"`) || !strings.Contains(rr.Body.String(), `"activeSessions":2`) {
		t.Fatalf("status unexpected response: code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr := request(http.MethodGet, "/sessions"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"id":"session-a"`) || !strings.Contains(rr.Body.String(), `"id":"session-b"`) {
		t.Fatalf("sessions unexpected response: code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr := request(http.MethodGet, "/metrics"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "rc_gateway_test_total 1") {
		t.Fatalf("metrics unexpected response: code=%d body=%q", rr.Code, rr.Body.String())
	}
	if rr := request(http.MethodPost, "/healthz"); rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST healthz should be 405, got %d", rr.Code)
	}
}

func TestRunSignalsListeningAndStops(t *testing.T) {
	s := &Server{
		Bind:     "127.0.0.1:0",
		NodeID:   "gw-test",
		Sessions: core.NewSessionRegistry(),
		Metrics:  metrics.New(),
	}
	listening := make(chan struct{})
	s.OnListening = func() { close(listening) }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	select {
	case <-listening:
	case <-time.After(2 * time.Second):
		t.Fatal("admin server did not signal listening")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("admin Run returned error on shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("admin server did not stop after cancellation")
	}
}
