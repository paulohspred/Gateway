package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Admin struct {
	Bind string `json:"bind"`
}

type Security struct {
	RequireAllowlist    bool `json:"requireAllowlist"`
	CommandPlaneEnabled bool `json:"commandPlaneEnabled"`
}

type Limits struct {
	MaxActivePairs int `json:"maxActivePairs,omitempty"`
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
	ID                 string   `json:"id"`
	Field              Endpoint `json:"field"`
	Consumer           Endpoint `json:"consumer"`
	PacketFraming      string   `json:"packetFraming,omitempty"`
	PairTimeoutS       int      `json:"pairTimeoutSeconds,omitempty"`
	WriteTimeoutS      int      `json:"writeTimeoutSeconds,omitempty"`
	DrainTimeoutS      int      `json:"drainTimeoutSeconds,omitempty"`
	MaxConcurrentPairs int      `json:"maxConcurrentPairs,omitempty"`
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

type USBHIDProvider struct {
	ID             string `json:"id"`
	Socket         string `json:"socket"`
	Device         string `json:"device,omitempty"`
	VendorID       string `json:"vendorId,omitempty"`
	ProductID      string `json:"productId,omitempty"`
	SerialNumber   string `json:"serialNumber,omitempty"`
	MaxReportBytes int    `json:"maxReportBytes,omitempty"`
	AllowWrite     bool   `json:"allowWrite,omitempty"`
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
	Limits          Limits           `json:"limits,omitempty"`
	SerialProviders []SerialProvider `json:"serialProviders,omitempty"`
	USBHIDProviders []USBHIDProvider `json:"usbHidProviders,omitempty"`
	CANProviders    []CANProvider    `json:"canProviders,omitempty"`
	Tunnels         []Tunnel         `json:"tunnels"`
	UDPTunnels      []UDPTunnel      `json:"udpTunnels,omitempty"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return loadRaw(raw)
}

func loadRaw(raw []byte) (Config, error) {
	var cfg Config
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
	if !isLoopbackBind(cfg.Admin.Bind) {
		return cfg, fmt.Errorf("admin bind must be loopback in this release; use VPN/SSH forwarding for remote management")
	}
	if cfg.Security.CommandPlaneEnabled {
		return cfg, fmt.Errorf("commandPlaneEnabled is intentionally unsupported in this release")
	}
	if cfg.Limits.MaxActivePairs <= 0 {
		cfg.Limits.MaxActivePairs = 1024
	}
	if cfg.Limits.MaxActivePairs > 10000 {
		return cfg, fmt.Errorf("limits.maxActivePairs exceeds safe limit 10000")
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
		if t.MaxConcurrentPairs <= 0 {
			t.MaxConcurrentPairs = 1
		}
		if t.PairTimeoutS > 3600 || t.WriteTimeoutS > 3600 || t.DrainTimeoutS > 300 {
			return cfg, fmt.Errorf("tunnel %s timeout exceeds safe configuration limit", t.ID)
		}
		if t.MaxConcurrentPairs > 1024 {
			return cfg, fmt.Errorf("tunnel %s maxConcurrentPairs exceeds safe limit 1024", t.ID)
		}
		if t.MaxConcurrentPairs > cfg.Limits.MaxActivePairs {
			return cfg, fmt.Errorf("tunnel %s maxConcurrentPairs exceeds limits.maxActivePairs", t.ID)
		}
		if err := validateEndpoint(&t.Field, "tunnel "+t.ID+" field", cfg.Security.RequireAllowlist); err != nil {
			return cfg, err
		}
		if err := validateEndpoint(&t.Consumer, "tunnel "+t.ID+" consumer", cfg.Security.RequireAllowlist); err != nil {
			return cfg, err
		}
		if t.MaxConcurrentPairs > 1 && !hasTriggeredPairing(t.Field.Mode, t.Consumer.Mode) {
			return cfg, fmt.Errorf("tunnel %s maxConcurrentPairs>1 requires exactly one listen endpoint and one connect endpoint", t.ID)
		}
		if err := validatePacketFraming(t); err != nil {
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

func hasTriggeredPairing(fieldMode, consumerMode string) bool {
	return (fieldMode == "listen" && consumerMode == "connect") || (fieldMode == "connect" && consumerMode == "listen")
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
		p.Device = strings.TrimSpace(p.Device)
		p.Standard = strings.ToLower(strings.TrimSpace(p.Standard))
		p.Parity = strings.ToLower(strings.TrimSpace(p.Parity))
		p.StopBits = strings.TrimSpace(p.StopBits)
		if p.ID == "" {
			return fmt.Errorf("serialProvider[%d] requires id", i)
		}
		if !filepath.IsAbs(p.Socket) {
			return fmt.Errorf("serialProvider %s requires absolute socket", p.ID)
		}
		p.Socket = filepath.Clean(p.Socket)
		if p.Device == "" {
			return fmt.Errorf("serialProvider %s device is required", p.ID)
		}
		if !filepath.IsAbs(p.Device) {
			return fmt.Errorf("serialProvider %s requires absolute device path", p.ID)
		}
		p.Device = filepath.Clean(p.Device)
		if err := claim("serialProvider", p.ID, p.Socket); err != nil {
			return err
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

	for i := range cfg.USBHIDProviders {
		p := &cfg.USBHIDProviders[i]
		p.ID = strings.TrimSpace(p.ID)
		p.Socket = strings.TrimSpace(p.Socket)
		p.Device = strings.TrimSpace(p.Device)
		p.SerialNumber = strings.TrimSpace(p.SerialNumber)
		if p.ID == "" {
			return fmt.Errorf("usbHidProvider[%d] requires id", i)
		}
		if !filepath.IsAbs(p.Socket) {
			return fmt.Errorf("usbHidProvider %s requires absolute socket", p.ID)
		}
		p.Socket = filepath.Clean(p.Socket)
		var err error
		if p.VendorID != "" {
			p.VendorID, err = normalizeUSBID(p.VendorID)
			if err != nil {
				return fmt.Errorf("usbHidProvider %s invalid vendorId: %w", p.ID, err)
			}
		}
		if p.ProductID != "" {
			p.ProductID, err = normalizeUSBID(p.ProductID)
			if err != nil {
				return fmt.Errorf("usbHidProvider %s invalid productId: %w", p.ID, err)
			}
		}
		if (p.VendorID == "") != (p.ProductID == "") {
			return fmt.Errorf("usbHidProvider %s vendorId and productId must be configured together", p.ID)
		}
		if p.SerialNumber != "" && p.VendorID == "" {
			return fmt.Errorf("usbHidProvider %s serialNumber requires vendorId/productId", p.ID)
		}
		if p.Device != "" {
			p.Device = filepath.Clean(p.Device)
			if !isHIDRawDevicePath(p.Device) {
				return fmt.Errorf("usbHidProvider %s device must be /dev/hidrawN", p.ID)
			}
		} else if p.VendorID == "" {
			return fmt.Errorf("usbHidProvider %s requires device or vendorId/productId selector", p.ID)
		}
		if p.MaxReportBytes <= 0 {
			p.MaxReportBytes = 4096
		}
		if p.MaxReportBytes > 16384 {
			return fmt.Errorf("usbHidProvider %s maxReportBytes must be 1..16384", p.ID)
		}
		if err := claim("usbHidProvider", p.ID, p.Socket); err != nil {
			return err
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
		p.Socket = filepath.Clean(p.Socket)
		if err := claim("canProvider", p.ID, p.Socket); err != nil {
			return err
		}
	}
	return nil
}

func validateEndpoint(ep *Endpoint, label string, requireAllowlist bool) error {
	ep.Mode = strings.ToLower(strings.TrimSpace(ep.Mode))
	ep.Network = strings.ToLower(strings.TrimSpace(ep.Network))
	if ep.Network == "" {
		ep.Network = "tcp"
	}
	switch ep.Network {
	case "tcp":
		return validateTCPEndpoint(ep, label, requireAllowlist)
	case "unix", "unixpacket":
		if tlsOptionsConfigured(ep.TLS) {
			return fmt.Errorf("%s TLS options are not valid for %s endpoint", label, ep.Network)
		}
		if len(ep.AllowedCIDRs) > 0 {
			return fmt.Errorf("%s allowedCidrs is not valid for %s endpoint", label, ep.Network)
		}
		if ep.Mode != "listen" && ep.Mode != "connect" {
			return fmt.Errorf("%s mode must be listen or connect", label)
		}
		path := ep.Address
		if ep.Mode == "listen" {
			path = ep.Bind
		}
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%s %s path must be absolute", label, ep.Network)
		}
		path = filepath.Clean(path)
		if ep.Mode == "listen" {
			ep.Bind = path
		} else {
			ep.Address = path
		}
		return nil
	default:
		return fmt.Errorf("%s unsupported network %q", label, ep.Network)
	}
}

func validatePacketFraming(t *Tunnel) error {
	framing := strings.ToLower(strings.TrimSpace(t.PacketFraming))
	if framing == "none" {
		framing = ""
	}
	fieldPacket := t.Field.Network == "unixpacket"
	consumerPacket := t.Consumer.Network == "unixpacket"
	if fieldPacket == consumerPacket {
		if framing != "" {
			return fmt.Errorf("tunnel %s packetFraming is only valid when exactly one endpoint uses unixpacket", t.ID)
		}
		t.PacketFraming = ""
		return nil
	}
	if framing != "length32be" {
		return fmt.Errorf("tunnel %s mixes unixpacket with stream transport and requires packetFraming=length32be", t.ID)
	}
	t.PacketFraming = framing
	return nil
}

func validateTCPEndpoint(ep *Endpoint, label string, requireAllowlist bool) error {
	if !ep.TLS.Enabled && tlsOptionsConfigured(ep.TLS) {
		return fmt.Errorf("%s TLS options require tls.enabled=true", label)
	}
	if ep.KeepAliveS <= 0 {
		ep.KeepAliveS = 30
	}
	if ep.KeepAliveS > 3600 {
		return fmt.Errorf("%s keepAliveSeconds exceeds safe limit", label)
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
		if len(ep.AllowedCIDRs) == 0 && !isLoopbackBind(ep.Bind) {
			if requireAllowlist {
				return fmt.Errorf("%s requires allowedCidrs by security policy", label)
			}
			return fmt.Errorf("%s public listener requires allowedCidrs (fail-closed policy)", label)
		}
		if ep.TLS.Enabled && strings.TrimSpace(ep.TLS.ServerName) != "" {
			return fmt.Errorf("%s TLS listener must not set serverName", label)
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
		if ep.DialTimeoutS > 3600 || ep.ReconnectS > 3600 {
			return fmt.Errorf("%s dial/reconnect timeout exceeds safe limit", label)
		}
		if ep.TLS.Enabled && ep.TLS.RequireClientCert {
			return fmt.Errorf("%s requireClientCert only applies to TLS listeners", label)
		}
		if (ep.TLS.CertFile == "") != (ep.TLS.KeyFile == "") {
			return fmt.Errorf("%s TLS certFile/keyFile must be configured together", label)
		}
	default:
		return fmt.Errorf("%s mode must be listen or connect", label)
	}
	return nil
}

func tlsOptionsConfigured(t TLS) bool {
	return t.Enabled || t.RequireClientCert || strings.TrimSpace(t.CAFile) != "" || strings.TrimSpace(t.CertFile) != "" || strings.TrimSpace(t.KeyFile) != "" || strings.TrimSpace(t.ServerName) != ""
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
		if len(ep.AllowedCIDRs) == 0 && !isLoopbackBind(ep.Bind) {
			if requireAllowlist {
				return fmt.Errorf("%s requires allowedCidrs by security policy", label)
			}
			return fmt.Errorf("%s public UDP listener requires allowedCidrs (fail-closed policy)", label)
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
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func normalizeUSBID(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "0x")
	if value == "" || len(value) > 4 {
		return "", fmt.Errorf("must be a 16-bit hexadecimal value")
	}
	n, err := strconv.ParseUint(value, 16, 16)
	if err != nil {
		return "", fmt.Errorf("must be a 16-bit hexadecimal value")
	}
	return fmt.Sprintf("%04x", n), nil
}

func isHIDRawDevicePath(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if filepath.Dir(path) != "/dev" {
		return false
	}
	base := filepath.Base(path)
	const prefix = "hidraw"
	if !strings.HasPrefix(base, prefix) || len(base) == len(prefix) {
		return false
	}
	for _, r := range base[len(prefix):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
