package rapid

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/profile"
)

const BindingSchemaVersion = 1

type BindingFile struct {
	Schema    int              `json:"schema"`
	ProfileID string           `json:"profileId"`
	Metrics   []ChannelBinding `json:"metrics"`
}

type ChannelBinding struct {
	Key           monitor.MetricKey `json:"key"`
	ChannelNumber int               `json:"channelNumber"`
	Transform     Transform         `json:"transform"`
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
		return fmt.Errorf("Rapid binding schema must be %d, got %d", BindingSchemaVersion, b.Schema)
	}
	if err := bundle.Validate(); err != nil {
		return fmt.Errorf("profile bundle invalid: %w", err)
	}
	if b.ProfileID != bundle.Manifest.ID {
		return fmt.Errorf("Rapid binding profileId %q does not match manifest %q", b.ProfileID, bundle.Manifest.ID)
	}
	if len(b.Metrics) == 0 {
		return errors.New("Rapid binding must contain at least one metric")
	}

	definitions := metricDefinitions(bundle)
	seen := make(map[monitor.MetricKey]struct{}, len(b.Metrics))
	for _, binding := range b.Metrics {
		definition, ok := definitions[binding.Key]
		if !ok {
			return fmt.Errorf("Rapid binding references undefined metric %q", binding.Key)
		}
		if _, ok := seen[binding.Key]; ok {
			return fmt.Errorf("duplicate Rapid binding metric %q", binding.Key)
		}
		seen[binding.Key] = struct{}{}
		if binding.ChannelNumber <= 0 {
			return fmt.Errorf("metric %q channelNumber must be positive", binding.Key)
		}
		if err := binding.Transform.Validate(definition.Kind); err != nil {
			return fmt.Errorf("metric %q transform: %w", binding.Key, err)
		}
	}
	for _, definition := range bundle.Telemetry.Metrics {
		if definition.Required {
			if _, ok := seen[definition.Key]; !ok {
				return fmt.Errorf("required metric %q has no Rapid channel binding", definition.Key)
			}
		}
	}
	return nil
}

func (b BindingFile) ChannelNumbers() []int {
	set := make(map[int]struct{}, len(b.Metrics))
	for _, binding := range b.Metrics {
		set[binding.ChannelNumber] = struct{}{}
	}
	channels := make([]int, 0, len(set))
	for channel := range set {
		channels = append(channels, channel)
	}
	sort.Ints(channels)
	return channels
}
