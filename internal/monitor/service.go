package monitor

import (
	"context"
	"errors"
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
	return s.provider.ListGenerators(ctx)
}

func (s *Service) GetGenerator(ctx context.Context, id string) (Generator, error) {
	return s.provider.GetGenerator(ctx, id)
}

func (s *Service) GetTelemetry(ctx context.Context, id string) (TelemetrySnapshot, error) {
	return s.provider.GetTelemetry(ctx, id)
}

func (s *Service) GetAlarms(ctx context.Context, id string) ([]Alarm, error) {
	return s.provider.GetAlarms(ctx, id)
}

func (s *Service) GetEvents(ctx context.Context, id string) ([]Event, error) {
	return s.provider.GetEvents(ctx, id)
}

func (s *Service) Health(ctx context.Context) (ProviderHealth, error) {
	return s.provider.Health(ctx)
}
