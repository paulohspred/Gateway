//go:build linux

package usbhid

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	hidrawClassRoot  = "/sys/class/hidraw"
	hidrawDeviceRoot = "/dev"
)

type hidIdentity struct {
	VendorID     string
	ProductID    string
	SerialNumber string
	Name         string
}

func resolveHIDRawDevice(cfg Config) (string, hidIdentity, error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return "", hidIdentity{}, err
	}

	if cfg.Device != "" {
		identity, identityErr := readHIDIdentity(filepath.Base(cfg.Device))
		if cfg.VendorID == "" {
			// Explicit device paths remain usable even if sysfs metadata is not
			// readable. openHIDRaw still validates the /dev node itself.
			return cfg.Device, identity, nil
		}
		if identityErr != nil {
			return "", hidIdentity{}, fmt.Errorf("USB HID provider %s cannot verify selector for %s: %w", cfg.ID, cfg.Device, identityErr)
		}
		if !matchesHIDSelector(identity, cfg) {
			return "", hidIdentity{}, fmt.Errorf("USB HID provider %s device %s does not match selector %s:%s serial=%q (found %s:%s serial=%q)", cfg.ID, cfg.Device, cfg.VendorID, cfg.ProductID, cfg.SerialNumber, identity.VendorID, identity.ProductID, identity.SerialNumber)
		}
		return cfg.Device, identity, nil
	}

	entries, err := os.ReadDir(hidrawClassRoot)
	if err != nil {
		return "", hidIdentity{}, fmt.Errorf("USB HID provider %s enumerate hidraw devices: %w", cfg.ID, err)
	}
	type match struct {
		device   string
		identity hidIdentity
	}
	matches := make([]match, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() || !isHIDRawName(entry.Name()) {
			continue
		}
		identity, err := readHIDIdentity(entry.Name())
		if err != nil {
			continue
		}
		if matchesHIDSelector(identity, cfg) {
			matches = append(matches, match{device: filepath.Join(hidrawDeviceRoot, entry.Name()), identity: identity})
		}
	}

	switch len(matches) {
	case 0:
		return "", hidIdentity{}, fmt.Errorf("USB HID provider %s found no hidraw device matching %s:%s serial=%q", cfg.ID, cfg.VendorID, cfg.ProductID, cfg.SerialNumber)
	case 1:
		return matches[0].device, matches[0].identity, nil
	default:
		return "", hidIdentity{}, fmt.Errorf("USB HID provider %s found %d hidraw devices matching %s:%s; configure serialNumber or explicit device", cfg.ID, len(matches), cfg.VendorID, cfg.ProductID)
	}
}

func readHIDIdentity(hidrawName string) (hidIdentity, error) {
	if !isHIDRawName(hidrawName) {
		return hidIdentity{}, fmt.Errorf("invalid hidraw name %q", hidrawName)
	}
	raw, err := os.ReadFile(filepath.Join(hidrawClassRoot, hidrawName, "device", "uevent"))
	if err != nil {
		return hidIdentity{}, err
	}
	return parseHIDUevent(string(raw))
}

func parseHIDUevent(raw string) (hidIdentity, error) {
	values := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	parts := strings.Split(values["HID_ID"], ":")
	if len(parts) != 3 {
		return hidIdentity{}, fmt.Errorf("HID_ID is missing or malformed")
	}
	vendorID, err := normalizeHIDUeventID(parts[1])
	if err != nil {
		return hidIdentity{}, fmt.Errorf("invalid HID vendor id: %w", err)
	}
	productID, err := normalizeHIDUeventID(parts[2])
	if err != nil {
		return hidIdentity{}, fmt.Errorf("invalid HID product id: %w", err)
	}
	return hidIdentity{
		VendorID:     vendorID,
		ProductID:    productID,
		SerialNumber: strings.TrimSpace(values["HID_UNIQ"]),
		Name:         strings.TrimSpace(values["HID_NAME"]),
	}, nil
}

func normalizeHIDUeventID(value string) (string, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(value), 16, 32)
	if err != nil || n > 0xffff {
		return "", fmt.Errorf("must fit in 16 bits")
	}
	return fmt.Sprintf("%04x", n), nil
}

func matchesHIDSelector(identity hidIdentity, cfg Config) bool {
	if identity.VendorID != cfg.VendorID || identity.ProductID != cfg.ProductID {
		return false
	}
	return cfg.SerialNumber == "" || identity.SerialNumber == cfg.SerialNumber
}

func isHIDRawName(name string) bool {
	return isHIDRawDevicePath(filepath.Join("/dev", name))
}
