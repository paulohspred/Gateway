package monitor

import (
	"math"
	"testing"
	"time"
)

func TestMetricValueRequiresExactlyOneKind(t *testing.T) {
	if _, err := (MetricValue{}).Kind(); err == nil {
		t.Fatal("expected empty metric value to be rejected")
	}

	number := 1.0
	text := "one"
	if _, err := (MetricValue{Number: &number, Text: &text}).Kind(); err == nil {
		t.Fatal("expected multi-kind metric value to be rejected")
	}
}

func TestMetricValueRejectsNonFiniteNumbers(t *testing.T) {
	value := NumberValue(math.NaN())
	if _, err := value.Kind(); err == nil {
		t.Fatal("expected NaN to be rejected")
	}
}

func TestNumericZeroIsARealValue(t *testing.T) {
	value := NumberValue(0)
	kind, err := value.Kind()
	if err != nil {
		t.Fatalf("zero should be a valid numeric value: %v", err)
	}
	if kind != ValueNumber {
		t.Fatalf("unexpected value kind: %s", kind)
	}
	if value.Number == nil || *value.Number != 0 {
		t.Fatalf("zero must remain explicitly present, got %#v", value)
	}
}

func TestTelemetrySnapshotValidationRejectsMismatchedMetricKey(t *testing.T) {
	now := time.Date(2026, time.September, 6, 9, 0, 0, 0, time.UTC)
	snapshot := TelemetrySnapshot{
		GeneratorID:   "gen-1",
		CapturedAt:    now,
		Communication: CommunicationOnline,
		Metrics: map[MetricKey]Metric{
			MetricEngineRPM: {
				Key:        MetricGeneratorFrequency,
				Value:      NumberValue(1500),
				Unit:       "rpm",
				Quality:    QualityGood,
				ObservedAt: now,
			},
		},
	}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("expected mismatched map/metric keys to be rejected")
	}
}
