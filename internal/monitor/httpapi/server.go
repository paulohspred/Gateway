package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
)

const APIVersion = "v1"

type Server struct {
	service      *monitor.Service
	handler      http.Handler
	startedAt    time.Time
	requestCount atomic.Uint64
}

func New(service *monitor.Service) (*Server, error) {
	if service == nil {
		return nil, errors.New("monitor service is required")
	}
	s := &Server{service: service, startedAt: time.Now().UTC()}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/api/v1/system/health", s.handleSystemHealth)
	mux.HandleFunc("/api/v1/generators", s.handleGenerators)
	mux.HandleFunc("/api/v1/generators/", s.handleGeneratorResource)
	mux.HandleFunc("/", s.handleNotFound)
	s.handler = securityHeaders(s.countRequests(getOnly(mux)))
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

type statusResponse struct {
	Status     string `json:"status"`
	APIVersion string `json:"apiVersion"`
}

type readyResponse struct {
	Status         string                 `json:"status"`
	APIVersion     string                 `json:"apiVersion"`
	ProviderStatus monitor.ProviderStatus `json:"providerStatus"`
}

type systemHealthResponse struct {
	Status     string                 `json:"status"`
	APIVersion string                 `json:"apiVersion"`
	Provider   monitor.ProviderHealth `json:"provider"`
}

type metricResponse struct {
	Value      any             `json:"value"`
	Unit       string          `json:"unit,omitempty"`
	Quality    monitor.Quality `json:"quality"`
	ObservedAt time.Time       `json:"observedAt"`
}

type telemetryResponse struct {
	GeneratorID   string                     `json:"generatorId"`
	CapturedAt    time.Time                  `json:"capturedAt"`
	Communication monitor.CommunicationState `json:"communication"`
	Metrics       map[string]metricResponse  `json:"metrics"`
}

type apiError struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok", APIVersion: APIVersion})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	health, err := s.service.Health(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if health.Status != monitor.ProviderHealthy {
		writeJSON(w, http.StatusServiceUnavailable, readyResponse{
			Status:         "not_ready",
			APIVersion:     APIVersion,
			ProviderStatus: health.Status,
		})
		return
	}
	writeJSON(w, http.StatusOK, readyResponse{
		Status:         "ready",
		APIVersion:     APIVersion,
		ProviderStatus: health.Status,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	uptime := time.Since(s.startedAt).Seconds()
	if uptime < 0 {
		uptime = 0
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w,
		"# HELP rc_monitor_uptime_seconds Process uptime in seconds.\n"+
			"# TYPE rc_monitor_uptime_seconds gauge\n"+
			"rc_monitor_uptime_seconds %.3f\n"+
			"# HELP rc_monitor_http_requests_total HTTP requests handled by RC Monitor.\n"+
			"# TYPE rc_monitor_http_requests_total counter\n"+
			"rc_monitor_http_requests_total %d\n",
		uptime, s.requestCount.Load())
}

func (s *Server) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	health, err := s.service.Health(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	status := "healthy"
	httpStatus := http.StatusOK
	if health.Status != monitor.ProviderHealthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}
	writeJSON(w, httpStatus, systemHealthResponse{
		Status:     status,
		APIVersion: APIVersion,
		Provider:   health,
	})
}

func (s *Server) handleGenerators(w http.ResponseWriter, r *http.Request) {
	generators, err := s.service.ListGenerators(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, generators)
}

func (s *Server) handleGeneratorResource(w http.ResponseWriter, r *http.Request) {
	remainder := strings.TrimPrefix(r.URL.Path, "/api/v1/generators/")
	remainder = strings.Trim(remainder, "/")
	if remainder == "" {
		s.handleNotFound(w, r)
		return
	}
	parts := strings.Split(remainder, "/")
	if len(parts) > 2 {
		s.handleNotFound(w, r)
		return
	}
	id := strings.TrimSpace(parts[0])
	if id == "" {
		s.handleNotFound(w, r)
		return
	}
	if len(parts) == 1 {
		generator, err := s.service.GetGenerator(r.Context(), id)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, generator)
		return
	}

	switch parts[1] {
	case "telemetry":
		snapshot, err := s.service.GetTelemetry(r.Context(), id)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		response, err := toTelemetryResponse(snapshot)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response)
	case "alarms":
		alarms, err := s.service.GetAlarms(r.Context(), id)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, alarms)
	case "events":
		events, err := s.service.GetEvents(r.Context(), id)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, events)
	default:
		s.handleNotFound(w, r)
	}
}

func (s *Server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "resource not found")
}

func toTelemetryResponse(snapshot monitor.TelemetrySnapshot) (telemetryResponse, error) {
	if err := snapshot.Validate(); err != nil {
		return telemetryResponse{}, fmt.Errorf("invalid telemetry snapshot: %w", err)
	}
	metrics := make(map[string]metricResponse, len(snapshot.Metrics))
	for key, metric := range snapshot.Metrics {
		value, err := scalarValue(metric.Value)
		if err != nil {
			return telemetryResponse{}, fmt.Errorf("metric %q: %w", key, err)
		}
		metrics[string(key)] = metricResponse{
			Value:      value,
			Unit:       metric.Unit,
			Quality:    metric.Quality,
			ObservedAt: metric.ObservedAt,
		}
	}
	return telemetryResponse{
		GeneratorID:   snapshot.GeneratorID,
		CapturedAt:    snapshot.CapturedAt,
		Communication: snapshot.Communication,
		Metrics:       metrics,
	}, nil
}

func scalarValue(value monitor.MetricValue) (any, error) {
	kind, err := value.Kind()
	if err != nil {
		return nil, err
	}
	switch kind {
	case monitor.ValueNumber:
		return *value.Number, nil
	case monitor.ValueText:
		return *value.Text, nil
	case monitor.ValueBoolean:
		return *value.Boolean, nil
	default:
		return nil, fmt.Errorf("unsupported metric value kind %q", kind)
	}
}

func (s *Server) countRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requestCount.Add(1)
		next.ServeHTTP(w, r)
	})
}

func getOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, monitor.ErrGeneratorNotFound):
		writeError(w, http.StatusNotFound, "generator_not_found", "generator not found")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "provider_timeout", "provider request timed out")
	case errors.Is(err, context.Canceled):
		writeError(w, http.StatusRequestTimeout, "request_canceled", "request canceled")
	default:
		writeError(w, http.StatusBadGateway, "provider_error", "provider request failed")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiError{Error: errorBody{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
