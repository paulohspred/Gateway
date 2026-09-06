package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/paulohspred/Gateway/internal/monitor"
)

const DraftCatalogSchemaVersion = 1

type DraftCatalog struct {
	Schema   int               `json:"schema"`
	Profiles []DraftController `json:"profiles"`
}

type DraftController struct {
	ID           string                 `json:"id"`
	Manufacturer string                 `json:"manufacturer"`
	Model        string                 `json:"model"`
	DisplayName  string                 `json:"displayName"`
	Metrics      []monitor.MetricKey    `json:"metrics"`
	Alarms       []DraftAlarmDefinition `json:"alarms"`
}

type DraftAlarmDefinition struct {
	Code     string                `json:"code"`
	Severity monitor.AlarmSeverity `json:"severity"`
	Message  string                `json:"message"`
}

func LoadDraftCatalog(path string) (map[string]Bundle, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, maxProfileFileBytes))
	decoder.DisallowUnknownFields()

	var catalog DraftCatalog
	if err := decoder.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("decode draft controller catalog: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("draft controller catalog contains multiple JSON values")
		}
		return nil, err
	}
	return catalog.Bundles()
}

func (c DraftCatalog) Bundles() (map[string]Bundle, error) {
	if c.Schema != DraftCatalogSchemaVersion {
		return nil, fmt.Errorf("draft controller catalog schema must be %d, got %d", DraftCatalogSchemaVersion, c.Schema)
	}
	if len(c.Profiles) == 0 {
		return nil, errors.New("draft controller catalog must contain at least one profile")
	}

	bundles := make(map[string]Bundle, len(c.Profiles))
	for _, controller := range c.Profiles {
		if _, exists := bundles[controller.ID]; exists {
			return nil, fmt.Errorf("duplicate draft controller profile %q", controller.ID)
		}
		bundle, err := controller.Bundle()
		if err != nil {
			return nil, fmt.Errorf("draft controller %q: %w", controller.ID, err)
		}
		bundles[controller.ID] = bundle
	}
	return bundles, nil
}

func (d DraftController) Bundle() (Bundle, error) {
	if !profileIDPattern.MatchString(d.ID) {
		return Bundle{}, fmt.Errorf("invalid profile id %q", d.ID)
	}
	if strings.TrimSpace(d.Manufacturer) == "" {
		return Bundle{}, errors.New("manufacturer is required")
	}
	if strings.TrimSpace(d.Model) == "" {
		return Bundle{}, errors.New("model is required")
	}
	if strings.TrimSpace(d.DisplayName) == "" {
		return Bundle{}, errors.New("displayName is required")
	}
	if len(d.Metrics) == 0 {
		return Bundle{}, errors.New("at least one canonical metric is required")
	}

	metrics := make([]MetricDefinition, 0, len(d.Metrics))
	seenMetrics := make(map[monitor.MetricKey]struct{}, len(d.Metrics))
	for _, key := range d.Metrics {
		if _, exists := seenMetrics[key]; exists {
			return Bundle{}, fmt.Errorf("duplicate metric %q", key)
		}
		seenMetrics[key] = struct{}{}
		metadata, ok := canonicalMetricMetadata(key)
		if !ok {
			return Bundle{}, fmt.Errorf("unknown canonical metric %q", key)
		}
		metrics = append(metrics, MetricDefinition{
			Key:               key,
			DisplayName:       metadata.DisplayName,
			Kind:              metadata.Kind,
			Unit:              metadata.Unit,
			Required:          canonicalMetricRequired(key),
			StaleAfterSeconds: metadata.StaleAfterSeconds,
		})
	}

	alarms := make([]AlarmDefinition, 0, len(d.Alarms))
	seenAlarms := make(map[string]struct{}, len(d.Alarms))
	for _, alarm := range d.Alarms {
		if !alarmCodePattern.MatchString(alarm.Code) {
			return Bundle{}, fmt.Errorf("invalid alarm code %q", alarm.Code)
		}
		if _, exists := seenAlarms[alarm.Code]; exists {
			return Bundle{}, fmt.Errorf("duplicate alarm code %q", alarm.Code)
		}
		seenAlarms[alarm.Code] = struct{}{}
		if !monitor.IsValidAlarmSeverity(alarm.Severity) {
			return Bundle{}, fmt.Errorf("alarm %q has invalid severity %q", alarm.Code, alarm.Severity)
		}
		if strings.TrimSpace(alarm.Message) == "" {
			return Bundle{}, fmt.Errorf("alarm %q message is required", alarm.Code)
		}
		alarms = append(alarms, AlarmDefinition(alarm))
	}

	bundle := Bundle{
		Manifest: Manifest{
			Schema:       SchemaVersion,
			ID:           d.ID,
			Manufacturer: d.Manufacturer,
			Model:        d.Model,
			DisplayName:  d.DisplayName,
			Status:       StatusDraft,
			Capabilities: Capabilities{
				Telemetry:     true,
				Alarms:        len(alarms) > 0,
				Events:        false,
				Maintenance:   hasMetricPrefix(d.Metrics, "maintenance."),
				RemoteControl: false,
			},
			Files: ProfileFiles{
				Telemetry: "telemetry.json",
				Alarms:    "alarms.json",
				UI:        "ui.json",
			},
		},
		Telemetry: TelemetryFile{
			Schema:    SchemaVersion,
			ProfileID: d.ID,
			Metrics:   metrics,
		},
		Alarms: AlarmsFile{
			Schema:    SchemaVersion,
			ProfileID: d.ID,
			Alarms:    alarms,
		},
		UI: UIFile{
			Schema:    SchemaVersion,
			ProfileID: d.ID,
			Sections:  draftUISections(d.Metrics),
		},
	}
	if err := bundle.Validate(); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

type metricMetadata struct {
	DisplayName       string
	Kind              monitor.ValueKind
	Unit              string
	StaleAfterSeconds int
}

func canonicalMetricMetadata(key monitor.MetricKey) (metricMetadata, bool) {
	number := func(name, unit string, stale int) (metricMetadata, bool) {
		return metricMetadata{DisplayName: name, Kind: monitor.ValueNumber, Unit: unit, StaleAfterSeconds: stale}, true
	}
	text := func(name string, stale int) (metricMetadata, bool) {
		return metricMetadata{DisplayName: name, Kind: monitor.ValueText, StaleAfterSeconds: stale}, true
	}
	boolean := func(name string, stale int) (metricMetadata, bool) {
		return metricMetadata{DisplayName: name, Kind: monitor.ValueBoolean, StaleAfterSeconds: stale}, true
	}

	switch key {
	case monitor.MetricEngineRPM:
		return number("Engine RPM", "rpm", 10)
	case monitor.MetricEngineState:
		return text("Engine State", 10)
	case monitor.MetricEngineOilPressure:
		return number("Oil Pressure", "bar", 10)
	case monitor.MetricEngineOilTemperature:
		return number("Oil Temperature", "C", 10)
	case monitor.MetricEngineOilLevel:
		return number("Oil Level", "%", 30)
	case monitor.MetricEngineCoolantTemp:
		return number("Coolant Temperature", "C", 10)
	case monitor.MetricEngineCoolantLevel:
		return number("Coolant Level", "%", 30)
	case monitor.MetricEngineRunHours:
		return number("Run Hours", "h", 300)
	case monitor.MetricEngineStarts:
		return number("Engine Starts", "", 300)
	case monitor.MetricGeneratorStatus:
		return text("Generator Status", 10)
	case monitor.MetricGeneratorVoltageL1:
		return number("Generator Voltage L1", "V", 10)
	case monitor.MetricGeneratorVoltageL2:
		return number("Generator Voltage L2", "V", 10)
	case monitor.MetricGeneratorVoltageL3:
		return number("Generator Voltage L3", "V", 10)
	case monitor.MetricGeneratorVoltageL1L2:
		return number("Generator Voltage L1-L2", "V", 10)
	case monitor.MetricGeneratorVoltageL2L3:
		return number("Generator Voltage L2-L3", "V", 10)
	case monitor.MetricGeneratorVoltageL3L1:
		return number("Generator Voltage L3-L1", "V", 10)
	case monitor.MetricGeneratorFrequency:
		return number("Generator Frequency", "Hz", 10)
	case monitor.MetricGeneratorCurrentL1:
		return number("Generator Current L1", "A", 10)
	case monitor.MetricGeneratorCurrentL2:
		return number("Generator Current L2", "A", 10)
	case monitor.MetricGeneratorCurrentL3:
		return number("Generator Current L3", "A", 10)
	case monitor.MetricGeneratorPowerKW:
		return number("Active Power", "kW", 10)
	case monitor.MetricGeneratorPowerKVA:
		return number("Apparent Power", "kVA", 10)
	case monitor.MetricGeneratorPowerKVAR:
		return number("Reactive Power", "kVAr", 10)
	case monitor.MetricGeneratorPowerFactor:
		return number("Power Factor", "", 10)
	case monitor.MetricGeneratorEnergyKWh:
		return number("Generated Energy", "kWh", 300)
	case monitor.MetricGeneratorLoadPercent:
		return number("Generator Load", "%", 10)
	case monitor.MetricMainsState:
		return text("Mains State", 10)
	case monitor.MetricMainsVoltageL1:
		return number("Mains Voltage L1", "V", 10)
	case monitor.MetricMainsVoltageL2:
		return number("Mains Voltage L2", "V", 10)
	case monitor.MetricMainsVoltageL3:
		return number("Mains Voltage L3", "V", 10)
	case monitor.MetricMainsVoltageL1L2:
		return number("Mains Voltage L1-L2", "V", 10)
	case monitor.MetricMainsVoltageL2L3:
		return number("Mains Voltage L2-L3", "V", 10)
	case monitor.MetricMainsVoltageL3L1:
		return number("Mains Voltage L3-L1", "V", 10)
	case monitor.MetricMainsFrequency:
		return number("Mains Frequency", "Hz", 10)
	case monitor.MetricControllerMode:
		return text("Controller Mode", 10)
	case monitor.MetricControllerStatus:
		return text("Controller Status", 10)
	case monitor.MetricControllerTemperature:
		return number("Controller Temperature", "C", 30)
	case monitor.MetricBreakerGCB:
		return boolean("Generator Breaker", 10)
	case monitor.MetricBreakerMCB:
		return boolean("Mains Breaker", 10)
	case monitor.MetricATSState:
		return text("ATS State", 10)
	case monitor.MetricBatteryVoltage:
		return number("Battery Voltage", "V", 30)
	case monitor.MetricBatteryCurrent:
		return number("Battery Current", "A", 30)
	case monitor.MetricBatteryChargerVoltage:
		return number("Battery Charger Voltage", "V", 30)
	case monitor.MetricBatteryChargerCurrent:
		return number("Battery Charger Current", "A", 30)
	case monitor.MetricFuelLevel:
		return number("Fuel Level", "%", 60)
	case monitor.MetricFuelConsumptionRate:
		return number("Fuel Consumption Rate", "L/h", 60)
	case monitor.MetricFuelTotalConsumption:
		return number("Total Fuel Consumption", "L", 300)
	case monitor.MetricMaintenanceHours:
		return number("Maintenance Hours Remaining", "h", 300)
	case monitor.MetricMaintenanceDue:
		return boolean("Maintenance Due", 300)
	default:
		return metricMetadata{}, false
	}
}

func canonicalMetricRequired(key monitor.MetricKey) bool {
	switch key {
	case monitor.MetricEngineRPM, monitor.MetricGeneratorVoltageL1, monitor.MetricGeneratorFrequency:
		return true
	default:
		return false
	}
}

func hasMetricPrefix(metrics []monitor.MetricKey, prefix string) bool {
	for _, key := range metrics {
		if strings.HasPrefix(string(key), prefix) {
			return true
		}
	}
	return false
}

func draftUISections(metrics []monitor.MetricKey) []UISection {
	type group struct {
		ID       string
		Title    string
		Prefixes []string
	}
	groups := []group{
		{ID: "engine", Title: "Engine", Prefixes: []string{"engine."}},
		{ID: "generator", Title: "Generator", Prefixes: []string{"generator."}},
		{ID: "mains", Title: "Mains", Prefixes: []string{"mains."}},
		{ID: "control", Title: "Control", Prefixes: []string{"controller.", "breaker.", "ats."}},
		{ID: "dc-fuel", Title: "DC and Fuel", Prefixes: []string{"battery.", "fuel."}},
		{ID: "maintenance", Title: "Maintenance", Prefixes: []string{"maintenance."}},
	}

	sections := make([]UISection, 0, len(groups))
	for _, group := range groups {
		keys := make([]monitor.MetricKey, 0)
		for _, key := range metrics {
			for _, prefix := range group.Prefixes {
				if strings.HasPrefix(string(key), prefix) {
					keys = append(keys, key)
					break
				}
			}
		}
		if len(keys) > 0 {
			sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
			sections = append(sections, UISection{ID: group.ID, Title: group.Title, Metrics: keys})
		}
	}
	return sections
}
