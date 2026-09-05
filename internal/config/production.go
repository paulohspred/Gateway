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
// JSON fields and trailing JSON values before applying schema defaults and
// validation to the same byte snapshot, then checks resource conflicts that
// would otherwise only appear when the daemon starts opening resources.
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

	cfg, err := loadRaw(raw)
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
	seenMetricTokens := map[string]string{}
	claimID := func(kind, id string) error {
		id = strings.TrimSpace(id)
		if prev, exists := seenIDs[id]; exists {
			return fmt.Errorf("resource id %q is already used by %s", id, prev)
		}
		seenIDs[id] = kind
		token := metricToken(id)
		if prev, exists := seenMetricTokens[token]; exists {
			return fmt.Errorf("resource id %q collides with %q after metrics sanitization", id, prev)
		}
		seenMetricTokens[token] = id
		return nil
	}

	physicalDevices := map[string]string{}
	usbSelectors := map[string]string{}
	providerSockets := map[string]string{}
	providerSocketNetworks := map[string]string{}
	unixListeners := map[string]string{}
	claimProviderSocket := func(label, socket, network string) error {
		socket = filepath.Clean(socket)
		if prev, exists := providerSockets[socket]; exists {
			return fmt.Errorf("provider socket %q for %s conflicts with %s", socket, label, prev)
		}
		providerSockets[socket] = label
		providerSocketNetworks[socket] = network
		unixListeners[socket] = label
		return nil
	}
	claimPhysicalDevice := func(label, kind, device string) error {
		device = filepath.Clean(strings.TrimSpace(device))
		if !filepath.IsAbs(device) {
			return fmt.Errorf("%s requires absolute device path", label)
		}
		if prev, exists := physicalDevices[device]; exists {
			return fmt.Errorf("%s device %q is already used by %s", kind, device, prev)
		}
		physicalDevices[device] = label
		return nil
	}

	for _, p := range cfg.SerialProviders {
		label := "serialProvider " + p.ID
		if err := claimID(label, p.ID); err != nil {
			return err
		}
		if err := claimPhysicalDevice(label, "serial", p.Device); err != nil {
			return err
		}
		if err := claimProviderSocket(label, p.Socket, "unix"); err != nil {
			return err
		}
	}
	for _, p := range cfg.USBHIDProviders {
		label := "usbHidProvider " + p.ID
		if err := claimID(label, p.ID); err != nil {
			return err
		}
		if p.Device != "" {
			if err := claimPhysicalDevice(label, "USB HID", p.Device); err != nil {
				return err
			}
		} else {
			selector := p.VendorID + ":" + p.ProductID + ":" + p.SerialNumber
			if prev, exists := usbSelectors[selector]; exists {
				return fmt.Errorf("USB HID selector %q for %s is already used by %s", selector, label, prev)
			}
			usbSelectors[selector] = label
		}
		if err := claimProviderSocket(label, p.Socket, "unixpacket"); err != nil {
			return err
		}
	}
	for _, p := range cfg.CANProviders {
		label := "canProvider " + p.ID
		if err := claimID(label, p.ID); err != nil {
			return err
		}
		if err := claimProviderSocket(label, p.Socket, "unixpacket"); err != nil {
			return err
		}
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
		case "unix", "unixpacket":
			if ep.Mode == "listen" {
				return claimUnixListener(label, ep.Bind)
			}
			path := filepath.Clean(ep.Address)
			if provider, exists := providerSockets[path]; exists {
				expectedNetwork := providerSocketNetworks[path]
				if expectedNetwork != network {
					return fmt.Errorf("provider socket %q (%s) requires network %s; %s uses %s", path, provider, expectedNetwork, label, network)
				}
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

func metricToken(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == ':' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
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
