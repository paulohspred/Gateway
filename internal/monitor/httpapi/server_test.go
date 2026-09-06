package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/fake"
)

var testNow = time.Date(2026, time.September, 6, 10, 30, 0, 0, time.UTC)

func newTestServer(t *testing.T, provider monitor.Provider) *Server {
	t.Helper()
	service, err := monitor.NewService(provider)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(service)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func newFakeProvider() *fake.Provider {
	return fake.NewProvider(fake.Options{Now: func() time.Time { return testNow }})
}

func TestHealthAndReady(t *testing.T) {
	server := newTestServer(t, newFakeProvider())

	for _, path := range []string{"/healthz", "/readyz"} {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", path, recorder.Code, recorder.Body.String())
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%s: expected no-store, got %q", path, got)
		}
	}
}

func TestReadyFailsClosedWhenProviderIsDegraded(t *testing.T) {
	provider := newFakeProvider()
	if err := provider.SetProviderStatus(monitor.ProviderDegraded); err != nil {
		t.Fatal(err)
	}
	server := newTestServer(t, provider)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", recorder.Code, recorder.Body.String())
	}
	assertErrorFreeJSON(t, recorder)
}

func TestGeneratorCollectionAndDetail(t *testing.T) {
	server := newTestServer(t, newFakeProvider())

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var generators []monitor.Generator
	if err := json.Unmarshal(recorder.Body.Bytes(), &generators); err != nil {
		t.Fatal(err)
	}
	if len(generators) != 1 || generators[0].ID != "gen-sim-001" {
		t.Fatalf("unexpected generators: %#v", generators)
	}

	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/gen-sim-001", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var generator monitor.Generator
	if err := json.Unmarshal(recorder.Body.Bytes(), &generator); err != nil {
		t.Fatal(err)
	}
	if generator.Controller.Manufacturer != "RC Simulator" {
		t.Fatalf("unexpected generator: %#v", generator)
	}
}

func TestTelemetryFlattensTypedValuesAndKeepsAbsentMetricAbsent(t *testing.T) {
	server := newTestServer(t, newFakeProvider())
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/gen-sim-001/telemetry", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Communication monitor.CommunicationState `json:"communication"`
		Metrics       map[string]struct {
			Value   any             `json:"value"`
			Quality monitor.Quality `json:"quality"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Communication != monitor.CommunicationOnline {
		t.Fatalf("unexpected communication state: %s", payload.Communication)
	}
	if got := payload.Metrics[string(monitor.MetricEngineRPM)].Value; got != float64(1500) {
		t.Fatalf("expected scalar RPM 1500, got %#v", got)
	}
	if got := payload.Metrics[string(monitor.MetricControllerMode)].Value; got != "auto" {
		t.Fatalf("expected scalar controller mode, got %#v", got)
	}
	if got := payload.Metrics[string(monitor.MetricBreakerGCB)].Value; got != true {
		t.Fatalf("expected scalar breaker value true, got %#v", got)
	}
	if _, exists := payload.Metrics[string(monitor.MetricFuelLevel)]; exists {
		t.Fatal("fuel.level must remain absent from API JSON")
	}
}

func TestTelemetryPreservesRealNumericZero(t *testing.T) {
	base := newFakeProvider()
	server := newTestServer(t, zeroTelemetryProvider{Provider: base})
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/gen-sim-001/telemetry", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Metrics map[string]struct {
			Value any `json:"value"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	metric, exists := payload.Metrics[string(monitor.MetricGeneratorPowerKW)]
	if !exists {
		t.Fatal("expected generator.power_kw to be present")
	}
	if metric.Value != float64(0) {
		t.Fatalf("expected explicit zero, got %#v", metric.Value)
	}
}

func TestStaleOfflineAlarmsAndEvents(t *testing.T) {
	provider := newFakeProvider()
	server := newTestServer(t, provider)

	if err := provider.SetScenario(fake.ScenarioStale); err != nil {
		t.Fatal(err)
	}
	assertMetricQuality(t, server, monitor.QualityStale)

	if err := provider.SetScenario(fake.ScenarioOffline); err != nil {
		t.Fatal(err)
	}
	assertMetricQuality(t, server, monitor.QualityOffline)

	if err := provider.SetScenario(fake.ScenarioAlarm); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/gen-sim-001/alarms", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var alarms []monitor.Alarm
	if err := json.Unmarshal(recorder.Body.Bytes(), &alarms); err != nil {
		t.Fatal(err)
	}
	if len(alarms) != 1 || !alarms[0].Active {
		t.Fatalf("unexpected alarms: %#v", alarms)
	}

	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/gen-sim-001/events", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestNotFoundMethodAndProviderErrorsAreStable(t *testing.T) {
	base := newFakeProvider()
	server := newTestServer(t, base)

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/missing", nil))
	assertError(t, recorder, http.StatusNotFound, "generator_not_found")

	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/generators", nil))
	assertError(t, recorder, http.StatusMethodNotAllowed, "method_not_allowed")
	if recorder.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("expected Allow: GET, got %q", recorder.Header().Get("Allow"))
	}

	providerFailure := errors.New("sensitive provider detail")
	server = newTestServer(t, generatorErrorProvider{Provider: base, err: providerFailure})
	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/gen-sim-001", nil))
	assertError(t, recorder, http.StatusBadGateway, "provider_error")
	if recorder.Body.String() == providerFailure.Error() {
		t.Fatal("raw provider error must not be exposed")
	}
}

func TestCanceledAndTimedOutProviderRequestsHaveDistinctErrors(t *testing.T) {
	server := newTestServer(t, newFakeProvider())

	request := httptest.NewRequest(http.MethodGet, "/api/v1/generators", nil)
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request.WithContext(ctx))
	assertError(t, recorder, http.StatusRequestTimeout, "request_canceled")

	request = httptest.NewRequest(http.MethodGet, "/api/v1/generators", nil)
	ctx, cancel = context.WithDeadline(request.Context(), time.Now().Add(-time.Second))
	defer cancel()
	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, request.WithContext(ctx))
	assertError(t, recorder, http.StatusGatewayTimeout, "provider_timeout")
}

func assertMetricQuality(t *testing.T, server *Server, expected monitor.Quality) {
	t.Helper()
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/gen-sim-001/telemetry", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Metrics map[string]struct {
			Quality monitor.Quality `json:"quality"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload.Metrics[string(monitor.MetricEngineRPM)].Quality; got != expected {
		t.Fatalf("expected quality %s, got %s", expected, got)
	}
}

func assertError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, recorder.Code, recorder.Body.String())
	}
	var payload apiError
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != code {
		t.Fatalf("expected error code %q, got %#v", code, payload)
	}
}

func assertErrorFreeJSON(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var payload any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v; body=%s", err, recorder.Body.String())
	}
}

type zeroTelemetryProvider struct {
	monitor.Provider
}

func (p zeroTelemetryProvider) GetTelemetry(ctx context.Context, id string) (monitor.TelemetrySnapshot, error) {
	snapshot, err := p.Provider.GetTelemetry(ctx, id)
	if err != nil {
		return monitor.TelemetrySnapshot{}, err
	}
	metric := snapshot.Metrics[monitor.MetricGeneratorPowerKW]
	metric.Value = monitor.NumberValue(0)
	snapshot.Metrics[monitor.MetricGeneratorPowerKW] = metric
	return snapshot, nil
}

type generatorErrorProvider struct {
	monitor.Provider
	err error
}

func (p generatorErrorProvider) GetGenerator(context.Context, string) (monitor.Generator, error) {
	return monitor.Generator{}, p.err
}
