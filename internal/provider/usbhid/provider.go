package usbhid

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultMaxReportBytes = 4096
	MaxReportBytes        = 16384
)

type Config struct {
	ID             string
	Socket         string
	Device         string
	VendorID       string
	ProductID      string
	SerialNumber   string
	MaxReportBytes int
	AllowWrite     bool
}

type Hooks struct {
	OnReady  func(providerID string)
	OnOpen   func(sessionID string)
	OnReport func(sessionID, direction string, n uint64)
	OnClose  func(sessionID string, err error)
}

func Validate(cfg Config) error {
	_, err := normalizeConfig(cfg)
	return err
}

func normalizeConfig(cfg Config) (Config, error) {
	cfg.ID = strings.TrimSpace(cfg.ID)
	cfg.Socket = filepath.Clean(strings.TrimSpace(cfg.Socket))
	cfg.Device = strings.TrimSpace(cfg.Device)
	cfg.SerialNumber = strings.TrimSpace(cfg.SerialNumber)
	if cfg.ID == "" {
		return cfg, fmt.Errorf("USB HID provider id is required")
	}
	if !filepath.IsAbs(cfg.Socket) {
		return cfg, fmt.Errorf("USB HID provider %s socket must be absolute", cfg.ID)
	}

	var err error
	if cfg.VendorID != "" {
		cfg.VendorID, err = normalizeUSBID(cfg.VendorID)
		if err != nil {
			return cfg, fmt.Errorf("USB HID provider %s invalid vendorId: %w", cfg.ID, err)
		}
	}
	if cfg.ProductID != "" {
		cfg.ProductID, err = normalizeUSBID(cfg.ProductID)
		if err != nil {
			return cfg, fmt.Errorf("USB HID provider %s invalid productId: %w", cfg.ID, err)
		}
	}
	if (cfg.VendorID == "") != (cfg.ProductID == "") {
		return cfg, fmt.Errorf("USB HID provider %s vendorId and productId must be configured together", cfg.ID)
	}
	if cfg.SerialNumber != "" && cfg.VendorID == "" {
		return cfg, fmt.Errorf("USB HID provider %s serialNumber requires vendorId/productId", cfg.ID)
	}

	if cfg.Device != "" {
		cfg.Device = filepath.Clean(cfg.Device)
		if !isHIDRawDevicePath(cfg.Device) {
			return cfg, fmt.Errorf("USB HID provider %s device must be /dev/hidrawN", cfg.ID)
		}
	} else if cfg.VendorID == "" {
		return cfg, fmt.Errorf("USB HID provider %s requires device or vendorId/productId selector", cfg.ID)
	}

	if cfg.MaxReportBytes <= 0 {
		cfg.MaxReportBytes = DefaultMaxReportBytes
	}
	if cfg.MaxReportBytes > MaxReportBytes {
		return cfg, fmt.Errorf("USB HID provider %s maxReportBytes must be 1..%d", cfg.ID, MaxReportBytes)
	}
	return cfg, nil
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
