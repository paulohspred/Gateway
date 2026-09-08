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

func TestLoadUSBHIDProviderAcceptsStableSelector(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"comap-usb","socket":"/run/rc-gateway/comap.sock","vendorId":"0x1A2b","productId":"3c","serialNumber":" ABC-123 "}],"tunnels":[]}`)
	cfg, err := LoadStrict(path)
	if err != nil {
		t.Fatalf("LoadStrict: %v", err)
	}
	p := cfg.USBHIDProviders[0]
	if p.Device != "" || p.VendorID != "1a2b" || p.ProductID != "003c" || p.SerialNumber != "ABC-123" {
		t.Fatalf("selector not normalized: %#v", p)
	}
}

func TestLoadRejectsIncompleteUSBHIDSelector(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"usb","socket":"/run/rc-gateway/usb.sock","vendorId":"1234"}],"tunnels":[]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "vendorId and productId") {
		t.Fatalf("expected selector pair rejection, got %v", err)
	}
}

func TestLoadRejectsDuplicateUSBHIDAutoSelector(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"usb-a","socket":"/run/rc-gateway/a.sock","vendorId":"1234","productId":"5678","serialNumber":"S1"},{"id":"usb-b","socket":"/run/rc-gateway/b.sock","vendorId":"1234","productId":"5678","serialNumber":"S1"}],"tunnels":[]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "USB HID selector") {
		t.Fatalf("expected duplicate selector rejection, got %v", err)
	}
}

func TestLoadRejectsWildcardUSBHIDSelectorOverlappingExactUnit(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"usb-any","socket":"/run/rc-gateway/any.sock","vendorId":"1234","productId":"5678"},{"id":"usb-s1","socket":"/run/rc-gateway/s1.sock","vendorId":"1234","productId":"5678","serialNumber":"S1"}],"tunnels":[]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "overlaps selector") {
		t.Fatalf("expected wildcard selector overlap rejection, got %v", err)
	}
}

func TestLoadAllowsDistinctUSBHIDSerialSelectors(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"usb-s1","socket":"/run/rc-gateway/s1.sock","vendorId":"1234","productId":"5678","serialNumber":"S1"},{"id":"usb-s2","socket":"/run/rc-gateway/s2.sock","vendorId":"1234","productId":"5678","serialNumber":"S2"}],"tunnels":[]}`)
	if _, err := LoadStrict(path); err != nil {
		t.Fatalf("distinct serial selectors should coexist: %v", err)
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

func TestLoadRejectsWrongSocketTypeForUSBHIDProvider(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"usb","socket":"/run/rc-gateway/usb.sock","device":"/dev/hidraw0"}],"tunnels":[{"id":"bad","field":{"mode":"connect","network":"unix","address":"/run/rc-gateway/usb.sock"},"consumer":{"mode":"listen","network":"unix","bind":"/run/rc-gateway/consumer.sock"}}]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "requires network unixpacket") {
		t.Fatalf("expected provider socket type rejection, got %v", err)
	}
}

func TestLoadAcceptsUnixpacketProviderConsumer(t *testing.T) {
	path := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","usbHidProviders":[{"id":"usb","socket":"/run/rc-gateway/usb.sock","device":"/dev/hidraw0"}],"tunnels":[{"id":"packet","field":{"mode":"connect","network":"unixpacket","address":"/run/rc-gateway/usb.sock"},"consumer":{"mode":"listen","network":"unixpacket","bind":"/run/rc-gateway/consumer.sock"}}]}`)
	if _, err := LoadStrict(path); err != nil {
		t.Fatalf("expected unixpacket tunnel to validate: %v", err)
	}
}

func TestLoadMixedUnixpacketAndTCPRequiresExplicitFraming(t *testing.T) {
	without := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"mixed","field":{"mode":"connect","network":"unixpacket","address":"/run/rc-gateway/source.sock"},"consumer":{"mode":"listen","network":"tcp","bind":"127.0.0.1:25001"}}]}`)
	_, err := LoadStrict(without)
	if err == nil || !strings.Contains(err.Error(), "packetFraming=length32be") {
		t.Fatalf("expected framing requirement, got %v", err)
	}

	with := writeUSBHIDConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"mixed","packetFraming":"length32be","field":{"mode":"connect","network":"unixpacket","address":"/run/rc-gateway/source.sock"},"consumer":{"mode":"listen","network":"tcp","bind":"127.0.0.1:25001"}}]}`)
	cfg, err := LoadStrict(with)
	if err != nil {
		t.Fatalf("expected explicit framing to validate: %v", err)
	}
	if cfg.Tunnels[0].PacketFraming != "length32be" {
		t.Fatalf("unexpected framing %q", cfg.Tunnels[0].PacketFraming)
	}
}
