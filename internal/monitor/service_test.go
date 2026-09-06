package monitor_test

import (
	"context"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/fake"
)

func TestNewServiceRejectsNilProvider(t *testing.T) {
	if _, err := monitor.NewService(nil); err == nil {
		t.Fatal("expected nil provider to be rejected")
	}
}

func TestServiceDependsOnlyOnProviderContract(t *testing.T) {
	now := time.Date(2026, time.September, 6, 9, 30, 0, 0, time.UTC)
	provider := fake.NewProvider(fake.Options{Now: func() time.Time { return now }})
	service, err := monitor.NewService(provider)
	if err != nil {
		t.Fatal(err)
	}

	generators, err := service.ListGenerators(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(generators) != 1 || generators[0].ID != "gen-sim-001" {
		t.Fatalf("unexpected generators: %#v", generators)
	}

	health, err := service.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != monitor.ProviderHealthy {
		t.Fatalf("unexpected provider health: %#v", health)
	}
}
