package rapid

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/profile"
)

const BindingSchemaVersion = 1

type BindingFile struct {
	Schema    int              `json:"schema"`
	ProfileID string           `json:"profileId"`
	Metrics   []ChannelBinding `json:"metrics"`
	Alarms    []AlarmBinding   `json:"alarms,omitempty"`
	Events    []EventBinding   `json:"events,omitempty"`
}

type ChannelBinding struct {
	Key           monitor.MetricKey `json:"key"`
	ChannelNumber int               `json:"channelNumber"`
	Transform     Transform         `json:"transform"`
}

type AlarmBinding struct {
	Code          string    `json:"code"`
	ChannelNumber int       `json:"channelNumber"`
	Active        Condition `json:"active"`
}

type EventBinding struct {
	Type          string     `json:"type"`
	ChannelNumber int        `json:"channelNumber"`
	Condition     *Condition `json:"condition,omitempty"`
	Message       string     `json:"message"`
}

type ConditionKind string

const (
	ConditionEquals  ConditionKind = "equals"
	ConditionOneOf   ConditionKind = "one_of"
	ConditionNonZero ConditionKind = "nonzero"
	ConditionBitSet  ConditionKind = "bit_set"
	ConditionGT      ConditionKind = "gt"
	ConditionGTE     ConditionKind = "gte"
	ConditionLT      ConditionKind = "lt"
	ConditionLTE     ConditionKind = "lte"
)

type Condition struct {
	Kind   ConditionKind `json:"kind"`
	Value  *float64      `json:"value,omitempty"`
	Values []float64     `json:"values,omitempty"`
	Bit    *uint         `json:"bit,omitempty"`
}

func LoadBinding(path string, bundle profile.Bundle) (BindingFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return BindingFile{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var binding BindingFile
	if err := decoder.Decode(&binding); err != nil {
		return BindingFile{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return BindingFile{}, errors.New("multiple JSON values are not allowed")
		}
		return BindingFile{}, err
	}
	if err := binding.Validate(bundle); err != nil {
		return BindingFile{}, err
	}
	return binding, nil
}

func (b BindingFile) Validate(bundle profile.Bundle) error {
	if b.Schema != BindingSchemaVersion {
		return fmt.Errorf("rapid binding schema must be %d, got %d", BindingSchemaVersion, b.Schema)
	}
	if err := bundle.Validate(); err != nil {
		return fmt.Errorf("profile bundle invalid: %w", err)
	}
	if b.ProfileID != bundle.Manifest.ID {
		return fmt.Errorf("rapid binding profileId %q does not match manifest %q", b.ProfileID, bundle.Manifest.ID)
	}
	if len(b.Metrics) == 0 {
		return errors.New("rapid binding must contain at least one metric")
	}

	definitions := metricDefinitions(bundle)
	seenMetrics := make(map[monitor.MetricKey]struct{}, len(b.Metrics))
	for _, binding := range b.Metrics {
		definition, ok := definitions[binding.Key]
		if !ok {
			return fmt.Errorf("rapid binding references undefined metric %q", binding.Key)
		}
		if _, ok := seenMetrics[binding.Key]; ok {
			return fmt.Errorf("duplicate rapid binding metric %q", binding.Key)
		}
		seenMetrics[binding.Key] = struct{}{}
		if binding.ChannelNumber <= 0 {
			return fmt.Errorf("metric %q channelNumber must be positive", binding.Key)
		}
		if err := binding.Transform.Validate(definition.Kind); err != nil {
			return fmt.Errorf("metric %q transform: %w", binding.Key, err)
		}
	}
	for _, definition := range bundle.Telemetry.Metrics {
		if definition.Required {
			if _, ok := seenMetrics[definition.Key]; !ok {
				return fmt.Errorf("required metric %q has no rapid channel binding", definition.Key)
			}
		}
	}

	alarmDefinitions := make(map[string]profile.AlarmDefinition, len(bundle.Alarms.Alarms))
	for _, definition := range bundle.Alarms.Alarms {
		alarmDefinitions[definition.Code] = definition
	}
	seenAlarms := make(map[string]struct{}, len(b.Alarms))
	for _, binding := range b.Alarms {
		if _, ok := alarmDefinitions[binding.Code]; !ok {
			return fmt.Errorf("rapid alarm binding references undefined alarm %q", binding.Code)
		}
		if _, ok := seenAlarms[binding.Code]; ok {
			return fmt.Errorf("duplicate rapid alarm binding %q", binding.Code)
		}
		seenAlarms[binding.Code] = struct{}{}
		if binding.ChannelNumber <= 0 {
			return fmt.Errorf("alarm %q channelNumber must be positive", binding.Code)
		}
		if err := binding.Active.Validate(); err != nil {
			return fmt.Errorf("alarm %q active condition: %w", binding.Code, err)
		}
	}
	if bundle.Manifest.Capabilities.Alarms && len(b.Alarms) == 0 {
		return errors.New("profile declares alarms capability but Rapid binding has no alarm bindings")
	}

	for i, binding := range b.Events {
		if strings.TrimSpace(binding.Type) == "" {
			return fmt.Errorf("events[%d] type is required", i)
		}
		if strings.ContainsAny(binding.Type, " \t\r\n/") {
			return fmt.Errorf("events[%d] type %q is invalid", i, binding.Type)
		}
		if binding.ChannelNumber <= 0 {
			return fmt.Errorf("event %q channelNumber must be positive", binding.Type)
		}
		if strings.TrimSpace(binding.Message) == "" {
			return fmt.Errorf("event %q message is required", binding.Type)
		}
		if binding.Condition != nil {
			if err := binding.Condition.Validate(); err != nil {
				return fmt.Errorf("event %q condition: %w", binding.Type, err)
			}
		}
	}
	if bundle.Manifest.Capabilities.Events && len(b.Events) == 0 && len(b.Alarms) == 0 {
		return errors.New("profile declares events capability but Rapid binding has no event-capable bindings")
	}
	return nil
}

func (b BindingFile) ChannelNumbers() []int {
	set := make(map[int]struct{}, len(b.Metrics))
	for _, binding := range b.Metrics {
		set[binding.ChannelNumber] = struct{}{}
	}
	return sortedChannelSet(set)
}

func (b BindingFile) AlarmChannelNumbers() []int {
	set := make(map[int]struct{}, len(b.Alarms))
	for _, binding := range b.Alarms {
		set[binding.ChannelNumber] = struct{}{}
	}
	return sortedChannelSet(set)
}

func (b BindingFile) SemanticChannelNumbers() []int {
	set := make(map[int]struct{}, len(b.Alarms)+len(b.Events))
	for _, binding := range b.Alarms {
		set[binding.ChannelNumber] = struct{}{}
	}
	for _, binding := range b.Events {
		set[binding.ChannelNumber] = struct{}{}
	}
	return sortedChannelSet(set)
}

func sortedChannelSet(set map[int]struct{}) []int {
	channels := make([]int, 0, len(set))
	for channel := range set {
		channels = append(channels, channel)
	}
	sort.Ints(channels)
	return channels
}

func (c Condition) Validate() error {
	hasValue := c.Value != nil
	hasValues := len(c.Values) > 0
	hasBit := c.Bit != nil
	validateFinite := func(value float64) error {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("condition values must be finite")
		}
		return nil
	}

	switch c.Kind {
	case ConditionEquals, ConditionGT, ConditionGTE, ConditionLT, ConditionLTE:
		if !hasValue || hasValues || hasBit {
			return fmt.Errorf("%s condition requires only value", c.Kind)
		}
		return validateFinite(*c.Value)
	case ConditionOneOf:
		if hasValue || !hasValues || hasBit {
			return errors.New("one_of condition requires only values")
		}
		seen := make(map[float64]struct{}, len(c.Values))
		for _, value := range c.Values {
			if err := validateFinite(value); err != nil {
				return err
			}
			if _, ok := seen[value]; ok {
				return fmt.Errorf("one_of condition contains duplicate value %v", value)
			}
			seen[value] = struct{}{}
		}
		return nil
	case ConditionNonZero:
		if hasValue || hasValues || hasBit {
			return errors.New("nonzero condition takes no parameters")
		}
		return nil
	case ConditionBitSet:
		if hasValue || hasValues || !hasBit {
			return errors.New("bit_set condition requires only bit")
		}
		if *c.Bit > 31 {
			return errors.New("bit_set bit must be between 0 and 31")
		}
		return nil
	default:
		return fmt.Errorf("invalid condition kind %q", c.Kind)
	}
}

func (c Condition) Matches(raw float64) (bool, error) {
	if math.IsNaN(raw) || math.IsInf(raw, 0) {
		return false, errors.New("condition raw value must be finite")
	}
	if err := c.Validate(); err != nil {
		return false, err
	}
	switch c.Kind {
	case ConditionEquals:
		return raw == *c.Value, nil
	case ConditionOneOf:
		for _, value := range c.Values {
			if raw == value {
				return true, nil
			}
		}
		return false, nil
	case ConditionNonZero:
		return raw != 0, nil
	case ConditionBitSet:
		if raw < 0 || raw > float64(^uint32(0)) || math.Trunc(raw) != raw {
			return false, fmt.Errorf("bit_set requires an unsigned 32-bit integer raw value, got %v", raw)
		}
		return uint32(raw)&(uint32(1)<<*c.Bit) != 0, nil
	case ConditionGT:
		return raw > *c.Value, nil
	case ConditionGTE:
		return raw >= *c.Value, nil
	case ConditionLT:
		return raw < *c.Value, nil
	case ConditionLTE:
		return raw <= *c.Value, nil
	default:
		return false, fmt.Errorf("unsupported condition kind %q", c.Kind)
	}
}
