package rapid

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/profile"
)

type EventQuery struct {
	ArchiveBit int
	PeriodDays int
	Limit      int
}

func (q EventQuery) Validate() error {
	if q.ArchiveBit <= 0 {
		return errors.New("rapid event archiveBit must be positive")
	}
	if q.PeriodDays < 1 || q.PeriodDays > 3650 {
		return errors.New("rapid event periodDays must be between 1 and 3650")
	}
	if q.Limit < 1 || q.Limit > 5000 {
		return errors.New("rapid event limit must be between 1 and 5000")
	}
	return nil
}

type RawEvent struct {
	ID             string
	ChannelNumber  int
	PreviousValue  float64
	PreviousStatus int
	Value          float64
	Status         int
	OccurredAt     time.Time
}

type RawReader interface {
	ReadCurrent(context.Context, []int) ([]ChannelData, error)
	ReadRecentEvents(context.Context, EventQuery) ([]RawEvent, error)
	Health(context.Context) error
}

type SemanticOptions struct {
	EventQuery EventQuery
}

type semanticGenerator struct {
	binding BindingFile
	profile profile.Bundle
}

type SemanticReader struct {
	raw        RawReader
	generators map[string]semanticGenerator
	eventQuery EventQuery
}

func NewSemanticReader(raw RawReader, configs []GeneratorConfig, options SemanticOptions) (*SemanticReader, error) {
	if raw == nil {
		return nil, errors.New("rapid raw reader is required")
	}
	if len(configs) == 0 {
		return nil, errors.New("at least one rapid semantic generator configuration is required")
	}
	query := options.EventQuery
	if query == (EventQuery{}) {
		query = EventQuery{ArchiveBit: 1, PeriodDays: 7, Limit: 1000}
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	reader := &SemanticReader{
		raw:        raw,
		generators: make(map[string]semanticGenerator, len(configs)),
		eventQuery: query,
	}
	for _, cfg := range configs {
		if err := cfg.Generator.Validate(); err != nil {
			return nil, fmt.Errorf("generator %q: %w", cfg.Generator.ID, err)
		}
		if err := cfg.Binding.Validate(cfg.Profile); err != nil {
			return nil, fmt.Errorf("generator %q Rapid semantic binding: %w", cfg.Generator.ID, err)
		}
		if _, exists := reader.generators[cfg.Generator.ID]; exists {
			return nil, fmt.Errorf("duplicate rapid semantic generator id %q", cfg.Generator.ID)
		}
		reader.generators[cfg.Generator.ID] = semanticGenerator{binding: cfg.Binding, profile: cfg.Profile}
	}
	return reader, nil
}

func (r *SemanticReader) ReadCurrent(ctx context.Context, channels []int) ([]ChannelData, error) {
	return r.raw.ReadCurrent(ctx, channels)
}

func (r *SemanticReader) ReadAlarms(ctx context.Context, generatorID string) ([]monitor.Alarm, error) {
	cfg, ok := r.generators[generatorID]
	if !ok {
		return nil, monitor.GeneratorNotFound(generatorID)
	}
	if len(cfg.binding.Alarms) == 0 {
		return []monitor.Alarm{}, nil
	}

	data, err := r.raw.ReadCurrent(ctx, cfg.binding.AlarmChannelNumbers())
	if err != nil {
		return nil, err
	}
	byChannel, err := semanticChannelMap(data)
	if err != nil {
		return nil, err
	}
	definitions := alarmDefinitions(cfg.profile)

	type activeAlarm struct {
		binding    AlarmBinding
		definition profile.AlarmDefinition
		detectedAt time.Time
	}
	active := make([]activeAlarm, 0, len(cfg.binding.Alarms))
	for _, binding := range cfg.binding.Alarms {
		sample, exists := byChannel[binding.ChannelNumber]
		if !exists || !sample.Defined() {
			continue
		}
		matched, err := binding.Active.Matches(sample.Value)
		if err != nil {
			return nil, fmt.Errorf("alarm %q channel %d: %w", binding.Code, binding.ChannelNumber, err)
		}
		if !matched {
			continue
		}
		definition := definitions[binding.Code]
		active = append(active, activeAlarm{
			binding:    binding,
			definition: definition,
			detectedAt: sample.ObservedAt.UTC(),
		})
	}
	if len(active) == 0 {
		return []monitor.Alarm{}, nil
	}

	history, historyErr := r.raw.ReadRecentEvents(ctx, r.eventQuery)
	alarms := make([]monitor.Alarm, 0, len(active))
	for _, item := range active {
		raisedAt := item.detectedAt
		if historyErr == nil {
			if historical := latestRaisedAt(history, item.binding); historical != nil {
				raisedAt = *historical
			}
		}
		alarms = append(alarms, monitor.Alarm{
			ID:          generatorID + ":" + item.binding.Code,
			GeneratorID: generatorID,
			Code:        item.binding.Code,
			Severity:    item.definition.Severity,
			Message:     item.definition.Message,
			Active:      true,
			RaisedAt:    raisedAt,
		})
	}
	sort.Slice(alarms, func(i, j int) bool {
		return alarms[i].Code < alarms[j].Code
	})
	return alarms, nil
}

func (r *SemanticReader) ReadEvents(ctx context.Context, generatorID string) ([]monitor.Event, error) {
	cfg, ok := r.generators[generatorID]
	if !ok {
		return nil, monitor.GeneratorNotFound(generatorID)
	}
	if len(cfg.binding.Alarms) == 0 && len(cfg.binding.Events) == 0 {
		return []monitor.Event{}, nil
	}

	rawEvents, err := r.raw.ReadRecentEvents(ctx, r.eventQuery)
	if err != nil {
		return nil, err
	}
	alarmDefs := alarmDefinitions(cfg.profile)
	alarmsByChannel := make(map[int][]AlarmBinding)
	for _, binding := range cfg.binding.Alarms {
		alarmsByChannel[binding.ChannelNumber] = append(alarmsByChannel[binding.ChannelNumber], binding)
	}
	eventsByChannel := make(map[int][]EventBinding)
	for _, binding := range cfg.binding.Events {
		eventsByChannel[binding.ChannelNumber] = append(eventsByChannel[binding.ChannelNumber], binding)
	}

	events := make([]monitor.Event, 0, len(rawEvents))
	for _, rawEvent := range rawEvents {
		if err := rawEvent.Validate(); err != nil {
			return nil, err
		}
		for _, binding := range alarmsByChannel[rawEvent.ChannelNumber] {
			if rawEvent.PreviousStatus <= 0 || rawEvent.Status <= 0 {
				continue
			}
			wasActive, err := binding.Active.Matches(rawEvent.PreviousValue)
			if err != nil {
				return nil, fmt.Errorf("event %s alarm %q previous value: %w", rawEvent.ID, binding.Code, err)
			}
			isActive, err := binding.Active.Matches(rawEvent.Value)
			if err != nil {
				return nil, fmt.Errorf("event %s alarm %q current value: %w", rawEvent.ID, binding.Code, err)
			}
			if wasActive == isActive {
				continue
			}
			definition := alarmDefs[binding.Code]
			eventType := "alarm.cleared"
			message := "Alarm cleared: " + definition.Message
			suffix := "cleared"
			if isActive {
				eventType = "alarm.raised"
				message = "Alarm raised: " + definition.Message
				suffix = "raised"
			}
			event := monitor.Event{
				ID:          rawEvent.ID + ":" + binding.Code + ":" + suffix,
				GeneratorID: generatorID,
				Type:        eventType,
				Message:     message,
				OccurredAt:  rawEvent.OccurredAt.UTC(),
			}
			events = append(events, event)
		}

		for _, binding := range eventsByChannel[rawEvent.ChannelNumber] {
			if rawEvent.Status <= 0 {
				continue
			}
			matched := true
			if binding.Condition != nil {
				var err error
				matched, err = binding.Condition.Matches(rawEvent.Value)
				if err != nil {
					return nil, fmt.Errorf("event %s binding %q: %w", rawEvent.ID, binding.Type, err)
				}
			}
			if !matched {
				continue
			}
			event := monitor.Event{
				ID:          rawEvent.ID + ":" + binding.Type,
				GeneratorID: generatorID,
				Type:        binding.Type,
				Message:     binding.Message,
				OccurredAt:  rawEvent.OccurredAt.UTC(),
			}
			events = append(events, event)
		}
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].OccurredAt.Equal(events[j].OccurredAt) {
			return events[i].ID < events[j].ID
		}
		return events[i].OccurredAt.After(events[j].OccurredAt)
	})
	return events, nil
}

func (r *SemanticReader) Health(ctx context.Context) error {
	return r.raw.Health(ctx)
}

func (e RawEvent) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return errors.New("rapid raw event id is required")
	}
	if e.ChannelNumber < 0 {
		return fmt.Errorf("rapid raw event %q channel number must be non-negative", e.ID)
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("rapid raw event %q occurredAt is required", e.ID)
	}
	if math.IsNaN(e.Value) || math.IsInf(e.Value, 0) || math.IsNaN(e.PreviousValue) || math.IsInf(e.PreviousValue, 0) {
		return fmt.Errorf("rapid raw event %q values must be finite", e.ID)
	}
	return nil
}

func semanticChannelMap(data []ChannelData) (map[int]ChannelData, error) {
	byChannel := make(map[int]ChannelData, len(data))
	for _, sample := range data {
		if sample.ChannelNumber <= 0 {
			return nil, errors.New("rapid raw reader returned invalid channel number")
		}
		if _, exists := byChannel[sample.ChannelNumber]; exists {
			return nil, fmt.Errorf("rapid raw reader returned duplicate channel %d", sample.ChannelNumber)
		}
		if sample.Defined() && sample.ObservedAt.IsZero() {
			return nil, fmt.Errorf("rapid channel %d is defined but has no observedAt", sample.ChannelNumber)
		}
		byChannel[sample.ChannelNumber] = sample
	}
	return byChannel, nil
}

func latestRaisedAt(events []RawEvent, binding AlarmBinding) *time.Time {
	var latest time.Time
	for _, event := range events {
		if event.ChannelNumber != binding.ChannelNumber || event.PreviousStatus <= 0 || event.Status <= 0 {
			continue
		}
		wasActive, err := binding.Active.Matches(event.PreviousValue)
		if err != nil {
			continue
		}
		isActive, err := binding.Active.Matches(event.Value)
		if err != nil || wasActive || !isActive {
			continue
		}
		occurredAt := event.OccurredAt.UTC()
		if occurredAt.After(latest) {
			latest = occurredAt
		}
	}
	if latest.IsZero() {
		return nil
	}
	return &latest
}

func alarmDefinitions(bundle profile.Bundle) map[string]profile.AlarmDefinition {
	definitions := make(map[string]profile.AlarmDefinition, len(bundle.Alarms.Alarms))
	for _, definition := range bundle.Alarms.Alarms {
		definitions[definition.Code] = definition
	}
	return definitions
}

var _ Reader = (*SemanticReader)(nil)
