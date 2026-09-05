package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type Admin struct {
	Bind string `json:"bind"`
}

type Security struct {
	RequireAllowlist    bool `json:"requireAllowlist"`
	CommandPlaneEnabled bool `json:"commandPlaneEnabled"`
}

type TLS struct {
	Enabled           bool   `json:"enabled,omitempty"`
	CAFile            string `json:"caFile,omitempty"`
	CertFile          string `json:"certFile,omitempty"`
	KeyFile           string `json:"keyFile,omitempty"`
	ServerName        string `json:"serverName,omitempty"`
	RequireClientCert bool   `json:"requireClientCert,omitempty"`
}

type Endpoint struct {
	Mode         string   `json:"mode"`
	Network      string   `json:"network,omitempty"`
	Bind         string   `json:"bind,omitempty"`
	Address      string   `json:"address,omitempty"`
	AllowedCIDRs []string `json:"allowedCidrs,omitempty"`
	DialTimeoutS int      `json:"dialTimeoutSeconds,omitempty"`
	ReconnectS   int      `json:"reconnectSeconds,omitempty"`
	KeepAliveS   int      `json:"keepAliveSeconds,omitempty"`
	TLS          TLS      `json:"tls,omitempty"`
}

type Tunnel struct {
	ID            string   `json:"id"`
	Field         Endpoint `json:"field"`
	Consumer      Endpoint `json:"consumer"`
	PairTimeoutS  int      `json:"pairTimeoutSeconds,omitempty"`
	WriteTimeoutS int      `json:"writeTimeoutSeconds,omitempty"`
	DrainTimeoutS int      `json:"drainTimeoutSeconds,omitempty"`
}

type SerialProvider struct {
	ID            string `json:"id"`
	Socket        string `json:"socket"`
	Device        string `json:"device"`
	Standard      string `json:"standard"`
	BaudRate      int    `json:"baudRate"`
	DataBits      int    `json:"dataBits"`
	Parity        string `json:"parity"`
	StopBits      string `json:"stopBits"`
	ReadTimeoutMS int    `json:"readTimeoutMilliseconds,omitempty"`
	RTS           bool   `json:"rts,omitempty"`
	DTR           bool   `json:"dtr,omitempty"`
}

type CANProvider struct {
	ID            string `json:"id"`
	Interface     string `json:"interface"`
	Socket        string `json:"socket"`
	EnableFD      bool   `json:"enableFd,omitempty"`
	ReceiveOwn    bool   `json:"receiveOwn,omitempty"`
	AllowTransmit bool   `json:"allowTransmit,omitempty"`
}

type UDPEndpoint struct {
	Mode         string   `json:"mode"`
	Bind         string   `json:"bind,omitempty"`
	Address      string   `json:"address,omitempty"`
	AllowedCIDRs []string `json:"allowedCidrs,omitempty"`
}

type UDPTunnel struct {
	ID               string      `json:"id"`
	Field            UDPEndpoint `json:"field"`
	Consumer         UDPEndpoint `json:"consumer"`
	IdleTimeoutS     int         `json:"idleTimeoutSeconds,omitempty"`
	MaxSessions      int         `json:"maxSessions,omitempty"`
	MaxDatagramBytes int         `json:"maxDatagramBytes,omitempty"`
}

type Config struct {
	Schema          int              `json:"schema"`
	NodeID          string           `json:"nodeId"`
	Admin           Admin            `json:"admin"`
	Security        Security         `json:"security"`
	SerialProviders []SerialProvider `json:"serialProviders,omitempty"`
	CANProviders    []CANProvider    `json:"canProviders,omitempty"`
	Tunnels         []Tunnel         `json:"tunnels"`
	UDPTunnels      []UDPTunnel      `json:"udpTunnels,omitempty"`
}

func Load(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Schema == 0 {
		cfg.Schema = 3
	}
	if cfg.Schema != 3 {
		return cfg, fmt.Errorf("unsupported schema %d; bridge-first configuration requires schema 3", cfg.Schema)
	}
	if strings.TrimSpace(cfg.NodeID) == "" {
		return cfg, fmt.Errorf("nodeId is required")
	}
	if cfg.Admin.Bind == "" {
		cfg.Admin.Bind = "127.0.0.1:18080"
	}
	if _, _, err := net.SplitHostPort(cfg.Admin.Bind); err != nil {
		return cfg, fmt.Errorf("admin invalid bind: %w", err)
	}
	if cfg.Security.CommandPlaneEnabled {
		return cfg, fmt.Errorf("commandPlaneEnabled is intentionally unsupported in this release")
	}
	if err := validateProviders(&cfg); err != nil {
		return cfg, err
	}

	seen := map[string]bool{}
	for i := range cfg.Tunnels {
		t := &cfg.Tunnels[i]
		t.ID = strings.TrimSpace(t.ID)
		if t.ID == "" {
			return cfg, fmt.Errorf("tunnel[%d] requires id", i)
		}
		if seen[t.ID] {
			return cfg, fmt.Errorf("duplicate tunnel id %q", t.ID)
		}
		seen[t.ID] = true
		if t.PairTimeoutS <= 0 {
			t.PairTimeoutS = 30
		}
		if t.WriteTimeoutS <= 0 {
			t.WriteTimeoutS = 30
		}
		if t.DrainTimeoutS <= 0 {
			t.DrainTimeoutS = 2
		}
		if t.PairTimeoutS > 3600 || t.WriteTimeoutS > 3600 || t.DrainTimeoutS > 300 {
			return cfg, fmt.Errorf("tunnel %s timeout exceeds safe configuration limit", t.ID)
		}
		if err := validateEndpoint(&t.Field, "tunnel "+t.ID+" field", cfg.Security.RequireAllowlist); err != nil {
			return cfg, err
		}
		if err := validateEndpoint(&t.Consumer, "tunnel "+t.ID+" consumer", cfg.Security.RequireAllowlist); err != nil {
			return cfg, err
		}
	}

	for i := range cfg.UDPTunnels {
		t := &cfg.UDPTunnels[i]
		t.ID = strings.TrimSpace(t.ID)
		if t.ID == "" {
			return cfg, fmt.Errorf("udpTunnel[%d] requires id", i)
		}
		if seen[t.ID] {
			return cfg, fmt.Errorf("duplicate tunnel id %q", t.ID)
		}
		seen[t.ID] = true
		if t.IdleTimeoutS <= 0 {
			t.IdleTimeoutS = 60
		}
		if t.MaxSessions <= 0 {
			t.MaxSessions = 1024
		}
		if t.MaxDatagramBytes <= 0 {
			t.MaxDatagramBytes = 65507
		}
		if t.IdleTimeoutS > 3600 {
			return cfg, fmt.Errorf("udp tunnel %s idle timeout exceeds safe limit", t.ID)
		}
		if t.MaxSessions > 10000 {
			return cfg, fmt.Errorf("udp tunnel %s maxSessions exceeds safe limit", t.ID)
		}
		if t.MaxDatagramBytes > 65507 {
			return cfg, fmt.Errorf("udp tunnel %s maxDatagramBytes exceeds UDP payload limit", t.ID)
		}
		if err := validateUDPTunnel(t, cfg.Security.RequireAllowlist); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

func validateProviders(cfg *Config) error {
	ids := map[string]string{}
	sockets := map[string]string{}
	claim := func(kind, id, socket string) error {
		if prev, exists := ids[id]; exists {
			return fmt.Errorf("provider id %q is already used by %s", id, prev)
		}
		ids[id] = kind
		if prev, exists := sockets[socket]; exists {
			return fmt.Errorf("provider socket %q is already used by %s", socket, prev)
		}
		sockets[socket] = kind + " " + id
		return nil
	}

	for i := range cfg.SerialProviders {
		p := &cfg.SerialProviders[i]
		p.ID = strings.TrimSpace(p.ID)
		p.Socket = strings.TrimSpace(p.Socket)
		p.Standard = strings.ToLower(strings.TrimSpace(p.Standard))
		p.Parity = strings.ToLower(strings.TrimSpace(p.Parity))
		p.StopBits = strings.TrimSpace(p.StopBits)
		if p.ID == "" {
			return fmt.Errorf("serialProvider[%d] requires id", i)
		}
		if !filepath.IsAbs(p.Socket) {
			return fmt.Errorf("serialProvider %s requires absolute socket", p.ID)
		}
		if err := claim("serialProvider", p.ID, p.Socket); err != nil {
			return err
		}
		if strings.TrimSpace(p.Device) == "" {
			return fmt.Errorf("serialProvider %s device is required", p.ID)
		}
		if p.Standard != "rs232" && p.Standard != "rs422" && p.Standard != "rs485" {
			return fmt.Errorf("serialProvider %s invalid standard %q", p.ID, p.Standard)
		}
		if p.BaudRate <= 0 || p.DataBits < 5 || p.DataBits > 8 {
			return fmt.Errorf("serialProvider %s invalid baudRate/dataBits", p.ID)
		}
		if p.Parity == "" {
			p.Parity = "none"
		}
		if p.Parity != "none" && p.Parity != "odd" && p.Parity != "even" && p.Parity != "mark" && p.Parity != "space" {
			return fmt.Errorf("serialProvider %s invalid parity", p.ID)
		}
		if p.StopBits == "" {
			p.StopBits = "1"
		}
		if p.StopBits != "1" && p.StopBits != "1.5" && p.StopBits != "2" {
			return fmt.Errorf("serialProvider %s invalid stopBits", p.ID)
		}
		if p.ReadTimeoutMS < 0 || p.ReadTimeoutMS > 60000 {
			return fmt.Errorf("serialProvider %s invalid readTimeoutMilliseconds", p.ID)
		}
	}

	for i := range cfg.CANProviders {
		p := &cfg.CANProviders[i]
		p.ID = strings.TrimSpace(p.ID)
		p.Interface = strings.TrimSpace(p.Interface)
		p.Socket = strings.TrimSpace(p.Socket)
		if p.ID == "" {
			return fmt.Errorf("canProvider[%d] requires id", i)
		}
		if p.Interface == "" {
			return fmt.Errorf("canProvider %s interface is required", p.ID)
		}
		if !filepath.IsAbs(p.Socket) {
			return fmt.Errorf("canProvider %s requires absolute socket", p.ID)
		}
		if err := claim("canProvider", p.ID, p.Socket); err != nil {
			return err
		}
	}
	return nil
}

func validateEndpoint(ep *Endpoint, label string, requireAllowlist bool) error {
	ep.Mode = strings.TrimSpace(ep.Mode)
	ep.Network = strings.TrimSpace(ep.Network)
	if ep.Network == "" {
		ep.Network = "tcp"
	}
	switch ep.Network {
	case "tcp":
		return validateTCPEndpoint(ep, label, requireAllowlist)
	case "unix":
		if ep.TLS.Enabled {
			return fmt.Errorf("%s TLS is not valid for unix endpoint", label)
		}
		if len(ep.AllowedCIDRs) > 0 {
			return fmt.Errorf("%s allowedCidrs is not valid for unix endpoint", label)
		}
		path := ep.Address
		if ep.Mode == "listen" {
			path = ep.Bind
		}
		if ep.Mode != "listen" && ep.Mode != "connect" {
			return fmt.Errorf("%s mode must be listen or connect", label)
		}
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%s unix path must be absolute", label)
		}
		return nil
	default:
		return fmt.Errorf("%s unsupported network %q", label, ep.Network)
	}
}

func validateTCPEndpoint(ep *Endpoint, label string, requireAllowlist bool) error {
	if ep.KeepAliveS <= 0 {
		ep.KeepAliveS = 30
	}
	switch ep.Mode {
	case "listen":
		if strings.TrimSpace(ep.Bind) == "" {
			return fmt.Errorf("%s requires bind in listen mode", label)
		}
		if _, _, err := net.SplitHostPort(ep.Bind); err != nil {
			return fmt.Errorf("%s invalid bind: %w", label, err)
		}
		for _, cidr := range ep.AllowedCIDRs {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return fmt.Errorf("%s invalid allowedCidrs entry %q: %w", label, cidr, err)
			}
		}
		if requireAllowlist && len(ep.AllowedCIDRs) == 0 && !isLoopbackBind(ep.Bind) {
			return fmt.Errorf("%s requires allowedCidrs by security policy", label)
		}
		if ep.TLS.Enabled && (strings.TrimSpace(ep.TLS.CertFile) == "" || strings.TrimSpace(ep.TLS.KeyFile) == "") {
			return fmt.Errorf("%s TLS listener requires certFile/keyFile", label)
		}
		if ep.TLS.RequireClientCert && strings.TrimSpace(ep.TLS.CAFile) == "" {
			return fmt.Errorf("%s mTLS listener requires caFile", label)
		}
	case "connect":
		if strings.TrimSpace(ep.Address) == "" {
			return fmt.Errorf("%s requires address in connect mode", label)
		}
		if _, _, err := net.SplitHostPort(ep.Address); err != nil {
			return fmt.Errorf("%s invalid address: %w", label, err)
		}
		if ep.DialTimeoutS <= 0 {
			ep.DialTimeoutS = 10
		}
		if ep.ReconnectS <= 0 {
			ep.ReconnectS = 5
		}
		if (ep.TLS.CertFile == "") != (ep.TLS.KeyFile == "") {
			return fmt.Errorf("%s TLS certFile/keyFile must be configured together", label)
		}
	default:
		return fmt.Errorf("%s mode must be listen or connect", label)
	}
	return nil
}

func validateUDPTunnel(t *UDPTunnel, requireAllowlist bool) error {
	fieldMode := strings.TrimSpace(t.Field.Mode)
	consumerMode := strings.TrimSpace(t.Consumer.Mode)
	t.Field.Mode = fieldMode
	t.Consumer.Mode = consumerMode
	if !((fieldMode == "listen" && consumerMode == "connect") || (fieldMode == "connect" && consumerMode == "listen")) {
		return fmt.Errorf("udp tunnel %s requires exactly one listen endpoint and one connect endpoint", t.ID)
	}
	if err := validateUDPEndpoint(&t.Field, "udp tunnel "+t.ID+" field", requireAllowlist); err != nil {
		return err
	}
	if err := validateUDPEndpoint(&t.Consumer, "udp tunnel "+t.ID+" consumer", requireAllowlist); err != nil {
		return err
	}
	return nil
}

func validateUDPEndpoint(ep *UDPEndpoint, label string, requireAllowlist bool) error {
	switch ep.Mode {
	case "listen":
		if strings.TrimSpace(ep.Bind) == "" {
			return fmt.Errorf("%s requires bind in listen mode", label)
		}
		if _, _, err := net.SplitHostPort(ep.Bind); err != nil {
			return fmt.Errorf("%s invalid bind: %w", label, err)
		}
		for _, cidr := range ep.AllowedCIDRs {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return fmt.Errorf("%s invalid allowedCidrs entry %q: %w", label, cidr, err)
			}
		}
		if requireAllowlist && len(ep.AllowedCIDRs) == 0 && !isLoopbackBind(ep.Bind) {
			return fmt.Errorf("%s requires allowedCidrs by security policy", label)
		}
	case "connect":
		if strings.TrimSpace(ep.Address) == "" {
			return fmt.Errorf("%s requires address in connect mode", label)
		}
		if _, _, err := net.SplitHostPort(ep.Address); err != nil {
			return fmt.Errorf("%s invalid address: %w", label, err)
		}
		if len(ep.AllowedCIDRs) > 0 {
			return fmt.Errorf("%s allowedCidrs only applies to listen mode", label)
		}
	default:
		return fmt.Errorf("%s mode must be listen or connect", label)
	}
	return nil
}

func isLoopbackBind(bind string) bool {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
