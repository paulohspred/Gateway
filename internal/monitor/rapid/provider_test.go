package rapid

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/profile"
)

var testNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

type stubReader struct {
	data      []ChannelData
	readErr   error
	healthErr error
	alarms    []monitor.Alarm
	events    []monitor.Event
}

func (r *stubReader) ReadCurrent(ctx context.Context, _ []int) ([]ChannelData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.readErr != nil {
		return nil, r.readErr
	}
	return append([]ChannelData(nil), r.data...), nil
}

func (r *stubReader) ReadAlarms(ctx context.Context, _ string) ([]monitor.Alarm, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]monitor.Alarm(nil), r.alarms...), nil
}

func (r *stubReader) ReadEvents(ctx context.Context, _ string) ([]monitor.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]monitor.Event(nil), r.events...), nil
}

func (r *stubReader) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.healthErr
}

func TestRapidProviderMapsCurrentDataWithoutInventingUndefinedValues(t *testing.T) {
	reader := &stubReader{data: []ChannelData{
		{ChannelNumber: 101, Value: 0, Status: 1, ObservedAt: testNow},
		{ChannelNumber: 105, Value: 230.1, Status: 1, ObservedAt: testNow},
		{ChannelNumber: 108, Value: 60.02, Status: 1, ObservedAt: testNow},
		{ChannelNumber: 112, Value: 2, Status: 1, ObservedAt: testNow},
		{ChannelNumber: 113, Value: 1, Status: 1, ObservedAt: testNow},
		{ChannelNumber: 114, Value: 0, Status: 0, ObservedAt: testNow},
	}}
	provider := newTestProvider(t, reader)

	snapshot, err := provider.GetTelemetry(context.Background(), "gen-rapid-test")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Communication != monitor.CommunicationOnline {
		t.Fatalf("expected online communication, got %s", snapshot.Communication)
	}
	if rpm, ok := snapshot.Metrics[monitor.MetricEngineRPM]; !ok || rpm.Value.Number == nil || *rpm.Value.Number != 0 {
		t.Fatalf("real numeric zero must be preserved: %#v", rpm)
	}
	if _, ok := snapshot.Metrics[monitor.MetricFuelLevel]; ok {
		t.Fatal("undefined Rapid channel must remain absent")
	}
	mode := snapshot.Metrics[monitor.MetricControllerMode]
	if mode.Value.Text == nil || *mode.Value.Text != "auto" {
		t.Fatalf("expected enum transform to auto, got %#v", mode.Value)
	}
	breaker := snapshot.Metrics[monitor.MetricBreakerGCB]
	if breaker.Value.Boolean == nil || !*breaker.Value.Boolean {
		t.Fatalf("expected boolean transform true, got %#v", breaker.Value)
	}
}

func TestRapidProviderMarksOldDefinedSampleStale(t *testing.T) {
	reader := &stubReader{data: []ChannelData{
		{ChannelNumber: 101, Value: 1500, Status: 1, ObservedAt: testNow.Add(-11 * time.Second)},
		{ChannelNumber: 105, Value: 230, Status: 1, ObservedAt: testNow},
		{ChannelNumber: 108, Value: 60, Status: 1, ObservedAt: testNow},
	}}
	provider := newTestProvider(t, reader)
	snapshot, err := provider.GetTelemetry(context.Background(), "gen-rapid-test")
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Metrics[monitor.MetricEngineRPM].Quality; got != monitor.QualityStale {
		t.Fatalf("expected stale RPM, got %s", got)
	}
}

func TestRapidProviderKeepsLastKnownAsOfflineOnReaderFailure(t *testing.T) {
	reader := &stubReader{data: []ChannelData{
		{ChannelNumber: 101, Value: 1500, Status: 1, ObservedAt: testNow},
		{ChannelNumber: 105, Value: 230, Status: 1, ObservedAt: testNow},
		{ChannelNumber: 108, Value: 60, Status: 1, ObservedAt: testNow},
	}}
	provider := newTestProvider(t, reader)
	if _, err := provider.GetTelemetry(context.Background(), "gen-rapid-test"); err != nil {
		t.Fatal(err)
	}
	reader.readErr = errors.New("Rapid adapter unavailable")

	snapshot, err := provider.GetTelemetry(context.Background(), "gen-rapid-test")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Communication != monitor.CommunicationOffline {
		t.Fatalf("expected offline communication, got %s", snapshot.Communication)
	}
	if got := snapshot.Metrics[monitor.MetricEngineRPM].Quality; got != monitor.QualityOffline {
		t.Fatalf("expected last-known RPM quality offline, got %s", got)
	}
}

func TestRapidProviderHealthIsUnavailableWhenReaderFails(t *testing.T) {
	reader := &stubReader{healthErr: errors.New("down")}
	provider := newTestProvider(t, reader)
	health, err := provider.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != monitor.ProviderUnavailable {
		t.Fatalf("expected unavailable health, got %s", health.Status)
	}
}

func TestRapidProviderRejectsAlarmAndEventForWrongGenerator(t *testing.T) {
	reader := &stubReader{
		alarms: []monitor.Alarm{{ID: "a", GeneratorID: "wrong"}},
		events: []monitor.Event{{ID: "e", GeneratorID: "wrong"}},
	}
	provider := newTestProvider(t, reader)
	if _, err := provider.GetAlarms(context.Background(), "gen-rapid-test"); err == nil {
		t.Fatal("expected wrong-generator alarm to fail")
	}
	if _, err := provider.GetEvents(context.Background(), "gen-rapid-test"); err == nil {
		t.Fatal("expected wrong-generator event to fail")
	}
}

func TestRapidBindingRequiresAllRequiredProfileMetrics(t *testing.T) {
	bundle := loadTestProfile(t)
	binding := BindingFile{
		Schema:    BindingSchemaVersion,
		ProfileID: bundle.Manifest.ID,
		Metrics: []ChannelBinding{
			{Key: monitor.MetricEngineRPM, ChannelNumber: 1, Transform: Transform{Kind: TransformNumber}},
		},
	}
	if err := binding.Validate(bundle); err == nil {
		t.Fatal("expected missing required Rapid bindings to fail")
	}
}

func TestTransformFailsClosedForUnmappedDiscreteValues(t *testing.T) {
	booleanTransform := Transform{Kind: TransformBoolean, TrueValues: []float64{1}, FalseValues: []float64{0}}
	if _, err := booleanTransform.Apply(2, monitor.ValueBoolean); err == nil {
		t.Fatal("expected unmapped boolean value to fail")
	}
	enumTransform := Transform{Kind: TransformEnum, EnumValues: map[string]string{"0": "off", "1": "auto"}}
	if _, err := enumTransform.Apply(2, monitor.ValueText); err == nil {
		t.Fatal("expected unmapped enum value to fail")
	}
}

func newTestProvider(t *testing.T, reader Reader) *Provider {
	t.Helper()
	bundle := loadTestProfile(t)
	bindingPath := filepath.Join("..", "..", "..", "controllers", "rc-simulator", "reference-controller", "rapid", "channels.json")
	binding, err := LoadBinding(bindingPath, bundle)
	if err != nil {
		t.Fatalf("load Rapid binding: %v", err)
	}
	provider, err := NewProvider(reader, []GeneratorConfig{{
		Generator: monitor.Generator{
			ID:     "gen-rapid-test",
			Name:   "Rapid Test Generator",
			SiteID: "site-test",
			Controller: monitor.ControllerRef{
				Manufacturer: bundle.Manifest.Manufacturer,
				Model:        bundle.Manifest.Model,
			},
		},
		Profile: bundle,
		Binding: binding,
	}}, Options{Now: func() time.Time { return testNow }})
	if err != nil {
		t.Fatalf("new Rapid provider: %v", err)
	}
	return provider
}

func loadTestProfile(t *testing.T) profile.Bundle {
	t.Helper()
	root := filepath.Join("..", "..", "..", "controllers", "rc-simulator", "reference-controller")
	bundle, err := profile.LoadDir(root)
	if err != nil {
		t.Fatalf("load test profile: %v", err)
	}
	return bundle
}
