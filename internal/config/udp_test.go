package config

import "testing"

func TestLoadAcceptsUDPTunnelDefaults(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","udpTunnels":[{"id":"udp-1","field":{"mode":"listen","bind":"127.0.0.1:16001"},"consumer":{"mode":"connect","address":"127.0.0.1:26001"}}],"tunnels":[]}`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.UDPTunnels) != 1 {
		t.Fatalf("expected one UDP tunnel, got %d", len(cfg.UDPTunnels))
	}
	u := cfg.UDPTunnels[0]
	if u.IdleTimeoutS != 60 || u.MaxSessions != 1024 || u.MaxDatagramBytes != 65507 {
		t.Fatalf("unexpected UDP defaults: %#v", u)
	}
}

func TestLoadRejectsUDPListenListen(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","udpTunnels":[{"id":"udp-bad","field":{"mode":"listen","bind":"127.0.0.1:16001"},"consumer":{"mode":"listen","bind":"127.0.0.1:26001"}}],"tunnels":[]}`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected UDP listen/listen rejection")
	}
}

func TestLoadRequiresAllowlistOnPublicUDPListener(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","security":{"requireAllowlist":true},"udpTunnels":[{"id":"udp-public","field":{"mode":"listen","bind":"0.0.0.0:16001"},"consumer":{"mode":"connect","address":"127.0.0.1:26001"}}],"tunnels":[]}`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected UDP allowlist rejection")
	}
}

func TestLoadRejectsUDPResourceLimits(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","udpTunnels":[{"id":"udp-large","maxSessions":10001,"field":{"mode":"listen","bind":"127.0.0.1:16001"},"consumer":{"mode":"connect","address":"127.0.0.1:26001"}}],"tunnels":[]}`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected maxSessions rejection")
	}
}

func TestLoadRejectsDuplicateIDAcrossStreamAndUDP(t *testing.T) {
	p := writeConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"same","field":{"mode":"listen","bind":"127.0.0.1:15001"},"consumer":{"mode":"listen","bind":"127.0.0.1:25001"}}],"udpTunnels":[{"id":"same","field":{"mode":"listen","bind":"127.0.0.1:16001"},"consumer":{"mode":"connect","address":"127.0.0.1:26001"}}]}`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected duplicate tunnel id rejection")
	}
}
