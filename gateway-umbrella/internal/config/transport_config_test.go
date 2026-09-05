package config

import "testing"

func TestLoadAcceptsUnixEndpoints(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"unix","field":{"mode":"connect","network":"unix","address":"/run/field.sock"},"consumer":{"mode":"listen","network":"unix","bind":"/run/consumer.sock"}}]}`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tunnels[0].Field.Network != "unix" {
		t.Fatalf("unexpected network: %s", cfg.Tunnels[0].Field.Network)
	}
}
func TestLoadRejectsMTLSListenerWithoutCA(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"tls","field":{"mode":"listen","bind":"127.0.0.1:15001","tls":{"enabled":true,"certFile":"server.crt","keyFile":"server.key","requireClientCert":true}},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}}]}`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected mTLS listener without CA to be rejected")
	}
}
