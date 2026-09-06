package fake

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
)

type Scenario string

const (
	ScenarioOnline  Scenario = "online"
	ScenarioStale   Scenario = "stale"
	ScenarioOffline Scenario = "offline"
	ScenarioAlarm   Scenario = "alarm"
)

type Options struct {
	Now func() time.Time
}

// Provider is a deterministic, in-memory read-only provider used to prove the
// RC Monitor domain contract before any Rapid SCADA dependency is introduced.
// It is not a historian and does not emulate Modbus or another wire protocol.
type Provider struct {
	mu        sync.RWMutex
	now       func() time.Time
	scenario  Scenario
	generator monitor.Generator
}

func NewProvider(options Options) *Provider {
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Provider{
		now:      now,
		scenario: ScenarioOnline,
		generator: monitor.Generator{
			ID:     "gen-sim-001",
			Name:   "Gerador Simulado 01",
			SiteID: "site-sim-001",
			Controller: monitor.ControllerRef{
				Manufacturer: "RC Simulator",
				Model:        "Reference Controller",
			},
		},
	}
}

func (p *Provider) SetScenario(scenario Scenario) error {
	if !validScenario(scenario) {
		return fmt.Errorf("unknown fake provider scenario %q", scenario)
	}
	p.mu.Lock()
	p.scenario = scenario
	p.mu.Unlock()
	return nil
}

func (p *Provider) ListGenerators(ctx context.Context) ([]monitor.Generator, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	p.mu.RLock()
	generator := p.generator
	p.mu.RUnlock()
	return []monitor.Generator{generator}, nil
}

func (p *Provider) GetGenerator(ctx context.Context, id string) (monitor.Generator, error) {
	if err := contextErr(ctx); err != nil {
		return monitor.Generator{}, err
	}
	p.mu.RLock()
	generator := p.generator
	p.mu.RUnlock()
	if id != generator.ID {
		return monitor.Generator{}, monitor.GeneratorNotFound(id)
	}
	return generator, nil
}

func (p *Provider) GetTelemetry(ctx context.Context, id string) (monitor.TelemetrySnapshot, error) {
	if err := contextErr(ctx); err != nil {
		return monitor.TelemetrySnapshot{}, err
	}
	p.mu.RLock()
	generator := p.generator
	scenario := p.scenario
	now := p.now().UTC()
	p.mu.RUnlock()
	if id != generator.ID {
		return monitor.TelemetrySnapshot{}, monitor.GeneratorNotFound(id)
	}

	communication := monitor.CommunicationOnline
	quality := monitor.QualityGood
	observedAt := now
	switch scenario {
	case ScenarioStale:
		quality = monitor.QualityStale
		observedAt = now.Add(-2 * time.Minute)
	case ScenarioOffline:
		communication = monitor.CommunicationOffline
		quality = monitor.QualityOffline
		observedAt = now.Add(-5 * time.Minute)
	case ScenarioOnline, ScenarioAlarm:
	default:
		return monitor.TelemetrySnapshot{}, fmt.Errorf("invalid internal fake scenario %q", scenario)
	}

	metrics := map[monitor.MetricKey]monitor.Metric{
		monitor.MetricEngineRPM:            numberMetric(monitor.MetricEngineRPM, 1500, "rpm", quality, observedAt),
		monitor.MetricEngineOilPressure:    numberMetric(monitor.MetricEngineOilPressure, 4.2, "bar", quality, observedAt),
		monitor.MetricEngineCoolantTemp:    numberMetric(monitor.MetricEngineCoolantTemp, 82, "C", quality, observedAt),
		monitor.MetricGeneratorVoltageL1:   numberMetric(monitor.MetricGeneratorVoltageL1, 230.1, "V", quality, observedAt),
		monitor.MetricGeneratorVoltageL2:   numberMetric(monitor.MetricGeneratorVoltageL2, 229.8, "V", quality, observedAt),
		monitor.MetricGeneratorVoltageL3:   numberMetric(monitor.MetricGeneratorVoltageL3, 231.0, "V", quality, observedAt),
		monitor.MetricGeneratorFrequency:   numberMetric(monitor.MetricGeneratorFrequency, 60.02, "Hz", quality, observedAt),
		monitor.MetricGeneratorPowerKW:     numberMetric(monitor.MetricGeneratorPowerKW, 86.4, "kW", quality, observedAt),
		monitor.MetricGeneratorPowerFactor: numberMetric(monitor.MetricGeneratorPowerFactor, 0.95, "", quality, observedAt),
		monitor.MetricBatteryVoltage:       numberMetric(monitor.MetricBatteryVoltage, 26.4, "V", quality, observedAt),
		monitor.MetricControllerMode:       textMetric(monitor.MetricControllerMode, "auto", quality, observedAt),
		monitor.MetricBreakerGCB:           boolMetric(monitor.MetricBreakerGCB, true, quality, observedAt),
	}
	// MetricFuelLevel is intentionally absent. This fixture proves that a metric
	// not supplied by a controller/provider is omitted rather than invented as 0.

	snapshot := monitor.TelemetrySnapshot{
		GeneratorID:   generator.ID,
		CapturedAt:    now,
		Communication: communication,
		Metrics:       metrics,
	}
	if err := snapshot.Validate(); err != nil {
		return monitor.TelemetrySnapshot{}, fmt.Errorf("fake telemetry fixture invalid: %w", err)
	}
	return snapshot, nil
}

func (p *Provider) GetAlarms(ctx context.Context, id string) ([]monitor.Alarm, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	p.mu.RLock()
	generator := p.generator
	scenario := p.scenario
	now := p.now().UTC()
	p.mu.RUnlock()
	if id != generator.ID {
		return nil, monitor.GeneratorNotFound(id)
	}
	if scenario != ScenarioAlarm {
		return []monitor.Alarm{}, nil
	}
	return []monitor.Alarm{
		{
			ID:          "alarm-sim-001",
			GeneratorID: generator.ID,
			Code:        "LOW_OIL_PRESSURE",
			Severity:    monitor.AlarmCritical,
			Message:     "Pressao de oleo abaixo do limite de teste",
			Active:      true,
			RaisedAt:    now.Add(-30 * time.Second),
		},
	}, nil
}

func (p *Provider) GetEvents(ctx context.Context, id string) ([]monitor.Event, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	p.mu.RLock()
	generator := p.generator
	scenario := p.scenario
	now := p.now().UTC()
	p.mu.RUnlock()
	if id != generator.ID {
		return nil, monitor.GeneratorNotFound(id)
	}

	message := "telemetria sintetica disponivel"
	eventType := "communication.online"
	if scenario == ScenarioOffline {
		message = "comunicacao sintetica indisponivel"
		eventType = "communication.offline"
	}
	return []monitor.Event{
		{
			ID:          "event-sim-current",
			GeneratorID: generator.ID,
			Type:        eventType,
			Message:     message,
			OccurredAt:  now,
		},
	}, nil
}

func (p *Provider) Health(ctx context.Context) (monitor.ProviderHealth, error) {
	if err := contextErr(ctx); err != nil {
		return monitor.ProviderHealth{}, err
	}
	p.mu.RLock()
	now := p.now().UTC()
	p.mu.RUnlock()
	return monitor.ProviderHealth{
		Status:    monitor.ProviderHealthy,
		CheckedAt: now,
		Message:   "fake provider ready",
	}, nil
}

func numberMetric(key monitor.MetricKey, value float64, unit string, quality monitor.Quality, observedAt time.Time) monitor.Metric {
	return monitor.Metric{Key: key, Value: monitor.NumberValue(value), Unit: unit, Quality: quality, ObservedAt: observedAt}
}

func textMetric(key monitor.MetricKey, value string, quality monitor.Quality, observedAt time.Time) monitor.Metric {
	return monitor.Metric{Key: key, Value: monitor.TextValue(value), Quality: quality, ObservedAt: observedAt}
}

func boolMetric(key monitor.MetricKey, value bool, quality monitor.Quality, observedAt time.Time) monitor.Metric {
	return monitor.Metric{Key: key, Value: monitor.BooleanValue(value), Quality: quality, ObservedAt: observedAt}
}

func contextErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func validScenario(scenario Scenario) bool {
	switch scenario {
	case ScenarioOnline, ScenarioStale, ScenarioOffline, ScenarioAlarm:
		return true
	default:
		return false
	}
}

var _ monitor.Provider = (*Provider)(nil)
