package monitor_test

import (
	"context"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/fake"
)

type invalidAlarmProvider struct {
	monitor.Provider
}

func (p invalidAlarmProvider) GetAlarms(context.Context, string) ([]monitor.Alarm, error) {
	return []monitor.Alarm{{ID: "invalid"}}, nil
}

type invalidHealthProvider struct {
	monitor.Provider
}

func (p invalidHealthProvider) Health(context.Context) (monitor.ProviderHealth, error) {
	return monitor.ProviderHealth{Status: "broken", CheckedAt: time.Now().UTC()}, nil
}

func TestServiceRejectsInvalidProviderAlarm(t *testing.T) {
	base := fake.NewProvider(fake.Options{})
	service, err := monitor.NewService(invalidAlarmProvider{Provider: base})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetAlarms(context.Background(), "gen-sim-001"); err == nil {
		t.Fatal("expected invalid provider alarm to fail")
	}
}

func TestServiceRejectsInvalidProviderHealth(t *testing.T) {
	base := fake.NewProvider(fake.Options{})
	service, err := monitor.NewService(invalidHealthProvider{Provider: base})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Health(context.Background()); err == nil {
		t.Fatal("expected invalid provider health to fail")
	}
}
