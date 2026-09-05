package admin

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/paulohspred/Gateway/internal/core"
	"github.com/paulohspred/Gateway/internal/metrics"
)

type Server struct {
	Bind        string
	NodeID      string
	Sessions    *core.SessionRegistry
	Metrics     *metrics.Registry
	OnListening func()
	ready       atomic.Bool
}

func (s *Server) SetReady(v bool) { s.ready.Store(v) }

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	register := func(path string, handler http.HandlerFunc) {
		mux.HandleFunc("GET "+path, handler)
		mux.HandleFunc("GET /v1"+path, handler)
	}

	register("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"status":"ok","apiVersion":"v1"}` + "\n"))
	})
	register("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !s.ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready","apiVersion":"v1"}` + "\n"))
	})
	register("/sessions", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(s.Sessions.Snapshot())
	})
	register("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Header().Set("Cache-Control", "no-store")
		s.Metrics.WritePrometheus(w)
	})
	register("/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		ready := s.ready.Load()
		active := s.Sessions.Count()
		state := "not_ready"
		if ready {
			state = "ready_idle"
			if active > 0 {
				state = "active"
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"apiVersion":       "v1",
			"nodeId":           s.NodeID,
			"ready":            ready,
			"operationalState": state,
			"activeSessions":   active,
			"commandPlane":     "disabled",
		})
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.Bind,
		Handler:           s.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	ln, err := net.Listen("tcp", s.Bind)
	if err != nil {
		return err
	}
	if s.OnListening != nil {
		s.OnListening()
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
