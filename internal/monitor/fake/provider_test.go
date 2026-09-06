package fake

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
)

var fixedNow = time.Date(2026, time.September, 6, 9, 30, 0, 0, time.UTC)

func newTestProvider() *Provider {
	return NewProvider(Options{Now: func() time.Time { return fixedNow }})
}

func TestOnlineTelemetryIsCanonicalAndDoesNotInventMissingMetric(t *testing.T) {
	provider := newTestProvider()
	snapshot, err := provider.GetTelemetry(context.Background(), "gen-sim-001")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Communication != monitor.CommunicationOnline {
		t.Fatalf("expected online communication, got %s", snapshot.Communication)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("snapshot should validate: %v", err)
	}

	rpm := snapshot.Metrics[monitor.MetricEngineRPM]
	if rpm.Value.Number == nil || *rpm.Value.Number != 1500 {
		t.Fatalf("unexpected RPM metric: %#v", rpm)
	}
	if rpm.Quality != monitor.QualityGood {
		t.Fatalf("expected good RPM quality, got %s", rpm.Quality)
	}
	if _, exists := snapshot.Metrics[monitor.MetricFuelLevel]; exists {
		t.Fatal("fuel.level is intentionally unsupported by the fixture and must be absent, not zero")
	}
}

func TestStaleScenarioKeepsLastKnownValueWithStaleQuality(t *testing.T) {
	provider := newTestProvider()
	if err := provider.SetScenario(ScenarioStale); err != nil {
		t.Fatal(err)
	}
	snapshot, err := provider.GetTelemetry(context.Background(), "gen-sim-001")
	if err != nil {
		t.Fatal(err)
	}
	rpm := snapshot.Metrics[monitor.MetricEngineRPM]
	if rpm.Quality != monitor.QualityStale {
		t.Fatalf("expected stale quality, got %s", rpm.Quality)
	}
	if rpm.Value.Number == nil || *rpm.Value.Number != 1500 {
		t.Fatalf("stale metric must preserve last-known value, got %#v", rpm.Value)
	}
	if !rpm.ObservedAt.Before(snapshot.CapturedAt) {
		t.Fatalf("stale metric must have an older observation time: observed=%s captured=%s", rpm.ObservedAt, snapshot.CapturedAt)
	}
}

func TestOfflineScenarioMarksCommunicationAndMetricQualityOffline(t *testing.T) {
	provider := newTestProvider()
	if err := provider.SetScenario(ScenarioOffline); err != nil {
		t.Fatal(err)
	}
	snapshot, err := provider.GetTelemetry(context.Background(), "gen-sim-001")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Communication != monitor.CommunicationOffline {
		t.Fatalf("expected offline communication, got %s", snapshot.Communication)
	}
	if got := snapshot.Metrics[monitor.MetricGeneratorFrequency].Quality; got != monitor.QualityOffline {
		t.Fatalf("expected offline metric quality, got %s", got)
	}
}

func TestAlarmScenarioExposesActiveCriticalAlarm(t *testing.T) {
	provider := newTestProvider()
	if err := provider.SetScenario(ScenarioAlarm); err != nil {
		t.Fatal(err)
	}
	alarms, err := provider.GetAlarms(context.Background(), "gen-sim-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(alarms) != 1 {
		t.Fatalf("expected one alarm, got %d", len(alarms))
	}
	if !alarms[0].Active || alarms[0].Severity != monitor.AlarmCritical {
		t.Fatalf("unexpected alarm: %#v", alarms[0])
	}
}

func TestRecoveryFromOfflineBackToOnline(t *testing.T) {
	provider := newTestProvider()
	if err := provider.SetScenario(ScenarioOffline); err != nil {
		t.Fatal(err)
	}
	offline, err := provider.GetTelemetry(context.Background(), "gen-sim-001")
	if err != nil {
		t.Fatal(err)
	}
	if offline.Communication != monitor.CommunicationOffline {
		t.Fatalf("expected offline state, got %s", offline.Communication)
	}

	if err := provider.SetScenario(ScenarioOnline); err != nil {
		t.Fatal(err)
	}
	online, err := provider.GetTelemetry(context.Background(), "gen-sim-001")
	if err != nil {
		t.Fatal(err)
	}
	if online.Communication != monitor.CommunicationOnline {
		t.Fatalf("expected recovered online state, got %s", online.Communication)
	}
	if online.Metrics[monitor.MetricEngineRPM].Quality != monitor.QualityGood {
		t.Fatalf("expected recovered metric quality good, got %s", online.Metrics[monitor.MetricEngineRPM].Quality)
	}
}

func TestProviderHealthCanBeDegradedWithoutInventingTransportFailure(t *testing.T) {
	provider := newTestProvider()
	if err := provider.SetProviderStatus(monitor.ProviderDegraded); err != nil {
		t.Fatal(err)
	}
	health, err := provider.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != monitor.ProviderDegraded {
		t.Fatalf("expected degraded provider health, got %s", health.Status)
	}
	if err := provider.SetProviderStatus(monitor.ProviderStatus("invalid")); err == nil {
		t.Fatal("expected invalid provider status to be rejected")
	}
}

func TestUnknownGeneratorUsesStableSentinel(t *testing.T) {
	provider := newTestProvider()
	_, err := provider.GetGenerator(context.Background(), "missing")
	if !errors.Is(err, monitor.ErrGeneratorNotFound) {
		t.Fatalf("expected ErrGeneratorNotFound, got %v", err)
	}
}

func TestCancelledContextIsHonored(t *testing.T) {
	provider := newTestProvider()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.ListGenerators(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestInvalidScenarioIsRejected(t *testing.T) {
	provider := newTestProvider()
	if err := provider.SetScenario(Scenario("invalid")); err == nil {
		t.Fatal("expected invalid scenario to be rejected")
	}
}
