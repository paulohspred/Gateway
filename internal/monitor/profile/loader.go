package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func LoadDir(dir string) (Bundle, error) {
	manifestPath := filepath.Join(dir, "manifest.json")
	var manifest Manifest
	if err := decodeStrictFile(manifestPath, &manifest); err != nil {
		return Bundle{}, fmt.Errorf("load manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Bundle{}, fmt.Errorf("validate manifest: %w", err)
	}

	var telemetry TelemetryFile
	if err := decodeStrictFile(filepath.Join(dir, manifest.Files.Telemetry), &telemetry); err != nil {
		return Bundle{}, fmt.Errorf("load telemetry: %w", err)
	}
	var alarms AlarmsFile
	if err := decodeStrictFile(filepath.Join(dir, manifest.Files.Alarms), &alarms); err != nil {
		return Bundle{}, fmt.Errorf("load alarms: %w", err)
	}
	var ui UIFile
	if err := decodeStrictFile(filepath.Join(dir, manifest.Files.UI), &ui); err != nil {
		return Bundle{}, fmt.Errorf("load ui: %w", err)
	}

	bundle := Bundle{
		Manifest:  manifest,
		Telemetry: telemetry,
		Alarms:    alarms,
		UI:        ui,
	}
	if err := bundle.Validate(); err != nil {
		return Bundle{}, fmt.Errorf("validate profile %q: %w", manifest.ID, err)
	}
	return bundle, nil
}

func decodeStrictFile(path string, dst any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}
