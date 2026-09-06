package monitor

import (
	"context"
	"errors"
	"fmt"
)

var ErrGeneratorNotFound = errors.New("generator not found")

// Provider is the only data-source contract RC Monitor depends on. A provider
// may read from Rapid SCADA, a deterministic simulator or another explicitly
// supported source, but callers must not depend on provider-specific details.
type Provider interface {
	ListGenerators(context.Context) ([]Generator, error)
	GetGenerator(context.Context, string) (Generator, error)
	GetTelemetry(context.Context, string) (TelemetrySnapshot, error)
	GetAlarms(context.Context, string) ([]Alarm, error)
	GetEvents(context.Context, string) ([]Event, error)
	Health(context.Context) (ProviderHealth, error)
}

func GeneratorNotFound(id string) error {
	return fmt.Errorf("%w: %s", ErrGeneratorNotFound, id)
}
