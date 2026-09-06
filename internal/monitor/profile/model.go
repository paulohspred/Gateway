package profile

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/paulohspred/Gateway/internal/monitor"
)

const SchemaVersion = 1

var (
	profileIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	alarmCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_.-]*$`)
)

type ProfileStatus string

const (
	StatusSynthetic ProfileStatus = "synthetic"
	StatusDraft     ProfileStatus = "draft"
	StatusValidated ProfileStatus = "validated"
)

type Manifest struct {
	Schema       int          `json:"schema"`
	ID           string       `json:"id"`
	Manufacturer string       `json:"manufacturer"`
	Model        string       `json:"model"`
	DisplayName  string       `json:"displayName"`
	Status       ProfileStatus `json:"status"`
	Capabilities Capabilities `json:"capabilities"`
	Files        ProfileFiles `json:"files"`
}

type Capabilities struct {
	Telemetry     bool `json:"telemetry"`
	Alarms        bool `json:"alarms"`
	Events        bool `json:"events"`
	Maintenance   bool `json:"maintenance"`
	RemoteControl bool `json:"remoteControl"`
}

type ProfileFiles struct {
	Telemetry string `json:"telemetry"`
	Alarms    string `json:"alarms"`
	UI        string `json:"ui"`
}

type TelemetryFile struct {
	Schema    int                `json:"schema"`
	ProfileID string             `json:"profileId"`
	Metrics   []MetricDefinition `json:"metrics"`
}

type MetricDefinition struct {
	Key               monitor.MetricKey `json:"key"`
	DisplayName       string            `json:"displayName"`
	Kind              monitor.ValueKind `json:"kind"`
	Unit              string            `json:"unit,omitempty"`
	Required          bool              `json:"required"`
	StaleAfterSeconds int               `json:"staleAfterSeconds"`
}

type AlarmsFile struct {
	Schema    int               `json:"schema"`
	ProfileID string            `json:"profileId"`
	Alarms    []AlarmDefinition `json:"alarms"`
}

type AlarmDefinition struct {
	Code     string                `json:"code"`
	Severity monitor.AlarmSeverity `json:"severity"`
	Message  string                `json:"message"`
}

type UIFile struct {
	Schema    int         `json:"schema"`
	ProfileID string      `json:"profileId"`
	Sections  []UISection `json:"sections"`
}

type UISection struct {
	ID      string              `json:"id"`
	Title   string              `json:"title"`
	Metrics []monitor.MetricKey `json:"metrics"`
}

type Bundle struct {
	Manifest  Manifest
	Telemetry TelemetryFile
	Alarms    AlarmsFile
	UI        UIFile
}

func (m Manifest) Validate() error {
	if m.Schema != SchemaVersion {
		return fmt.Errorf("manifest schema must be %d, got %d", SchemaVersion, m.Schema)
	}
	if !profileIDPattern.MatchString(m.ID) {
		return fmt.Errorf("invalid profile id %q", m.ID)
	}
	if strings.TrimSpace(m.Manufacturer) == "" {
		return errors.New("manufacturer is required")
	}
	if strings.TrimSpace(m.Model) == "" {
		return errors.New("model is required")
	}
	if strings.TrimSpace(m.DisplayName) == "" {
		return errors.New("displayName is required")
	}
	switch m.Status {
	case StatusSynthetic, StatusDraft, StatusValidated:
	default:
		return fmt.Errorf("invalid profile status %q", m.Status)
	}
	if !m.Capabilities.Telemetry {
		return errors.New("telemetry capability is required in controller profile schema v1")
	}
	for label, path := range map[string]string{
		"telemetry": m.Files.Telemetry,
		"alarms":    m.Files.Alarms,
		"ui":        m.Files.UI,
	} {
		if err := validateRelativeJSONPath(path); err != nil {
			return fmt.Errorf("%s file: %w", label, err)
		}
	}
	return nil
}

func (t TelemetryFile) Validate(profileID string) error {
	if t.Schema != SchemaVersion {
		return fmt.Errorf("telemetry schema must be %d, got %d", SchemaVersion, t.Schema)
	}
	if t.ProfileID != profileID {
		return fmt.Errorf("telemetry profileId %q does not match manifest %q", t.ProfileID, profileID)
	}
	if len(t.Metrics) == 0 {
		return errors.New("telemetry must define at least one metric")
	}
	seen := make(map[monitor.MetricKey]struct{}, len(t.Metrics))
	for _, metric := range t.Metrics {
		if !monitor.IsKnownMetricKey(metric.Key) {
			return fmt.Errorf("unknown canonical metric key %q", metric.Key)
		}
		if _, ok := seen[metric.Key]; ok {
			return fmt.Errorf("duplicate metric key %q", metric.Key)
		}
		seen[metric.Key] = struct{}{}
		if strings.TrimSpace(metric.DisplayName) == "" {
			return fmt.Errorf("metric %q displayName is required", metric.Key)
		}
		if !monitor.IsValidValueKind(metric.Kind) {
			return fmt.Errorf("metric %q has invalid kind %q", metric.Key, metric.Kind)
		}
		if metric.StaleAfterSeconds <= 0 || metric.StaleAfterSeconds > 86400 {
			return fmt.Errorf("metric %q staleAfterSeconds must be between 1 and 86400", metric.Key)
		}
	}
	return nil
}

func (a AlarmsFile) Validate(profileID string) error {
	if a.Schema != SchemaVersion {
		return fmt.Errorf("alarms schema must be %d, got %d", SchemaVersion, a.Schema)
	}
	if a.ProfileID != profileID {
		return fmt.Errorf("alarms profileId %q does not match manifest %q", a.ProfileID, profileID)
	}
	seen := make(map[string]struct{}, len(a.Alarms))
	for _, alarm := range a.Alarms {
		if !alarmCodePattern.MatchString(alarm.Code) {
			return fmt.Errorf("invalid alarm code %q", alarm.Code)
		}
		if _, ok := seen[alarm.Code]; ok {
			return fmt.Errorf("duplicate alarm code %q", alarm.Code)
		}
		seen[alarm.Code] = struct{}{}
		if !monitor.IsValidAlarmSeverity(alarm.Severity) {
			return fmt.Errorf("alarm %q has invalid severity %q", alarm.Code, alarm.Severity)
		}
		if strings.TrimSpace(alarm.Message) == "" {
			return fmt.Errorf("alarm %q message is required", alarm.Code)
		}
	}
	return nil
}

func (u UIFile) Validate(profileID string, telemetry TelemetryFile) error {
	if u.Schema != SchemaVersion {
		return fmt.Errorf("ui schema must be %d, got %d", SchemaVersion, u.Schema)
	}
	if u.ProfileID != profileID {
		return fmt.Errorf("ui profileId %q does not match manifest %q", u.ProfileID, profileID)
	}
	defined := make(map[monitor.MetricKey]struct{}, len(telemetry.Metrics))
	for _, metric := range telemetry.Metrics {
		defined[metric.Key] = struct{}{}
	}
	seenSections := make(map[string]struct{}, len(u.Sections))
	seenMetrics := make(map[monitor.MetricKey]struct{})
	for _, section := range u.Sections {
		if !profileIDPattern.MatchString(section.ID) {
			return fmt.Errorf("invalid ui section id %q", section.ID)
		}
		if _, ok := seenSections[section.ID]; ok {
			return fmt.Errorf("duplicate ui section id %q", section.ID)
		}
		seenSections[section.ID] = struct{}{}
		if strings.TrimSpace(section.Title) == "" {
			return fmt.Errorf("ui section %q title is required", section.ID)
		}
		for _, key := range section.Metrics {
			if _, ok := defined[key]; !ok {
				return fmt.Errorf("ui section %q references undefined metric %q", section.ID, key)
			}
			if _, ok := seenMetrics[key]; ok {
				return fmt.Errorf("ui metric %q appears in more than one section", key)
			}
			seenMetrics[key] = struct{}{}
		}
	}
	return nil
}

func (b Bundle) Validate() error {
	if err := b.Manifest.Validate(); err != nil {
		return err
	}
	if err := b.Telemetry.Validate(b.Manifest.ID); err != nil {
		return err
	}
	if err := b.Alarms.Validate(b.Manifest.ID); err != nil {
		return err
	}
	if err := b.UI.Validate(b.Manifest.ID, b.Telemetry); err != nil {
		return err
	}
	if b.Manifest.Capabilities.Alarms != (len(b.Alarms.Alarms) > 0) {
		return errors.New("manifest alarms capability must match alarms definitions")
	}
	return nil
}

func validateRelativeJSONPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is required")
	}
	if filepath.IsAbs(path) {
		return errors.New("absolute paths are not allowed")
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("path traversal is not allowed")
	}
	if filepath.Ext(clean) != ".json" {
		return errors.New("profile component must be a .json file")
	}
	return nil
}
