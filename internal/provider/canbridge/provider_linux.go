//go:build linux

package canbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	CANMTU   = 16
	CANFDMTU = 72
)

var ErrTransmitDisabled = errors.New("CAN transmit disabled by configuration")

type Config struct {
	ID            string
	Interface     string
	Socket        string
	EnableFD      bool
	ReceiveOwn    bool
	AllowTransmit bool
}

type Hooks struct {
	OnOpen  func(sessionID string)
	OnFrame func(sessionID, direction string, n uint64)
	OnClose func(sessionID string, err error)
}

type rawCAN struct {
	fd   int
	once sync.Once
}

func (c *rawCAN) Read(p []byte) (int, error)  { return unix.Read(c.fd, p) }
func (c *rawCAN) Write(p []byte) (int, error) { return unix.Write(c.fd, p) }
func (c *rawCAN) Close() error {
	var err error
	c.once.Do(func() { err = unix.Close(c.fd) })
	return err
}

type frameDevice interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}

func Run(ctx context.Context, cfg Config, logger *slog.Logger, hooks Hooks) error {
	if cfg.ID == "" || cfg.Interface == "" || !filepath.IsAbs(cfg.Socket) {
		return fmt.Errorf("CAN provider requires id, interface and absolute socket")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Socket), 0o750); err != nil {
		return err
	}
	_ = os.Remove(cfg.Socket)
	addr := &net.UnixAddr{Name: cfg.Socket, Net: "unixpacket"}
	ln, err := net.ListenUnix("unixpacket", addr)
	if err != nil {
		return fmt.Errorf("CAN provider %s listen: %w", cfg.ID, err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(cfg.Socket)
	}()
	if err := os.Chmod(cfg.Socket, 0o660); err != nil {
		return err
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

	var seq uint64
	for {
		conn, err := ln.AcceptUnix()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		device, err := openRawCAN(cfg)
		if err != nil {
			_ = conn.Close()
			if logger != nil {
				logger.Error("CAN interface open failed", "provider", cfg.ID, "interface", cfg.Interface, "error", err)
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				continue
			}
		}

		seq++
		sessionID := fmt.Sprintf("can:%s:%d", cfg.ID, seq)
		if hooks.OnOpen != nil {
			hooks.OnOpen(sessionID)
		}
		if logger != nil {
			logger.Info("CAN consumer connected", "provider", cfg.ID, "interface", cfg.Interface, "socket", cfg.Socket, "fd", cfg.EnableFD, "allowTransmit", cfg.AllowTransmit)
		}
		bridgeErr := bridgeFrames(ctx, conn, device, cfg.AllowTransmit, sessionID, hooks)
		_ = conn.Close()
		_ = device.Close()
		if hooks.OnClose != nil {
			hooks.OnClose(sessionID, bridgeErr)
		}
		if logger != nil {
			logger.Info("CAN consumer disconnected", "provider", cfg.ID, "error", bridgeErr)
		}
	}
}

func openRawCAN(cfg Config) (frameDevice, error) {
	iface, err := net.InterfaceByName(cfg.Interface)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.CAN_RAW)
	if err != nil {
		return nil, err
	}
	dev := &rawCAN{fd: fd}
	fail := func(err error) (frameDevice, error) {
		_ = dev.Close()
		return nil, err
	}
	if cfg.EnableFD {
		if err := unix.SetsockoptInt(fd, unix.SOL_CAN_RAW, unix.CAN_RAW_FD_FRAMES, 1); err != nil {
			return fail(fmt.Errorf("enable CAN-FD: %w", err))
		}
	}
	if cfg.ReceiveOwn {
		if err := unix.SetsockoptInt(fd, unix.SOL_CAN_RAW, unix.CAN_RAW_RECV_OWN_MSGS, 1); err != nil {
			return fail(fmt.Errorf("receive own CAN frames: %w", err))
		}
	}
	if err := unix.Bind(fd, &unix.SockaddrCAN{Ifindex: iface.Index}); err != nil {
		return fail(err)
	}
	return dev, nil
}

func bridgeFrames(ctx context.Context, conn net.Conn, device frameDevice, allowTransmit bool, sessionID string, hooks Hooks) error {
	results := make(chan error, 2)
	go func() { results <- copyFramesToConsumer(conn, device, sessionID, hooks) }()
	go func() { results <- copyFramesToCAN(device, conn, allowTransmit, sessionID, hooks) }()
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

func copyFramesToConsumer(dst net.Conn, src frameDevice, sessionID string, hooks Hooks) error {
	buf := make([]byte, CANFDMTU)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if !validFrameSize(n) {
				return fmt.Errorf("invalid SocketCAN frame size %d", n)
			}
			written, werr := dst.Write(buf[:n])
			if werr != nil {
				return werr
			}
			if written != n {
				return io.ErrShortWrite
			}
			if hooks.OnFrame != nil {
				hooks.OnFrame(sessionID, "field_to_consumer", uint64(n))
			}
		}
		if err != nil {
			return err
		}
	}
}

func copyFramesToCAN(dst frameDevice, src net.Conn, allowTransmit bool, sessionID string, hooks Hooks) error {
	buf := make([]byte, CANFDMTU+1)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if !validFrameSize(n) {
				return fmt.Errorf("invalid SocketCAN frame packet size %d", n)
			}
			if !allowTransmit {
				return ErrTransmitDisabled
			}
			written, werr := dst.Write(buf[:n])
			if werr != nil {
				return werr
			}
			if written != n {
				return io.ErrShortWrite
			}
			if hooks.OnFrame != nil {
				hooks.OnFrame(sessionID, "consumer_to_field", uint64(n))
			}
		}
		if err != nil {
			return err
		}
	}
}

func validFrameSize(n int) bool { return n == CANMTU || n == CANFDMTU }

func isNormalClose(err error) bool {
	return err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrClosed)
}
