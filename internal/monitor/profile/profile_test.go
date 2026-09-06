package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paulohspred/Gateway/internal/monitor"
)

func TestLoadSyntheticProfile(t *testing.T) {
	root := filepath.Join("..", "..", "..", "controllers", "rc-simulator", "reference-controller")
	bundle, err := LoadDir(root)
	if err != nil {
		t.Fatalf("load synthetic profile: %v", err)
	}
	if bundle.Manifest.ID != "rc-simulator.reference-controller" {
		t.Fatalf("unexpected profile id %q", bundle.Manifest.ID)
	}
	if bundle.Manifest.Status != StatusSynthetic {
		t.Fatalf("unexpected profile status %q", bundle.Manifest.Status)
	}
	if bundle.Manifest.Capabilities.RemoteControl {
		t.Fatal("synthetic profile must not enable remote control")
	}
	if len(bundle.Telemetry.Metrics) < 10 {
		t.Fatalf("expected representative telemetry set, got %d", len(bundle.Telemetry.Metrics))
	}
	if len(bundle.Alarms.Alarms) != 1 {
		t.Fatalf("expected one synthetic alarm definition, got %d", len(bundle.Alarms.Alarms))
	}
}

func TestLoadRejectsUnknownJSONFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifest.json"), `{
  "schema": 1,
  "id": "test.controller",
  "manufacturer": "Test",
  "model": "Controller",
  "displayName": "Test Controller",
  "status": "synthetic",
  "capabilities": {"telemetry": true, "alarms": false, "events": false, "maintenance": false, "remoteControl": false},
  "files": {"telemetry": "telemetry.json", "alarms": "alarms.json", "ui": "ui.json"},
  "unexpected": true
}`)
	_, err := LoadDir(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestTelemetryValidationRejectsUnknownAndDuplicateMetricKeys(t *testing.T) {
	valid := TelemetryFile{
		Schema:    SchemaVersion,
		ProfileID: "test.controller",
		Metrics: []MetricDefinition{
			{Key: monitor.MetricEngineRPM, DisplayName: "RPM", Kind: monitor.ValueNumber, Unit: "rpm", StaleAfterSeconds: 10},
		},
	}
	if err := valid.Validate("test.controller"); err != nil {
		t.Fatalf("valid telemetry rejected: %v", err)
	}

	unknown := valid
	unknown.Metrics = []MetricDefinition{{Key: monitor.MetricKey("vendor.private_register"), DisplayName: "Bad", Kind: monitor.ValueNumber, StaleAfterSeconds: 10}}
	if err := unknown.Validate("test.controller"); err == nil {
		t.Fatal("expected unknown canonical metric to be rejected")
	}

	duplicate := valid
	duplicate.Metrics = append(duplicate.Metrics, duplicate.Metrics[0])
	if err := duplicate.Validate("test.controller"); err == nil {
		t.Fatal("expected duplicate metric to be rejected")
	}
}

func TestManifestRejectsUnsafeComponentPath(t *testing.T) {
	manifest := Manifest{
		Schema:       SchemaVersion,
		ID:           "test.controller",
		Manufacturer: "Test",
		Model:        "Controller",
		DisplayName:  "Test Controller",
		Status:       StatusSynthetic,
		Capabilities: Capabilities{Telemetry: true},
		Files: ProfileFiles{
			Telemetry: "../telemetry.json",
			Alarms:    "alarms.json",
			UI:        "ui.json",
		},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestUIRejectsUndefinedAndRepeatedMetrics(t *testing.T) {
	telemetry := TelemetryFile{
		Schema:    SchemaVersion,
		ProfileID: "test.controller",
		Metrics: []MetricDefinition{
			{Key: monitor.MetricEngineRPM, DisplayName: "RPM", Kind: monitor.ValueNumber, StaleAfterSeconds: 10},
		},
	}
	undefined := UIFile{
		Schema:    SchemaVersion,
		ProfileID: "test.controller",
		Sections:  []UISection{{ID: "engine", Title: "Engine", Metrics: []monitor.MetricKey{monitor.MetricFuelLevel}}},
	}
	if err := undefined.Validate("test.controller", telemetry); err == nil {
		t.Fatal("expected undefined UI metric to be rejected")
	}

	repeated := UIFile{
		Schema:    SchemaVersion,
		ProfileID: "test.controller",
		Sections: []UISection{
			{ID: "engine", Title: "Engine", Metrics: []monitor.MetricKey{monitor.MetricEngineRPM}},
			{ID: "summary", Title: "Summary", Metrics: []monitor.MetricKey{monitor.MetricEngineRPM}},
		},
	}
	if err := repeated.Validate("test.controller", telemetry); err == nil {
		t.Fatal("expected repeated UI metric to be rejected")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
