package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsRemoteAdminBind(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","admin":{"bind":"0.0.0.0:18080"},"tunnels":[]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "admin bind must be loopback") {
		t.Fatalf("expected loopback-only admin rejection, got %v", err)
	}
}

func TestLoadRejectsTLSOptionsWhenDisabled(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"tls-misconfigured","field":{"mode":"listen","bind":"127.0.0.1:15001","tls":{"enabled":false,"requireClientCert":true,"caFile":"/tmp/ca.pem"}},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "TLS options require tls.enabled=true") {
		t.Fatalf("expected disabled-TLS option rejection, got %v", err)
	}
}

func TestLoadRejectsPublicTCPWithoutAllowlistEvenWhenLegacyFlagFalse(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","security":{"requireAllowlist":false},"tunnels":[{"id":"public","field":{"mode":"listen","bind":"0.0.0.0:15001"},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "public listener requires allowedCidrs") {
		t.Fatalf("expected fail-closed public listener rejection, got %v", err)
	}
}

func TestLoadRejectsCanonicalProviderSocketCollision(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","serialProviders":[{"id":"serial","socket":"/run/rc-gateway/bus.sock","device":"/dev/ttyUSB0","standard":"rs485","baudRate":9600,"dataBits":8}],"canProviders":[{"id":"can","interface":"can0","socket":"/run/rc-gateway/x/../bus.sock"}],"tunnels":[]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "provider socket") {
		t.Fatalf("expected canonical provider socket collision, got %v", err)
	}
}

func TestLoadRejectsMetricIdentityCollision(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"a-b","field":{"mode":"listen","bind":"127.0.0.1:15001"},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}},{"id":"a_b","field":{"mode":"listen","bind":"127.0.0.1:15002"},"consumer":{"mode":"listen","bind":"127.0.0.1:25002"}}]}`)
	_, err := LoadStrict(p)
	if err == nil || !strings.Contains(err.Error(), "metrics sanitization") {
		t.Fatalf("expected metrics identity collision rejection, got %v", err)
	}
}
