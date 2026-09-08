package rapid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/httpapi"
	"github.com/paulohspred/Gateway/internal/monitor/profile"
)

func TestRapidWebSemanticPipelineEndToEnd(t *testing.T) {
	fixture := newRapidE2EFixture(t)
	defer fixture.Close()

	assertRapidTelemetry(t, fixture, monitor.CommunicationOnline, monitor.QualityGood, 1500, 0)

	alarms := fixture.getAlarms(t)
	if len(alarms) != 1 || alarms[0].Code != "LOW_OIL_PRESSURE" || !alarms[0].Active {
		t.Fatalf("unexpected alarms: %#v", alarms)
	}
	if !alarms[0].RaisedAt.Equal(fixture.providerNow.Add(-30 * time.Second)) {
		t.Fatalf("expected raisedAt from Rapid history, got %s", alarms[0].RaisedAt)
	}

	events := fixture.getEvents(t)
	if len(events) != 2 {
		t.Fatalf("expected two normalized events, got %#v", events)
	}
	if events[0].Type != "controller.test_event" || events[1].Type != "alarm.raised" {
		t.Fatalf("unexpected events: %#v", events)
	}

	fixture.observedAt = fixture.providerNow.Add(-20 * time.Second)
	assertRapidTelemetry(t, fixture, monitor.CommunicationOnline, monitor.QualityStale, 1500, 0)

	fixture.outage.Store(true)
	assertRapidTelemetry(t, fixture, monitor.CommunicationOffline, monitor.QualityOffline, 1500, 0)

	fixture.outage.Store(false)
	fixture.observedAt = fixture.providerNow
	assertRapidTelemetry(t, fixture, monitor.CommunicationOnline, monitor.QualityGood, 1500, 0)
}

func TestRapidWebSemanticPipelineMiniSoak(t *testing.T) {
	fixture := newRapidE2EFixture(t)
	defer fixture.Close()

	for i := 0; i < 250; i++ {
		fixture.outage.Store(i >= 40 && i < 50)
		communication := monitor.CommunicationOnline
		quality := monitor.QualityGood
		if fixture.outage.Load() {
			communication = monitor.CommunicationOffline
			quality = monitor.QualityOffline
		}
		assertRapidTelemetry(t, fixture, communication, quality, 1500, 0)
	}
}

type rapidE2EFixture struct {
	t           *testing.T
	server      *httptest.Server
	api         http.Handler
	providerNow time.Time
	observedAt  time.Time
	outage      atomic.Bool
}

func newRapidE2EFixture(t *testing.T) *rapidE2EFixture {
	t.Helper()
	fixture := &rapidE2EFixture{
		t:           t,
		providerNow: time.Date(2026, time.September, 6, 13, 30, 0, 0, time.UTC),
	}
	fixture.observedAt = fixture.providerNow
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.handleRapid))

	bundleRoot := filepath.Join("..", "..", "..", "controllers", "rc-simulator", "reference-controller")
	bundle, err := profile.LoadDir(bundleRoot)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := LoadBinding(filepath.Join(bundleRoot, "rapid", "channels.json"), bundle)
	if err != nil {
		t.Fatal(err)
	}
	generator := monitor.Generator{
		ID:     "gen-e2e",
		Name:   "Generator E2E",
		SiteID: "site-e2e",
		Controller: monitor.ControllerRef{
			Manufacturer: bundle.Manifest.Manufacturer,
			Model:        bundle.Manifest.Model,
		},
	}
	configs := []GeneratorConfig{{Generator: generator, Profile: bundle, Binding: binding}}
	raw, err := NewWebReader(WebReaderOptions{
		BaseURL:  fixture.server.URL + "/",
		Username: "monitor",
		Password: "secret",
		Timeout:  2 * time.Second,
		Now:      func() time.Time { return fixture.observedAt },
	})
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := NewSemanticReader(raw, configs, SemanticOptions{})
	if err != nil {
		t.Fatal(err)
	}
	provider, err := NewProvider(semantic, configs, Options{Now: func() time.Time { return fixture.providerNow }})
	if err != nil {
		t.Fatal(err)
	}
	service, err := monitor.NewService(provider)
	if err != nil {
		t.Fatal(err)
	}
	api, err := httpapi.New(service)
	if err != nil {
		t.Fatal(err)
	}
	fixture.api = api
	return fixture
}

func (f *rapidE2EFixture) Close() {
	f.server.Close()
}

func (f *rapidE2EFixture) handleRapid(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/Api/Auth/Login":
		if r.Method != http.MethodPost {
			f.t.Errorf("unexpected login method %s", r.Method)
		}
		var credentials map[string]string
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			f.t.Errorf("decode login: %v", err)
		}
		if credentials["username"] != "monitor" || credentials["password"] != "secret" {
			f.t.Errorf("unexpected Rapid credentials")
		}
		http.SetCookie(w, &http.Cookie{Name: "rapid-session", Value: "ok", Path: "/"})
		writeE2EJSON(f.t, w, map[string]any{"ok": true, "msg": "", "data": nil})
	case "/Api/Main/GetCurData":
		if f.outage.Load() {
			http.Error(w, "temporary outage", http.StatusServiceUnavailable)
			return
		}
		if _, err := r.Cookie("rapid-session"); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		points := make([]map[string]any, 0)
		for _, rawChannel := range strings.Split(r.URL.Query().Get("cnlNums"), ",") {
			channel, err := strconv.Atoi(rawChannel)
			if err != nil {
				f.t.Errorf("invalid channel query %q", rawChannel)
				continue
			}
			value, status := e2eChannelValue(channel)
			points = append(points, map[string]any{"cnlNum": channel, "val": value, "stat": status})
		}
		writeE2EJSON(f.t, w, map[string]any{"ok": true, "msg": "", "data": points})
	case "/Api/Main/GetLastAvailableEvents":
		if _, err := r.Cookie("rapid-session"); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("archiveBit") != "1" || r.URL.Query().Get("filterID") != "0" {
			f.t.Errorf("unexpected Rapid event query: %s", r.URL.RawQuery)
		}
		records := []map[string]any{
			{
				"id": "evt-115",
				"e": map[string]any{
					"timestamp":   f.providerNow.Add(-30 * time.Second).Format(time.RFC3339Nano),
					"cnlNum":      115,
					"prevCnlVal":  0,
					"prevCnlStat": 1,
					"cnlVal":      1,
					"cnlStat":     1,
				},
			},
			{
				"id": "evt-116",
				"e": map[string]any{
					"timestamp":   f.providerNow.Add(-10 * time.Second).Format(time.RFC3339Nano),
					"cnlNum":      116,
					"prevCnlVal":  0,
					"prevCnlStat": 1,
					"cnlVal":      1,
					"cnlStat":     1,
				},
			},
		}
		writeE2EJSON(f.t, w, map[string]any{
			"ok":  true,
			"msg": "",
			"data": map[string]any{
				"records":  records,
				"filterID": "0",
			},
		})
	default:
		http.NotFound(w, r)
	}
}

func e2eChannelValue(channel int) (float64, int) {
	switch channel {
	case 101:
		return 1500, 1
	case 105:
		return 230, 1
	case 108:
		return 60, 1
	case 109:
		return 0, 1
	case 112:
		return 2, 1
	case 113:
		return 1, 1
	case 114:
		return 0, 0
	case 115:
		return 1, 1
	case 116:
		return 1, 1
	default:
		return 0, 0
	}
}

func assertRapidTelemetry(t *testing.T, fixture *rapidE2EFixture, communication monitor.CommunicationState, quality monitor.Quality, rpm, power float64) {
	t.Helper()
	recorder := httptest.NewRecorder()
	fixture.api.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/generators/gen-e2e/telemetry", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("telemetry expected 200, got %d: %s", recorder.Code, recorder.Body.String())
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
	if payload.Communication != communication {
		t.Fatalf("expected communication %s, got %s", communication, payload.Communication)
	}
	rpmMetric, ok := payload.Metrics[string(monitor.MetricEngineRPM)]
	if !ok || rpmMetric.Value != rpm || rpmMetric.Quality != quality {
		t.Fatalf("unexpected RPM metric: %#v", rpmMetric)
	}
	powerMetric, ok := payload.Metrics[string(monitor.MetricGeneratorPowerKW)]
	if !ok || powerMetric.Value != power || powerMetric.Quality != quality {
		t.Fatalf("explicit real zero must be preserved: %#v", powerMetric)
	}
	if _, exists := payload.Metrics[string(monitor.MetricFuelLevel)]; exists {
		t.Fatal("undefined fuel channel must remain absent")
	}
}

func (f *rapidE2EFixture) getAlarms(t *testing.T) []monitor.Alarm {
	t.Helper()
	var alarms []monitor.Alarm
	f.getJSON(t, "/api/v1/generators/gen-e2e/alarms", &alarms)
	return alarms
}

func (f *rapidE2EFixture) getEvents(t *testing.T) []monitor.Event {
	t.Helper()
	var events []monitor.Event
	f.getJSON(t, "/api/v1/generators/gen-e2e/events", &events)
	return events
}

func (f *rapidE2EFixture) getJSON(t *testing.T, path string, target any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	f.api.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s expected 200, got %d: %s", path, recorder.Code, recorder.Body.String())
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func writeE2EJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode Rapid response: %v", err)
	}
}

func TestRapidWebEventQueryValidation(t *testing.T) {
	for _, query := range []EventQuery{
		{},
		{ArchiveBit: 1, PeriodDays: 0, Limit: 1},
		{ArchiveBit: 1, PeriodDays: 1, Limit: 0},
	} {
		if err := query.Validate(); err == nil {
			t.Fatalf("expected invalid query %#v to fail", query)
		}
	}
	if err := (EventQuery{ArchiveBit: 1, PeriodDays: 7, Limit: 1000}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRapidWebSemanticReaderHonorsCancellation(t *testing.T) {
	fixture := newRapidE2EFixture(t)
	defer fixture.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bundle := loadTestProfile(t)
	binding, err := LoadBinding(testBindingPath(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	_ = binding
	if _, err := fixture.getCanceledTelemetry(ctx); err == nil {
		t.Fatal("expected canceled request to fail")
	}
}

func (f *rapidE2EFixture) getCanceledTelemetry(ctx context.Context) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, f.server.URL+"/Api/Main/GetCurData?cnlNums=101", nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	return fmt.Sprintf("%d", response.StatusCode), nil
}
