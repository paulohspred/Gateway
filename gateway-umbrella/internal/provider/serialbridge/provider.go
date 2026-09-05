package serialbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.bug.st/serial"
)

type Config struct {
	ID          string
	Socket      string
	Device      string
	Standard    string
	BaudRate    int
	DataBits    int
	Parity      string
	StopBits    string
	ReadTimeout time.Duration
	RTS         bool
	DTR         bool
}

func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := removeStaleSocket(cfg.Socket); err != nil {
		return err
	}
	ln, err := net.Listen("unix", cfg.Socket)
	if err != nil {
		return fmt.Errorf("serial provider %s listen: %w", cfg.ID, err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(cfg.Socket)
	}()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	if logger != nil {
		logger.Info("serial provider ready", "id", cfg.ID, "socket", cfg.Socket, "device", cfg.Device, "standard", cfg.Standard)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("serial provider %s accept: %w", cfg.ID, err)
		}
		if err := servePair(ctx, cfg, conn, logger); err != nil && logger != nil && ctx.Err() == nil {
			logger.Warn("serial pair closed with error", "id", cfg.ID, "error", err)
		}
	}
}

func Validate(cfg Config) error {
	if strings.TrimSpace(cfg.ID) == "" {
		return fmt.Errorf("serial provider id is required")
	}
	if !filepath.IsAbs(cfg.Socket) {
		return fmt.Errorf("serial provider %s socket must be absolute", cfg.ID)
	}
	if strings.TrimSpace(cfg.Device) == "" {
		return fmt.Errorf("serial provider %s device is required", cfg.ID)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Standard)) {
	case "rs232", "rs422", "rs485":
	default:
		return fmt.Errorf("serial provider %s standard must be rs232, rs422 or rs485", cfg.ID)
	}
	if cfg.BaudRate <= 0 {
		return fmt.Errorf("serial provider %s baudRate must be > 0", cfg.ID)
	}
	if cfg.DataBits < 5 || cfg.DataBits > 8 {
		return fmt.Errorf("serial provider %s dataBits must be 5..8", cfg.ID)
	}
	if _, err := parseParity(cfg.Parity); err != nil {
		return fmt.Errorf("serial provider %s: %w", cfg.ID, err)
	}
	if _, err := parseStopBits(cfg.StopBits); err != nil {
		return fmt.Errorf("serial provider %s: %w", cfg.ID, err)
	}
	if cfg.ReadTimeout < 0 || cfg.ReadTimeout > time.Minute {
		return fmt.Errorf("serial provider %s read timeout must be between 0 and 60s", cfg.ID)
	}
	return nil
}

func servePair(ctx context.Context, cfg Config, conn net.Conn, logger *slog.Logger) error {
	defer conn.Close()
	parity, _ := parseParity(cfg.Parity)
	stopBits, _ := parseStopBits(cfg.StopBits)
	mode := &serial.Mode{
		BaudRate:          cfg.BaudRate,
		DataBits:          cfg.DataBits,
		Parity:            parity,
		StopBits:          stopBits,
		InitialStatusBits: &serial.ModemOutputBits{RTS: cfg.RTS, DTR: cfg.DTR},
	}
	port, err := serial.Open(cfg.Device, mode)
	if err != nil {
		return fmt.Errorf("open %s: %w", cfg.Device, err)
	}
	defer port.Close()
	if cfg.ReadTimeout > 0 {
		if err := port.SetReadTimeout(cfg.ReadTimeout); err != nil {
			return fmt.Errorf("set serial read timeout: %w", err)
		}
	}
	if logger != nil {
		logger.Info("serial pair open", "id", cfg.ID, "device", cfg.Device, "peer", conn.RemoteAddr())
	}
	done := make(chan error, 2)
	go func() { _, err := io.CopyBuffer(conn, port, make([]byte, 64*1024)); done <- normalize(err) }()
	go func() { _, err := io.CopyBuffer(port, conn, make([]byte, 64*1024)); done <- normalize(err) }()
	select {
	case <-ctx.Done():
		_ = conn.Close()
		_ = port.Close()
		return nil
	case err := <-done:
		_ = conn.Close()
		_ = port.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		return err
	}
}

func parseParity(v string) (serial.Parity, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "none", "n":
		return serial.NoParity, nil
	case "odd", "o":
		return serial.OddParity, nil
	case "even", "e":
		return serial.EvenParity, nil
	case "mark", "m":
		return serial.MarkParity, nil
	case "space", "s":
		return serial.SpaceParity, nil
	default:
		return serial.NoParity, fmt.Errorf("invalid parity %q", v)
	}
}

func parseStopBits(v string) (serial.StopBits, error) {
	switch strings.TrimSpace(v) {
	case "", "1":
		return serial.OneStopBit, nil
	case "1.5":
		return serial.OnePointFiveStopBits, nil
	case "2":
		return serial.TwoStopBits, nil
	default:
		return serial.OneStopBit, fmt.Errorf("invalid stopBits %q", v)
	}
}

func normalize(err error) error {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
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
