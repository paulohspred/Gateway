package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsUnknownJSONField(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","unexpected":true,"tunnels":[]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected strict unknown-field rejection, got %v", err)
	}
}

func TestLoadRejectsTrailingJSONValue(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[]} {"extra":true}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "trailing JSON value") {
		t.Fatalf("expected trailing JSON rejection, got %v", err)
	}
}

func TestLoadRejectsAdminAndWildcardTCPConflict(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","admin":{"bind":"127.0.0.1:18080"},"tunnels":[{"id":"bad","field":{"mode":"listen","bind":"0.0.0.0:18080"},"consumer":{"mode":"listen","bind":"127.0.0.1:28080"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected TCP bind conflict, got %v", err)
	}
}

func TestLoadRejectsDuplicateTCPListenerPortWithWildcard(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"one","field":{"mode":"listen","bind":"0.0.0.0:15001"},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}},{"id":"two","field":{"mode":"listen","bind":"127.0.0.1:15001"},"consumer":{"mode":"listen","bind":"127.0.0.1:25002"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "TCP bind") {
		t.Fatalf("expected duplicate TCP listener rejection, got %v", err)
	}
}

func TestLoadRejectsDuplicateUDPListener(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[],"udpTunnels":[{"id":"u1","field":{"mode":"listen","bind":"0.0.0.0:16001"},"consumer":{"mode":"connect","address":"127.0.0.1:26001"}},{"id":"u2","field":{"mode":"listen","bind":"127.0.0.1:16001"},"consumer":{"mode":"connect","address":"127.0.0.1:26002"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "UDP bind") {
		t.Fatalf("expected duplicate UDP listener rejection, got %v", err)
	}
}

func TestLoadRejectsUnixListenerOnProviderSocket(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"bus","socket":"/run/rc-gateway/bus.sock","device":"/dev/ttyUSB0","standard":"rs485","baudRate":9600,"dataBits":8}],"tunnels":[{"id":"bad","field":{"mode":"listen","network":"unix","bind":"/run/rc-gateway/bus.sock"},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "unix listen path") {
		t.Fatalf("expected unix/provider socket conflict, got %v", err)
	}
}

func TestLoadRejectsMultipleConsumersForProviderSocket(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"bus","socket":"/run/rc-gateway/bus.sock","device":"/dev/ttyUSB0","standard":"rs485","baudRate":9600,"dataBits":8}],"tunnels":[{"id":"one","field":{"mode":"connect","network":"unix","address":"/run/rc-gateway/bus.sock"},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}},{"id":"two","field":{"mode":"connect","network":"unix","address":"/run/rc-gateway/bus.sock"},"consumer":{"mode":"listen","bind":"127.0.0.1:25002"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "consumed by both") {
		t.Fatalf("expected provider single-consumer rejection, got %v", err)
	}
}

func TestLoadRejectsDuplicateSerialDevice(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"a","socket":"/run/rc-gateway/a.sock","device":"/dev/ttyUSB0","standard":"rs485","baudRate":9600,"dataBits":8},{"id":"b","socket":"/run/rc-gateway/b.sock","device":"/dev/ttyUSB0","standard":"rs485","baudRate":9600,"dataBits":8}],"tunnels":[]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "serial device") {
		t.Fatalf("expected duplicate serial device rejection, got %v", err)
	}
}

func TestLoadRejectsResourceIDCollisionAcrossKinds(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"same","socket":"/run/rc-gateway/a.sock","device":"/dev/ttyUSB0","standard":"rs485","baudRate":9600,"dataBits":8}],"tunnels":[{"id":"same","field":{"mode":"listen","bind":"127.0.0.1:15001"},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "resource id") {
		t.Fatalf("expected global resource ID rejection, got %v", err)
	}
}

func TestLoadAllowsTCPAndUDPOnSamePort(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"tcp","field":{"mode":"listen","bind":"127.0.0.1:15001"},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}}],"udpTunnels":[{"id":"udp","field":{"mode":"listen","bind":"127.0.0.1:15001"},"consumer":{"mode":"connect","address":"127.0.0.1:26001"}}]}`)
	if _, err := LoadStrict(p); err != nil {
		t.Fatalf("TCP and UDP may share a numeric port: %v", err)
	}
}
