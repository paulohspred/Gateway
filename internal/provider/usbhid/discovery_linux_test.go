//go:build linux

package usbhid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHIDUeventNormalizesIdentity(t *testing.T) {
	identity, err := parseHIDUevent("HID_ID=0003:00001A2B:0000003C\nHID_NAME=ComAp Controller\nHID_UNIQ=ABC-123\n")
	if err != nil {
		t.Fatalf("parseHIDUevent: %v", err)
	}
	if identity.VendorID != "1a2b" || identity.ProductID != "003c" || identity.SerialNumber != "ABC-123" || identity.Name != "ComAp Controller" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestResolveHIDRawDeviceUsesVIDPIDAndSerial(t *testing.T) {
	classRoot, deviceRoot := installFakeHIDRoots(t)
	writeFakeHID(t, classRoot, "hidraw0", "1234", "5678", "SERIAL-A", "Controller A")
	writeFakeHID(t, classRoot, "hidraw1", "1234", "5678", "SERIAL-B", "Controller B")

	device, identity, err := resolveHIDRawDevice(Config{
		ID:           "usb",
		Socket:       "/run/rc-gateway/usb.sock",
		VendorID:     "1234",
		ProductID:    "5678",
		SerialNumber: "SERIAL-B",
	})
	if err != nil {
		t.Fatalf("resolveHIDRawDevice: %v", err)
	}
	if device != filepath.Join(deviceRoot, "hidraw1") {
		t.Fatalf("unexpected resolved device %q", device)
	}
	if identity.Name != "Controller B" {
		t.Fatalf("unexpected identity %#v", identity)
	}
}

func TestResolveHIDRawDeviceRejectsAmbiguousSelector(t *testing.T) {
	classRoot, _ := installFakeHIDRoots(t)
	writeFakeHID(t, classRoot, "hidraw0", "1234", "5678", "SERIAL-A", "Controller A")
	writeFakeHID(t, classRoot, "hidraw1", "1234", "5678", "SERIAL-B", "Controller B")

	_, _, err := resolveHIDRawDevice(Config{ID: "usb", Socket: "/run/rc-gateway/usb.sock", VendorID: "1234", ProductID: "5678"})
	if err == nil || !strings.Contains(err.Error(), "found 2") {
		t.Fatalf("expected ambiguous selector error, got %v", err)
	}
}

func TestResolveExplicitDeviceVerifiesConfiguredIdentity(t *testing.T) {
	classRoot, _ := installFakeHIDRoots(t)
	writeFakeHID(t, classRoot, "hidraw0", "1234", "5678", "SERIAL-A", "Controller A")

	_, _, err := resolveHIDRawDevice(Config{
		ID:           "usb",
		Socket:       "/run/rc-gateway/usb.sock",
		Device:       "/dev/hidraw0",
		VendorID:     "9999",
		ProductID:    "5678",
		SerialNumber: "SERIAL-A",
	})
	if err == nil || !strings.Contains(err.Error(), "does not match selector") {
		t.Fatalf("expected selector mismatch, got %v", err)
	}
}

func installFakeHIDRoots(t *testing.T) (string, string) {
	t.Helper()
	oldClassRoot := hidrawClassRoot
	oldDeviceRoot := hidrawDeviceRoot
	base := t.TempDir()
	hidrawClassRoot = filepath.Join(base, "sys", "class", "hidraw")
	hidrawDeviceRoot = filepath.Join(base, "dev")
	if err := os.MkdirAll(hidrawClassRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(hidrawDeviceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		hidrawClassRoot = oldClassRoot
		hidrawDeviceRoot = oldDeviceRoot
	})
	return hidrawClassRoot, hidrawDeviceRoot
}

func writeFakeHID(t *testing.T, classRoot, name, vendorID, productID, serial, productName string) {
	t.Helper()
	dir := filepath.Join(classRoot, name, "device")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "HID_ID=0003:0000" + strings.ToUpper(vendorID) + ":0000" + strings.ToUpper(productID) + "\n" +
		"HID_NAME=" + productName + "\n" +
		"HID_UNIQ=" + serial + "\n"
	if err := os.WriteFile(filepath.Join(dir, "uevent"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
