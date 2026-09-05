//go:build linux

package usbhid

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"
)

var ErrWriteDisabled = errors.New("USB HID write disabled by configuration")

type reportDevice interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}

func Run(ctx context.Context, cfg Config, logger *slog.Logger, hooks Hooks) error {
	var err error
	cfg, err = normalizeConfig(cfg)
	if err != nil {
		return err
	}
	resolvedDevice, identity, err := resolveHIDRawDevice(cfg)
	if err != nil {
		return err
	}
	cfg.Device = resolvedDevice
	if err := validateHIDRawNode(cfg.Device); err != nil {
		return fmt.Errorf("USB HID provider %s validate device: %w", cfg.ID, err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Socket), 0o750); err != nil {
		return fmt.Errorf("USB HID provider %s create socket dir: %w", cfg.ID, err)
	}
	if err := removeStaleSocket(cfg.Socket); err != nil {
		return err
	}
	addr := &net.UnixAddr{Name: cfg.Socket, Net: "unixpacket"}
	ln, err := net.ListenUnix("unixpacket", addr)
	if err != nil {
		return fmt.Errorf("USB HID provider %s listen: %w", cfg.ID, err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(cfg.Socket)
	}()
	if err := os.Chmod(cfg.Socket, 0o660); err != nil {
		return fmt.Errorf("USB HID provider %s chmod socket: %w", cfg.ID, err)
	}
	if hooks.OnReady != nil {
		hooks.OnReady(cfg.ID)
	}

	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = ln.Close()
		case <-stop:
		}
	}()
	defer close(stop)

	if logger != nil {
		vendorID := identity.VendorID
		if vendorID == "" {
			vendorID = cfg.VendorID
		}
		productID := identity.ProductID
		if productID == "" {
			productID = cfg.ProductID
		}
		serialNumber := identity.SerialNumber
		if serialNumber == "" {
			serialNumber = cfg.SerialNumber
		}
		logger.Info("USB HID provider ready", "id", cfg.ID, "socket", cfg.Socket, "device", cfg.Device, "vendorId", vendorID, "productId", productID, "serialNumber", serialNumber, "hidName", identity.Name, "maxReportBytes", cfg.MaxReportBytes, "allowWrite", cfg.AllowWrite)
	}

	var seq uint64
	for {
		conn, err := ln.AcceptUnix()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("USB HID provider %s accept: %w", cfg.ID, err)
		}

		device, err := openHIDRaw(cfg)
		if err != nil {
			_ = conn.Close()
			if logger != nil {
				logger.Warn("USB HID device open failed", "provider", cfg.ID, "device", cfg.Device, "error", err)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				continue
			}
		}

		seq++
		sessionID := fmt.Sprintf("usb-hid:%s:%d", cfg.ID, seq)
		if hooks.OnOpen != nil {
			hooks.OnOpen(sessionID)
		}
		if logger != nil {
			logger.Info("USB HID consumer connected", "provider", cfg.ID, "device", cfg.Device, "socket", cfg.Socket, "allowWrite", cfg.AllowWrite)
		}

		bridgeErr := bridgeReports(ctx, conn, device, cfg, sessionID, hooks)
		_ = conn.Close()
		_ = device.Close()
		if hooks.OnClose != nil {
			hooks.OnClose(sessionID, bridgeErr)
		}
		if logger != nil {
			logger.Info("USB HID consumer disconnected", "provider", cfg.ID, "error", bridgeErr)
		}
	}
}

func validateHIDRawNode(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink USB HID device %s", path)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("USB HID device %s is not a character device", path)
	}
	return nil
}

func openHIDRaw(cfg Config) (reportDevice, error) {
	if err := validateHIDRawNode(cfg.Device); err != nil {
		return nil, err
	}
	flags := os.O_RDONLY
	if cfg.AllowWrite {
		flags = os.O_RDWR
	}
	f, err := os.OpenFile(cfg.Device, flags, 0)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func bridgeReports(ctx context.Context, conn net.Conn, device reportDevice, cfg Config, sessionID string, hooks Hooks) error {
	results := make(chan error, 2)
	go func() { results <- copyReportsToConsumer(conn, device, cfg.MaxReportBytes, sessionID, hooks) }()
	go func() {
		results <- copyReportsToDevice(device, conn, cfg.AllowWrite, cfg.MaxReportBytes, sessionID, hooks)
	}()

	select {
	case <-ctx.Done():
		_ = conn.Close()
		_ = device.Close()
		return nil
	case err := <-results:
		_ = conn.Close()
		_ = device.Close()
		select {
		case <-results:
		case <-time.After(time.Second):
		}
		if isNormalClose(err) {
			return nil
		}
		return err
	}
}

func copyReportsToConsumer(dst net.Conn, src reportDevice, maxReportBytes int, sessionID string, hooks Hooks) error {
	buf := make([]byte, maxReportBytes+1)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if n > maxReportBytes {
				return fmt.Errorf("USB HID report exceeds maxReportBytes: %d > %d", n, maxReportBytes)
			}
			written, werr := dst.Write(buf[:n])
			if werr != nil {
				return werr
			}
			if written != n {
				return io.ErrShortWrite
			}
			if hooks.OnReport != nil {
				hooks.OnReport(sessionID, "field_to_consumer", uint64(n))
			}
		}
		if err != nil {
			return err
		}
	}
}

func copyReportsToDevice(dst reportDevice, src net.Conn, allowWrite bool, maxReportBytes int, sessionID string, hooks Hooks) error {
	buf := make([]byte, maxReportBytes+1)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if n > maxReportBytes {
				return fmt.Errorf("USB HID report exceeds maxReportBytes: %d > %d", n, maxReportBytes)
			}
			if !allowWrite {
				return ErrWriteDisabled
			}
			written, werr := dst.Write(buf[:n])
			if werr != nil {
				return werr
			}
			if written != n {
				return io.ErrShortWrite
			}
			if hooks.OnReport != nil {
				hooks.OnReport(sessionID, "consumer_to_field", uint64(n))
			}
		}
		if err != nil {
			return err
		}
	}
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket path %s", path)
	}
	return os.Remove(path)
}

func isNormalClose(err error) bool {
	return err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrClosed)
}
