package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeUSBHIDConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadUSBHIDProviderDefaultsAndCanonicalizes(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"comap-usb","socket":"/run/rc-gateway/x/../comap.sock","device":"/dev/hidraw0"}],"tunnels":[]}`)
	cfg, err := LoadStrict(path)
	if err != nil {
		t.Fatalf("LoadStrict: %v", err)
	}
	if len(cfg.USBHIDProviders) != 1 {
		t.Fatalf("expected one USB HID provider, got %d", len(cfg.USBHIDProviders))
	}
	p := cfg.USBHIDProviders[0]
	if p.Socket != "/run/rc-gateway/comap.sock" {
		t.Fatalf("socket not canonicalized: %q", p.Socket)
	}
	if p.MaxReportBytes != 4096 {
		t.Fatalf("expected default maxReportBytes=4096, got %d", p.MaxReportBytes)
	}
	if p.AllowWrite {
		t.Fatal("allowWrite must default to false")
	}
}

func TestLoadRejectsUnsafeUSBHIDDevicePath(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"usb","socket":"/run/rc-gateway/usb.sock","device":"/tmp/hidraw0"}],"tunnels":[]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "/dev/hidrawN") {
		t.Fatalf("expected hidraw path rejection, got %v", err)
	}
}

func TestLoadRejectsUSBHIDProviderSocketCollision(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"serial","socket":"/run/rc-gateway/provider.sock","device":"/dev/ttyUSB0","standard":"rs232","baudRate":9600,"dataBits":8}],"usbHidProviders":[{"id":"usb","socket":"/run/rc-gateway/x/../provider.sock","device":"/dev/hidraw0"}],"tunnels":[]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "provider socket") {
		t.Fatalf("expected provider socket collision, got %v", err)
	}
}

func TestLoadRejectsDuplicatePhysicalDeviceAcrossProviders(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"serial","socket":"/run/rc-gateway/serial.sock","device":"/dev/hidraw0","standard":"rs232","baudRate":9600,"dataBits":8}],"usbHidProviders":[{"id":"usb","socket":"/run/rc-gateway/usb.sock","device":"/dev/hidraw0"}],"tunnels":[]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("expected physical device collision, got %v", err)
	}
}
