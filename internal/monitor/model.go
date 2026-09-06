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
	MetricEngineRPM             MetricKey = "engine.rpm"
	MetricEngineState           MetricKey = "engine.state"
	MetricEngineOilPressure     MetricKey = "engine.oil_pressure"
	MetricEngineOilTemperature  MetricKey = "engine.oil_temperature"
	MetricEngineOilLevel        MetricKey = "engine.oil_level"
	MetricEngineCoolantTemp     MetricKey = "engine.coolant_temperature"
	MetricEngineCoolantLevel    MetricKey = "engine.coolant_level"
	MetricEngineRunHours        MetricKey = "engine.run_hours"
	MetricEngineStarts          MetricKey = "engine.starts"
	MetricGeneratorStatus       MetricKey = "generator.status"
	MetricGeneratorVoltageL1    MetricKey = "generator.voltage_l1"
	MetricGeneratorVoltageL2    MetricKey = "generator.voltage_l2"
	MetricGeneratorVoltageL3    MetricKey = "generator.voltage_l3"
	MetricGeneratorVoltageL1L2  MetricKey = "generator.voltage_l1_l2"
	MetricGeneratorVoltageL2L3  MetricKey = "generator.voltage_l2_l3"
	MetricGeneratorVoltageL3L1  MetricKey = "generator.voltage_l3_l1"
	MetricGeneratorFrequency    MetricKey = "generator.frequency"
	MetricGeneratorCurrentL1    MetricKey = "generator.current_l1"
	MetricGeneratorCurrentL2    MetricKey = "generator.current_l2"
	MetricGeneratorCurrentL3    MetricKey = "generator.current_l3"
	MetricGeneratorPowerKW      MetricKey = "generator.power_kw"
	MetricGeneratorPowerKVA     MetricKey = "generator.power_kva"
	MetricGeneratorPowerKVAR    MetricKey = "generator.power_kvar"
	MetricGeneratorPowerFactor  MetricKey = "generator.power_factor"
	MetricGeneratorEnergyKWh    MetricKey = "generator.energy_kwh"
	MetricGeneratorLoadPercent  MetricKey = "generator.load_percent"
	MetricMainsState            MetricKey = "mains.state"
	MetricMainsVoltageL1        MetricKey = "mains.voltage_l1"
	MetricMainsVoltageL2        MetricKey = "mains.voltage_l2"
	MetricMainsVoltageL3        MetricKey = "mains.voltage_l3"
	MetricMainsVoltageL1L2      MetricKey = "mains.voltage_l1_l2"
	MetricMainsVoltageL2L3      MetricKey = "mains.voltage_l2_l3"
	MetricMainsVoltageL3L1      MetricKey = "mains.voltage_l3_l1"
	MetricMainsFrequency        MetricKey = "mains.frequency"
	MetricControllerMode        MetricKey = "controller.mode"
	MetricControllerStatus      MetricKey = "controller.status"
	MetricControllerTemperature MetricKey = "controller.temperature"
	MetricBreakerGCB            MetricKey = "breaker.gcb"
	MetricBreakerMCB            MetricKey = "breaker.mcb"
	MetricATSState              MetricKey = "ats.state"
	MetricBatteryVoltage        MetricKey = "battery.voltage"
	MetricBatteryCurrent        MetricKey = "battery.current"
	MetricBatteryChargerVoltage MetricKey = "battery.charger_voltage"
	MetricBatteryChargerCurrent MetricKey = "battery.charger_current"
	MetricFuelLevel             MetricKey = "fuel.level"
	MetricFuelConsumptionRate   MetricKey = "fuel.consumption_rate"
	MetricFuelTotalConsumption  MetricKey = "fuel.total_consumption"
	MetricMaintenanceHours      MetricKey = "maintenance.hours_remaining"
	MetricMaintenanceDue        MetricKey = "maintenance.due"
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
	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	Firmware        string `json:"firmware,omitempty"`
	HardwareVersion string `json:"hardwareVersion,omitempty"`
	SerialNumber    string `json:"serialNumber,omitempty"`
}

type GeneratorSpec struct {
	RatedPowerKW     *float64 `json:"ratedPowerKw,omitempty"`
	NominalVoltage   *float64 `json:"nominalVoltage,omitempty"`
	NominalFrequency *float64 `json:"nominalFrequency,omitempty"`
	NominalRPM       *float64 `json:"nominalRpm,omitempty"`
	PhaseCount       *int     `json:"phaseCount,omitempty"`
}

type Generator struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	SiteID     string         `json:"siteId"`
	Controller ControllerRef  `json:"controller"`
	Spec       *GeneratorSpec `json:"spec,omitempty"`
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
	if g.Spec != nil {
		if err := g.Spec.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (s GeneratorSpec) Validate() error {
	if s.RatedPowerKW != nil && !finitePositiveOrZero(*s.RatedPowerKW) {
		return errors.New("generator ratedPowerKw must be finite and non-negative")
	}
	if s.NominalVoltage != nil && !finitePositive(*s.NominalVoltage) {
		return errors.New("generator nominalVoltage must be finite and positive")
	}
	if s.NominalFrequency != nil && !finitePositive(*s.NominalFrequency) {
		return errors.New("generator nominalFrequency must be finite and positive")
	}
	if s.NominalRPM != nil && !finitePositive(*s.NominalRPM) {
		return errors.New("generator nominalRpm must be finite and positive")
	}
	if s.PhaseCount != nil && *s.PhaseCount != 1 && *s.PhaseCount != 3 {
		return errors.New("generator phaseCount must be 1 or 3")
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

func IsKnownMetricKey(key MetricKey) bool {
	switch key {
	case MetricEngineRPM,
		MetricEngineState,
		MetricEngineOilPressure,
		MetricEngineOilTemperature,
		MetricEngineOilLevel,
		MetricEngineCoolantTemp,
		MetricEngineCoolantLevel,
		MetricEngineRunHours,
		MetricEngineStarts,
		MetricGeneratorStatus,
		MetricGeneratorVoltageL1,
		MetricGeneratorVoltageL2,
		MetricGeneratorVoltageL3,
		MetricGeneratorVoltageL1L2,
		MetricGeneratorVoltageL2L3,
		MetricGeneratorVoltageL3L1,
		MetricGeneratorFrequency,
		MetricGeneratorCurrentL1,
		MetricGeneratorCurrentL2,
		MetricGeneratorCurrentL3,
		MetricGeneratorPowerKW,
		MetricGeneratorPowerKVA,
		MetricGeneratorPowerKVAR,
		MetricGeneratorPowerFactor,
		MetricGeneratorEnergyKWh,
		MetricGeneratorLoadPercent,
		MetricMainsState,
		MetricMainsVoltageL1,
		MetricMainsVoltageL2,
		MetricMainsVoltageL3,
		MetricMainsVoltageL1L2,
		MetricMainsVoltageL2L3,
		MetricMainsVoltageL3L1,
		MetricMainsFrequency,
		MetricControllerMode,
		MetricControllerStatus,
		MetricControllerTemperature,
		MetricBreakerGCB,
		MetricBreakerMCB,
		MetricATSState,
		MetricBatteryVoltage,
		MetricBatteryCurrent,
		MetricBatteryChargerVoltage,
		MetricBatteryChargerCurrent,
		MetricFuelLevel,
		MetricFuelConsumptionRate,
		MetricFuelTotalConsumption,
		MetricMaintenanceHours,
		MetricMaintenanceDue:
		return true
	default:
		return false
	}
}

func IsValidValueKind(kind ValueKind) bool {
	switch kind {
	case ValueNumber, ValueText, ValueBoolean:
		return true
	default:
		return false
	}
}

func IsValidAlarmSeverity(severity AlarmSeverity) bool {
	switch severity {
	case AlarmInfo, AlarmWarning, AlarmCritical:
		return true
	default:
		return false
	}
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

func finitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func finitePositiveOrZero(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}
