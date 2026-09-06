package monitor

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

type MetricKey string

const (
	MetricEngineRPM            MetricKey = "engine.rpm"
	MetricEngineOilPressure    MetricKey = "engine.oil_pressure"
	MetricEngineCoolantTemp    MetricKey = "engine.coolant_temperature"
	MetricEngineRunHours       MetricKey = "engine.run_hours"
	MetricGeneratorVoltageL1   MetricKey = "generator.voltage_l1"
	MetricGeneratorVoltageL2   MetricKey = "generator.voltage_l2"
	MetricGeneratorVoltageL3   MetricKey = "generator.voltage_l3"
	MetricGeneratorFrequency   MetricKey = "generator.frequency"
	MetricGeneratorCurrentL1   MetricKey = "generator.current_l1"
	MetricGeneratorPowerKW     MetricKey = "generator.power_kw"
	MetricGeneratorPowerKVA    MetricKey = "generator.power_kva"
	MetricGeneratorPowerKVAR   MetricKey = "generator.power_kvar"
	MetricGeneratorPowerFactor MetricKey = "generator.power_factor"
	MetricMainsVoltageL1       MetricKey = "mains.voltage_l1"
	MetricMainsFrequency       MetricKey = "mains.frequency"
	MetricControllerMode       MetricKey = "controller.mode"
	MetricBreakerGCB           MetricKey = "breaker.gcb"
	MetricBreakerMCB           MetricKey = "breaker.mcb"
	MetricBatteryVoltage       MetricKey = "battery.voltage"
	MetricFuelLevel            MetricKey = "fuel.level"
)

type ValueKind string

const (
	ValueNumber  ValueKind = "number"
	ValueText    ValueKind = "text"
	ValueBoolean ValueKind = "boolean"
)

type Quality string

const (
	QualityGood    Quality = "good"
	QualityStale   Quality = "stale"
	QualityOffline Quality = "offline"
	QualityBad     Quality = "bad"
	QualityUnknown Quality = "unknown"
)

type CommunicationState string

const (
	CommunicationOnline  CommunicationState = "online"
	CommunicationOffline CommunicationState = "offline"
	CommunicationUnknown CommunicationState = "unknown"
)

type ProviderStatus string

const (
	ProviderHealthy     ProviderStatus = "healthy"
	ProviderDegraded    ProviderStatus = "degraded"
	ProviderUnavailable ProviderStatus = "unavailable"
)

type AlarmSeverity string

const (
	AlarmInfo     AlarmSeverity = "info"
	AlarmWarning  AlarmSeverity = "warning"
	AlarmCritical AlarmSeverity = "critical"
)

// MetricValue is a strict tagged value. Exactly one field must be populated.
// A missing metric is represented by absence from TelemetrySnapshot.Metrics,
// never by an invented numeric zero.
type MetricValue struct {
	Number  *float64 `json:"number,omitempty"`
	Text    *string  `json:"text,omitempty"`
	Boolean *bool    `json:"boolean,omitempty"`
}

func NumberValue(value float64) MetricValue {
	v := value
	return MetricValue{Number: &v}
}

func TextValue(value string) MetricValue {
	v := value
	return MetricValue{Text: &v}
}

func BooleanValue(value bool) MetricValue {
	v := value
	return MetricValue{Boolean: &v}
}

func (v MetricValue) Kind() (ValueKind, error) {
	count := 0
	var kind ValueKind
	if v.Number != nil {
		count++
		kind = ValueNumber
	}
	if v.Text != nil {
		count++
		kind = ValueText
	}
	if v.Boolean != nil {
		count++
		kind = ValueBoolean
	}
	if count != 1 {
		return "", fmt.Errorf("metric value must contain exactly one value kind, got %d", count)
	}
	if v.Number != nil && (math.IsNaN(*v.Number) || math.IsInf(*v.Number, 0)) {
		return "", errors.New("numeric metric value must be finite")
	}
	return kind, nil
}

type Metric struct {
	Key        MetricKey   `json:"key"`
	Value      MetricValue `json:"value"`
	Unit       string      `json:"unit,omitempty"`
	Quality    Quality     `json:"quality"`
	ObservedAt time.Time   `json:"observedAt"`
}

func (m Metric) Validate() error {
	if strings.TrimSpace(string(m.Key)) == "" {
		return errors.New("metric key is required")
	}
	if _, err := m.Value.Kind(); err != nil {
		return fmt.Errorf("metric %q: %w", m.Key, err)
	}
	if !validQuality(m.Quality) {
		return fmt.Errorf("metric %q: invalid quality %q", m.Key, m.Quality)
	}
	if m.ObservedAt.IsZero() {
		return fmt.Errorf("metric %q: observedAt is required", m.Key)
	}
	return nil
}

type ControllerRef struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
}

type Generator struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	SiteID     string        `json:"siteId"`
	Controller ControllerRef `json:"controller"`
}

func (g Generator) Validate() error {
	if strings.TrimSpace(g.ID) == "" {
		return errors.New("generator id is required")
	}
	if strings.TrimSpace(g.Name) == "" {
		return errors.New("generator name is required")
	}
	if strings.TrimSpace(g.SiteID) == "" {
		return errors.New("generator site id is required")
	}
	if strings.TrimSpace(g.Controller.Manufacturer) == "" {
		return errors.New("controller manufacturer is required")
	}
	if strings.TrimSpace(g.Controller.Model) == "" {
		return errors.New("controller model is required")
	}
	return nil
}

type TelemetrySnapshot struct {
	GeneratorID   string               `json:"generatorId"`
	CapturedAt    time.Time            `json:"capturedAt"`
	Communication CommunicationState   `json:"communication"`
	Metrics       map[MetricKey]Metric `json:"metrics"`
}

func (s TelemetrySnapshot) Validate() error {
	if strings.TrimSpace(s.GeneratorID) == "" {
		return errors.New("telemetry generator id is required")
	}
	if s.CapturedAt.IsZero() {
		return errors.New("telemetry capturedAt is required")
	}
	if !validCommunicationState(s.Communication) {
		return fmt.Errorf("invalid communication state %q", s.Communication)
	}
	for key, metric := range s.Metrics {
		if metric.Key != key {
			return fmt.Errorf("metric map key %q does not match metric key %q", key, metric.Key)
		}
		if err := metric.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type Alarm struct {
	ID          string        `json:"id"`
	GeneratorID string        `json:"generatorId"`
	Code        string        `json:"code"`
	Severity    AlarmSeverity `json:"severity"`
	Message     string        `json:"message"`
	Active      bool          `json:"active"`
	RaisedAt    time.Time     `json:"raisedAt"`
	ClearedAt   *time.Time    `json:"clearedAt,omitempty"`
}

type Event struct {
	ID          string    `json:"id"`
	GeneratorID string    `json:"generatorId"`
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	OccurredAt  time.Time `json:"occurredAt"`
}

type ProviderHealth struct {
	Status    ProviderStatus `json:"status"`
	CheckedAt time.Time      `json:"checkedAt"`
	Message   string         `json:"message,omitempty"`
}

func validQuality(q Quality) bool {
	switch q {
	case QualityGood, QualityStale, QualityOffline, QualityBad, QualityUnknown:
		return true
	default:
		return false
	}
}

func validCommunicationState(state CommunicationState) bool {
	switch state {
	case CommunicationOnline, CommunicationOffline, CommunicationUnknown:
		return true
	default:
		return false
	}
}
