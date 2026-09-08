package usbhid

import "testing"

func TestValidateAcceptsHIDRawDeviceAndDefaultsReportLimit(t *testing.T) {
	cfg, err := normalizeConfig(Config{ID: "comap-usb", Socket: "/run/rc-gateway/comap.sock", Device: "/dev/hidraw0"})
	if err != nil {
		t.Fatalf("normalizeConfig: %v", err)
	}
	if cfg.MaxReportBytes != DefaultMaxReportBytes {
		t.Fatalf("expected default max report bytes %d, got %d", DefaultMaxReportBytes, cfg.MaxReportBytes)
	}
}

func TestValidateAcceptsStableSelectorWithoutDevice(t *testing.T) {
	cfg, err := normalizeConfig(Config{ID: "comap-usb", Socket: "/run/rc-gateway/comap.sock", VendorID: "0x1a2B", ProductID: "003c", SerialNumber: " ABC-123 "})
	if err != nil {
		t.Fatalf("normalizeConfig: %v", err)
	}
	if cfg.Device != "" {
		t.Fatalf("expected device to remain unresolved, got %q", cfg.Device)
	}
	if cfg.VendorID != "1a2b" || cfg.ProductID != "003c" || cfg.SerialNumber != "ABC-123" {
		t.Fatalf("selector not normalized: %#v", cfg)
	}
}

func TestValidateRejectsIncompleteStableSelector(t *testing.T) {
	for _, cfg := range []Config{
		{ID: "usb", Socket: "/run/rc-gateway/usb.sock", VendorID: "1234"},
		{ID: "usb", Socket: "/run/rc-gateway/usb.sock", ProductID: "5678"},
		{ID: "usb", Socket: "/run/rc-gateway/usb.sock", SerialNumber: "serial-only"},
	} {
		if err := Validate(cfg); err == nil {
			t.Fatalf("expected incomplete selector to be rejected: %#v", cfg)
		}
	}
}

func TestValidateRejectsUnsafeDevicePath(t *testing.T) {
	for _, path := range []string{"/tmp/hidraw0", "/dev/ttyUSB0", "/dev/hidraw", "hidraw0"} {
		if err := Validate(Config{ID: "usb", Socket: "/run/rc-gateway/usb.sock", Device: path}); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
}

func TestValidateRejectsOversizedReports(t *testing.T) {
	err := Validate(Config{ID: "usb", Socket: "/run/rc-gateway/usb.sock", Device: "/dev/hidraw1", MaxReportBytes: MaxReportBytes + 1})
	if err == nil {
		t.Fatal("expected maxReportBytes validation error")
	}
}
