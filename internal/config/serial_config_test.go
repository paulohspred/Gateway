package config

import "testing"

func TestLoadAcceptsSerialProvider(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"bus-1","socket":"/run/rc-gateway/bus-1.sock","device":"/dev/ttyUSB0","standard":"rs485","baudRate":9600,"dataBits":8,"parity":"even","stopBits":"1","readTimeoutMilliseconds":250}],"tunnels":[]}`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.SerialProviders) != 1 || cfg.SerialProviders[0].Parity != "even" {
		t.Fatalf("unexpected serial provider: %#v", cfg.SerialProviders)
	}
}

func TestLoadRejectsDuplicateSerialSocket(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"a","socket":"/run/rc-gateway/bus.sock","device":"/dev/ttyUSB0","standard":"rs485","baudRate":9600,"dataBits":8},{"id":"b","socket":"/run/rc-gateway/bus.sock","device":"/dev/ttyUSB1","standard":"rs485","baudRate":9600,"dataBits":8}],"tunnels":[]}`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected duplicate serial socket rejection")
	}
}
