package rapid

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/profile"
)

// ChannelData is the narrow RC-side representation of Rapid SCADA current
// channel data. Status follows Rapid SCADA semantics: values with status > 0
// are defined, while status <= 0 is undefined. The reader must provide the
// observation time explicitly; the provider never fabricates a value for an
// undefined channel.
type ChannelData struct {
	ChannelNumber int
	Value         float64
	Status        int
	ObservedAt    time.Time
}

func (d ChannelData) Defined() bool {
	return d.Status > 0
}

// Reader is the seam between the Go RC Monitor domain and the concrete Rapid
// SCADA client adapter. MON-004 deliberately does not reimplement the Rapid
// Server wire protocol. MON-005 will connect this seam to a local adapter that
// uses Rapid SCADA's supported client libraries.
type Reader interface {
	ReadCurrent(context.Context, []int) ([]ChannelData, error)
	ReadAlarms(context.Context, string) ([]monitor.Alarm, error)
	ReadEvents(context.Context, string) ([]monitor.Event, error)
	Health(context.Context) error
}

type GeneratorConfig struct {
	Generator monitor.Generator
	Profile   profile.Bundle
	Binding   BindingFile
}

type Provider struct {
	reader     Reader
	now        func() time.Time
	generators map[string]GeneratorConfig
	orderedIDs []string

	mu   sync.RWMutex
	last map[string]monitor.TelemetrySnapshot
}

type Options struct {
	Now func() time.Time
}

func NewProvider(reader Reader, configs []GeneratorConfig, options Options) (*Provider, error) {
	if reader == nil {
		return nil, errors.New("rapid reader is required")
	}
	if len(configs) == 0 {
		return nil, errors.New("at least one generator configuration is required")
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	p := &Provider{
		reader:     reader,
		now:        now,
		generators: make(map[string]GeneratorConfig, len(configs)),
		orderedIDs: make([]string, 0, len(configs)),
		last:       make(map[string]monitor.TelemetrySnapshot),
	}
	for _, cfg := range configs {
		if err := cfg.Generator.Validate(); err != nil {
			return nil, fmt.Errorf("generator %q: %w", cfg.Generator.ID, err)
		}
		if err := cfg.Profile.Validate(); err != nil {
			return nil, fmt.Errorf("generator %q profile: %w", cfg.Generator.ID, err)
		}
		if err := cfg.Binding.Validate(cfg.Profile); err != nil {
			return nil, fmt.Errorf("generator %q Rapid binding: %w", cfg.Generator.ID, err)
		}
		if cfg.Generator.Controller.Manufacturer != cfg.Profile.Manifest.Manufacturer ||
			cfg.Generator.Controller.Model != cfg.Profile.Manifest.Model {
			return nil, fmt.Errorf("generator %q controller does not match profile %q", cfg.Generator.ID, cfg.Profile.Manifest.ID)
		}
		if _, exists := p.generators[cfg.Generator.ID]; exists {
			return nil, fmt.Errorf("duplicate generator id %q", cfg.Generator.ID)
		}
		p.generators[cfg.Generator.ID] = cfg
		p.orderedIDs = append(p.orderedIDs, cfg.Generator.ID)
	}
	sort.Strings(p.orderedIDs)
	return p, nil
}

func (p *Provider) ListGenerators(ctx context.Context) ([]monitor.Generator, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	out := make([]monitor.Generator, 0, len(p.orderedIDs))
	for _, id := range p.orderedIDs {
		out = append(out, p.generators[id].Generator)
	}
	return out, nil
}

func (p *Provider) GetGenerator(ctx context.Context, id string) (monitor.Generator, error) {
	if err := contextErr(ctx); err != nil {
		return monitor.Generator{}, err
	}
	cfg, ok := p.generators[id]
	if !ok {
		return monitor.Generator{}, monitor.GeneratorNotFound(id)
	}
	return cfg.Generator, nil
}

func (p *Provider) GetTelemetry(ctx context.Context, id string) (monitor.TelemetrySnapshot, error) {
	if err := contextErr(ctx); err != nil {
		return monitor.TelemetrySnapshot{}, err
	}
	cfg, ok := p.generators[id]
	if !ok {
		return monitor.TelemetrySnapshot{}, monitor.GeneratorNotFound(id)
	}

	capturedAt := p.now().UTC()
	channels := cfg.Binding.ChannelNumbers()
	data, err := p.reader.ReadCurrent(ctx, channels)
	if err != nil {
		return p.offlineSnapshot(id, capturedAt), nil
	}

	byChannel := make(map[int]ChannelData, len(data))
	for _, sample := range data {
		if sample.ChannelNumber <= 0 {
			return monitor.TelemetrySnapshot{}, errors.New("rapid reader returned invalid channel number")
		}
		if sample.Defined() && sample.ObservedAt.IsZero() {
			return monitor.TelemetrySnapshot{}, fmt.Errorf("rapid channel %d is defined but has no observedAt", sample.ChannelNumber)
		}
		if sample.Defined() && (math.IsNaN(sample.Value) || math.IsInf(sample.Value, 0)) {
			return monitor.TelemetrySnapshot{}, fmt.Errorf("rapid channel %d returned non-finite value", sample.ChannelNumber)
		}
		byChannel[sample.ChannelNumber] = sample
	}

	metrics := make(map[monitor.MetricKey]monitor.Metric)
	definitions := metricDefinitions(cfg.Profile)
	for _, binding := range cfg.Binding.Metrics {
		sample, exists := byChannel[binding.ChannelNumber]
		if !exists || !sample.Defined() {
			continue
		}
		definition := definitions[binding.Key]
		value, err := binding.Transform.Apply(sample.Value, definition.Kind)
		if err != nil {
			return monitor.TelemetrySnapshot{}, fmt.Errorf("metric %q from channel %d: %w", binding.Key, binding.ChannelNumber, err)
		}
		quality := monitor.QualityGood
		if capturedAt.Sub(sample.ObservedAt.UTC()) > time.Duration(definition.StaleAfterSeconds)*time.Second {
			quality = monitor.QualityStale
		}
		metric := monitor.Metric{
			Key:        binding.Key,
			Value:      value,
			Unit:       definition.Unit,
			Quality:    quality,
			ObservedAt: sample.ObservedAt.UTC(),
		}
		if err := metric.Validate(); err != nil {
			return monitor.TelemetrySnapshot{}, err
		}
		metrics[binding.Key] = metric
	}

	snapshot := monitor.TelemetrySnapshot{
		GeneratorID:   id,
		CapturedAt:    capturedAt,
		Communication: monitor.CommunicationOnline,
		Metrics:       metrics,
	}
	if err := snapshot.Validate(); err != nil {
		return monitor.TelemetrySnapshot{}, err
	}
	p.mu.Lock()
	p.last[id] = cloneSnapshot(snapshot)
	p.mu.Unlock()
	return snapshot, nil
}

func (p *Provider) GetAlarms(ctx context.Context, id string) ([]monitor.Alarm, error) {
	if _, err := p.GetGenerator(ctx, id); err != nil {
		return nil, err
	}
	alarms, err := p.reader.ReadAlarms(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range alarms {
		if alarms[i].GeneratorID != id {
			return nil, fmt.Errorf("rapid reader returned alarm for unexpected generator %q", alarms[i].GeneratorID)
		}
	}
	return alarms, nil
}

func (p *Provider) GetEvents(ctx context.Context, id string) ([]monitor.Event, error) {
	if _, err := p.GetGenerator(ctx, id); err != nil {
		return nil, err
	}
	events, err := p.reader.ReadEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range events {
		if events[i].GeneratorID != id {
			return nil, fmt.Errorf("rapid reader returned event for unexpected generator %q", events[i].GeneratorID)
		}
	}
	return events, nil
}

func (p *Provider) Health(ctx context.Context) (monitor.ProviderHealth, error) {
	if err := contextErr(ctx); err != nil {
		return monitor.ProviderHealth{}, err
	}
	now := p.now().UTC()
	if err := p.reader.Health(ctx); err != nil {
		return monitor.ProviderHealth{
			Status:    monitor.ProviderUnavailable,
			CheckedAt: now,
			Message:   "Rapid SCADA reader unavailable",
		}, nil
	}
	return monitor.ProviderHealth{
		Status:    monitor.ProviderHealthy,
		CheckedAt: now,
		Message:   "Rapid SCADA reader available",
	}, nil
}

func (p *Provider) offlineSnapshot(id string, capturedAt time.Time) monitor.TelemetrySnapshot {
	p.mu.RLock()
	last, ok := p.last[id]
	p.mu.RUnlock()
	metrics := make(map[monitor.MetricKey]monitor.Metric)
	if ok {
		for key, metric := range last.Metrics {
			metric.Quality = monitor.QualityOffline
			metrics[key] = metric
		}
	}
	return monitor.TelemetrySnapshot{
		GeneratorID:   id,
		CapturedAt:    capturedAt,
		Communication: monitor.CommunicationOffline,
		Metrics:       metrics,
	}
}

func metricDefinitions(bundle profile.Bundle) map[monitor.MetricKey]profile.MetricDefinition {
	definitions := make(map[monitor.MetricKey]profile.MetricDefinition, len(bundle.Telemetry.Metrics))
	for _, definition := range bundle.Telemetry.Metrics {
		definitions[definition.Key] = definition
	}
	return definitions
}

func cloneSnapshot(in monitor.TelemetrySnapshot) monitor.TelemetrySnapshot {
	out := in
	out.Metrics = make(map[monitor.MetricKey]monitor.Metric, len(in.Metrics))
	for key, metric := range in.Metrics {
		out.Metrics[key] = metric
	}
	return out
}

func contextErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

type TransformKind string

const (
	TransformNumber  TransformKind = "number"
	TransformBoolean TransformKind = "boolean"
	TransformEnum    TransformKind = "enum"
)

type Transform struct {
	Kind        TransformKind     `json:"kind"`
	Scale       *float64          `json:"scale,omitempty"`
	Offset      *float64          `json:"offset,omitempty"`
	TrueValues  []float64         `json:"trueValues,omitempty"`
	FalseValues []float64         `json:"falseValues,omitempty"`
	EnumValues  map[string]string `json:"enumValues,omitempty"`
}

func (t Transform) Validate(expected monitor.ValueKind) error {
	switch t.Kind {
	case TransformNumber:
		if expected != monitor.ValueNumber {
			return fmt.Errorf("number transform cannot produce %q", expected)
		}
		if len(t.TrueValues) > 0 || len(t.FalseValues) > 0 || len(t.EnumValues) > 0 {
			return errors.New("number transform cannot contain boolean/enum mappings")
		}
	case TransformBoolean:
		if expected != monitor.ValueBoolean {
			return fmt.Errorf("boolean transform cannot produce %q", expected)
		}
		if len(t.TrueValues) == 0 || len(t.FalseValues) == 0 {
			return errors.New("boolean transform requires trueValues and falseValues")
		}
		if t.Scale != nil || t.Offset != nil || len(t.EnumValues) > 0 {
			return errors.New("boolean transform cannot contain number/enum options")
		}
		for _, trueValue := range t.TrueValues {
			if containsFloat(t.FalseValues, trueValue) {
				return fmt.Errorf("boolean value %v appears in both trueValues and falseValues", trueValue)
			}
		}
	case TransformEnum:
		if expected != monitor.ValueText {
			return fmt.Errorf("enum transform cannot produce %q", expected)
		}
		if len(t.EnumValues) == 0 {
			return errors.New("enum transform requires enumValues")
		}
		if t.Scale != nil || t.Offset != nil || len(t.TrueValues) > 0 || len(t.FalseValues) > 0 {
			return errors.New("enum transform cannot contain number/boolean options")
		}
		for raw, text := range t.EnumValues {
			if _, err := strconv.ParseFloat(raw, 64); err != nil {
				return fmt.Errorf("invalid enum numeric key %q", raw)
			}
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("enum value %q has empty text", raw)
			}
		}
	default:
		return fmt.Errorf("invalid transform kind %q", t.Kind)
	}
	return nil
}

func (t Transform) Apply(raw float64, expected monitor.ValueKind) (monitor.MetricValue, error) {
	if math.IsNaN(raw) || math.IsInf(raw, 0) {
		return monitor.MetricValue{}, errors.New("raw value must be finite")
	}
	if err := t.Validate(expected); err != nil {
		return monitor.MetricValue{}, err
	}
	switch t.Kind {
	case TransformNumber:
		scale := 1.0
		if t.Scale != nil {
			scale = *t.Scale
		}
		offset := 0.0
		if t.Offset != nil {
			offset = *t.Offset
		}
		value := raw*scale + offset
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return monitor.MetricValue{}, errors.New("transformed numeric value must be finite")
		}
		return monitor.NumberValue(value), nil
	case TransformBoolean:
		if containsFloat(t.TrueValues, raw) {
			return monitor.BooleanValue(true), nil
		}
		if containsFloat(t.FalseValues, raw) {
			return monitor.BooleanValue(false), nil
		}
		return monitor.MetricValue{}, fmt.Errorf("boolean raw value %v is not mapped", raw)
	case TransformEnum:
		key := strconv.FormatFloat(raw, 'g', -1, 64)
		text, ok := t.EnumValues[key]
		if !ok {
			return monitor.MetricValue{}, fmt.Errorf("enum raw value %v is not mapped", raw)
		}
		return monitor.TextValue(text), nil
	default:
		return monitor.MetricValue{}, fmt.Errorf("unsupported transform kind %q", t.Kind)
	}
}

func containsFloat(values []float64, wanted float64) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

var _ monitor.Provider = (*Provider)(nil)
