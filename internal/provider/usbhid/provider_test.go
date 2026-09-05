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
