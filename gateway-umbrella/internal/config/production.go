package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type bindClaim struct {
	label string
	bind  string
}

// LoadStrict is the production configuration entrypoint. It rejects unknown
// JSON fields and trailing JSON values before applying the normal schema
// defaults/validation, then checks resource conflicts that would otherwise
// only appear when the daemon starts opening sockets/devices.
func LoadStrict(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var probe Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&probe); err != nil {
		return Config{}, fmt.Errorf("invalid configuration JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("invalid configuration JSON: trailing JSON value")
		}
		return Config{}, fmt.Errorf("invalid configuration JSON after first value: %w", err)
	}

	cfg, err := Load(path)
	if err != nil {
		return Config{}, err
	}
	if err := validateProductionConflicts(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateProductionConflicts(cfg *Config) error {
	seenIDs := map[string]string{}
	claimID := func(kind, id string) error {
		id = strings.TrimSpace(id)
		if prev, exists := seenIDs[id]; exists {
			return fmt.Errorf("resource id %q is already used by %s", id, prev)
		}
		seenIDs[id] = kind
		return nil
	}

	serialDevices := map[string]string{}
	providerSockets := map[string]string{}
	unixListeners := map[string]string{}
	for _, p := range cfg.SerialProviders {
		if err := claimID("serialProvider "+p.ID, p.ID); err != nil {
			return err
		}
		device := filepath.Clean(strings.TrimSpace(p.Device))
		if !filepath.IsAbs(device) {
			return fmt.Errorf("serialProvider %s requires absolute device path", p.ID)
		}
		if prev, exists := serialDevices[device]; exists {
			return fmt.Errorf("serial device %q is already used by %s", device, prev)
		}
		serialDevices[device] = "serialProvider " + p.ID
		socket := filepath.Clean(p.Socket)
		providerSockets[socket] = "serialProvider " + p.ID
		unixListeners[socket] = "serialProvider " + p.ID
	}
	for _, p := range cfg.CANProviders {
		if err := claimID("canProvider "+p.ID, p.ID); err != nil {
			return err
		}
		socket := filepath.Clean(p.Socket)
		providerSockets[socket] = "canProvider " + p.ID
		unixListeners[socket] = "canProvider " + p.ID
	}
	for _, t := range cfg.Tunnels {
		if err := claimID("tunnel "+t.ID, t.ID); err != nil {
			return err
		}
	}
	for _, t := range cfg.UDPTunnels {
		if err := claimID("udpTunnel "+t.ID, t.ID); err != nil {
			return err
		}
	}

	tcpClaims := []bindClaim{{label: "admin", bind: cfg.Admin.Bind}}
	udpClaims := []bindClaim{}
	providerConsumers := map[string]string{}

	claimTCP := func(label, bind string) error {
		for _, prev := range tcpClaims {
			if bindsConflict(prev.bind, bind) {
				return fmt.Errorf("TCP bind %q for %s conflicts with %s at %q", bind, label, prev.label, prev.bind)
			}
		}
		tcpClaims = append(tcpClaims, bindClaim{label: label, bind: bind})
		return nil
	}
	claimUDP := func(label, bind string) error {
		for _, prev := range udpClaims {
			if bindsConflict(prev.bind, bind) {
				return fmt.Errorf("UDP bind %q for %s conflicts with %s at %q", bind, label, prev.label, prev.bind)
			}
		}
		udpClaims = append(udpClaims, bindClaim{label: label, bind: bind})
		return nil
	}
	claimUnixListener := func(label, path string) error {
		path = filepath.Clean(path)
		if prev, exists := unixListeners[path]; exists {
			return fmt.Errorf("unix listen path %q for %s conflicts with %s", path, label, prev)
		}
		unixListeners[path] = label
		return nil
	}
	observeStreamEndpoint := func(label string, ep Endpoint) error {
		network := strings.TrimSpace(ep.Network)
		if network == "" {
			network = "tcp"
		}
		switch network {
		case "tcp":
			if ep.Mode == "listen" {
				return claimTCP(label, ep.Bind)
			}
		case "unix":
			if ep.Mode == "listen" {
				return claimUnixListener(label, ep.Bind)
			}
			path := filepath.Clean(ep.Address)
			if provider, exists := providerSockets[path]; exists {
				if prev, used := providerConsumers[path]; used {
					return fmt.Errorf("provider socket %q (%s) is consumed by both %s and %s", path, provider, prev, label)
				}
				providerConsumers[path] = label
			}
		}
		return nil
	}

	for _, t := range cfg.Tunnels {
		if err := observeStreamEndpoint("tunnel "+t.ID+" field", t.Field); err != nil {
			return err
		}
		if err := observeStreamEndpoint("tunnel "+t.ID+" consumer", t.Consumer); err != nil {
			return err
		}
	}
	for _, t := range cfg.UDPTunnels {
		if t.Field.Mode == "listen" {
			if err := claimUDP("udp tunnel "+t.ID+" field", t.Field.Bind); err != nil {
				return err
			}
		}
		if t.Consumer.Mode == "listen" {
			if err := claimUDP("udp tunnel "+t.ID+" consumer", t.Consumer.Bind); err != nil {
				return err
			}
		}
	}
	return nil
}

func bindsConflict(a, b string) bool {
	ha, pa, errA := net.SplitHostPort(a)
	hb, pb, errB := net.SplitHostPort(b)
	if errA != nil || errB != nil || pa != pb {
		return false
	}
	ha = canonicalBindHost(ha)
	hb = canonicalBindHost(hb)
	return ha == "*" || hb == "*" || ha == hb
}

func canonicalBindHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "*"
	}
	if host == "localhost" {
		return "loopback"
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsUnspecified() {
			return "*"
		}
		if ip.IsLoopback() {
			return "loopback"
		}
		return ip.String()
	}
	return host
}
