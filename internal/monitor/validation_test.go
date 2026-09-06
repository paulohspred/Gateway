package monitor

import (
	"testing"
	"time"
)

func TestAlarmValidation(t *testing.T) {
	now := time.Date(2026, time.September, 6, 14, 0, 0, 0, time.UTC)
	alarm := Alarm{
		ID:          "a1",
		GeneratorID: "g1",
		Code:        "LOW_OIL_PRESSURE",
		Severity:    AlarmCritical,
		Message:     "Low oil pressure",
		Active:      true,
		RaisedAt:    now,
	}
	if err := alarm.Validate(); err != nil {
		t.Fatal(err)
	}

	cleared := now.Add(time.Minute)
	alarm.ClearedAt = &cleared
	if err := alarm.Validate(); err == nil {
		t.Fatal("expected active alarm with clearedAt to fail")
	}
}

func TestEventValidation(t *testing.T) {
	event := Event{
		ID:          "e1",
		GeneratorID: "g1",
		Type:        "alarm.raised",
		Message:     "Alarm raised",
		OccurredAt:  time.Date(2026, time.September, 6, 14, 0, 0, 0, time.UTC),
	}
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
	event.Type = "bad type"
	if err := event.Validate(); err == nil {
		t.Fatal("expected invalid event type to fail")
	}
}

func TestProviderHealthValidation(t *testing.T) {
	health := ProviderHealth{
		Status:    ProviderHealthy,
		CheckedAt: time.Date(2026, time.September, 6, 14, 0, 0, 0, time.UTC),
	}
	if err := health.Validate(); err != nil {
		t.Fatal(err)
	}
	health.Status = "broken"
	if err := health.Validate(); err == nil {
		t.Fatal("expected invalid provider status to fail")
	}
}
