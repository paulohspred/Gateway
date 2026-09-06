package monitor

import (
	"context"
	"errors"
	"fmt"
)

// Service is the provider-independent application core. HTTP, CLI and future
// consumers should depend on Service rather than importing a concrete provider.
type Service struct {
	provider Provider
}

func NewService(provider Provider) (*Service, error) {
	if provider == nil {
		return nil, errors.New("monitor provider is required")
	}
	return &Service{provider: provider}, nil
}

func (s *Service) ListGenerators(ctx context.Context) ([]Generator, error) {
	generators, err := s.provider.ListGenerators(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(generators))
	for i := range generators {
		if err := generators[i].Validate(); err != nil {
			return nil, fmt.Errorf("provider generator[%d]: %w", i, err)
		}
		if _, ok := seen[generators[i].ID]; ok {
			return nil, fmt.Errorf("provider returned duplicate generator id %q", generators[i].ID)
		}
		seen[generators[i].ID] = struct{}{}
	}
	return generators, nil
}

func (s *Service) GetGenerator(ctx context.Context, id string) (Generator, error) {
	generator, err := s.provider.GetGenerator(ctx, id)
	if err != nil {
		return Generator{}, err
	}
	if err := generator.Validate(); err != nil {
		return Generator{}, fmt.Errorf("provider generator: %w", err)
	}
	if generator.ID != id {
		return Generator{}, fmt.Errorf("provider returned generator %q for request %q", generator.ID, id)
	}
	return generator, nil
}

func (s *Service) GetTelemetry(ctx context.Context, id string) (TelemetrySnapshot, error) {
	snapshot, err := s.provider.GetTelemetry(ctx, id)
	if err != nil {
		return TelemetrySnapshot{}, err
	}
	if err := snapshot.Validate(); err != nil {
		return TelemetrySnapshot{}, fmt.Errorf("provider telemetry: %w", err)
	}
	if snapshot.GeneratorID != id {
		return TelemetrySnapshot{}, fmt.Errorf("provider returned telemetry for generator %q instead of %q", snapshot.GeneratorID, id)
	}
	return snapshot, nil
}

func (s *Service) GetAlarms(ctx context.Context, id string) ([]Alarm, error) {
	alarms, err := s.provider.GetAlarms(ctx, id)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(alarms))
	for i := range alarms {
		if err := alarms[i].Validate(); err != nil {
			return nil, fmt.Errorf("provider alarm[%d]: %w", i, err)
		}
		if alarms[i].GeneratorID != id {
			return nil, fmt.Errorf("provider returned alarm for generator %q instead of %q", alarms[i].GeneratorID, id)
		}
		if _, ok := seen[alarms[i].ID]; ok {
			return nil, fmt.Errorf("provider returned duplicate alarm id %q", alarms[i].ID)
		}
		seen[alarms[i].ID] = struct{}{}
	}
	return alarms, nil
}

func (s *Service) GetEvents(ctx context.Context, id string) ([]Event, error) {
	events, err := s.provider.GetEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(events))
	for i := range events {
		if err := events[i].Validate(); err != nil {
			return nil, fmt.Errorf("provider event[%d]: %w", i, err)
		}
		if events[i].GeneratorID != id {
			return nil, fmt.Errorf("provider returned event for generator %q instead of %q", events[i].GeneratorID, id)
		}
		if _, ok := seen[events[i].ID]; ok {
			return nil, fmt.Errorf("provider returned duplicate event id %q", events[i].ID)
		}
		seen[events[i].ID] = struct{}{}
	}
	return events, nil
}

func (s *Service) Health(ctx context.Context) (ProviderHealth, error) {
	health, err := s.provider.Health(ctx)
	if err != nil {
		return ProviderHealth{}, err
	}
	if err := health.Validate(); err != nil {
		return ProviderHealth{}, fmt.Errorf("provider health: %w", err)
	}
	return health, nil
}
