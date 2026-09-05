//go:build linux

package systemdnotify

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Notifier struct {
	socket           string
	watchdogInterval time.Duration
	watchdogOnce     sync.Once
}

func FromEnv() *Notifier {
	n := &Notifier{socket: strings.TrimSpace(os.Getenv("NOTIFY_SOCKET"))}
	if n.socket == "" || !watchdogAppliesToCurrentProcess() {
		return n
	}
	raw := strings.TrimSpace(os.Getenv("WATCHDOG_USEC"))
	if raw == "" {
		return n
	}
	usec, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || usec == 0 {
		return n
	}
	period := time.Duration(usec) * time.Microsecond
	if period < 2*time.Second {
		period = 2 * time.Second
	}
	n.watchdogInterval = period / 2
	return n
}

func watchdogAppliesToCurrentProcess() bool {
	raw := strings.TrimSpace(os.Getenv("WATCHDOG_PID"))
	if raw == "" {
		return true
	}
	pid, err := strconv.Atoi(raw)
	return err == nil && pid == os.Getpid()
}

func (n *Notifier) Enabled() bool { return n != nil && n.socket != "" }

func (n *Notifier) WatchdogEnabled() bool {
	return n != nil && n.Enabled() && n.watchdogInterval > 0
}

func (n *Notifier) Ready() error {
	return n.Notify("READY=1\nSTATUS=RC Universal Gateway ready")
}

func (n *Notifier) Stopping() error {
	return n.Notify("STOPPING=1\nSTATUS=RC Universal Gateway stopping")
}

func (n *Notifier) Notify(state string) error {
	if n == nil || !n.Enabled() {
		return nil
	}
	if strings.TrimSpace(state) == "" {
		return fmt.Errorf("systemd notify state is empty")
	}
	addr := &net.UnixAddr{Name: n.socket, Net: "unixgram"}
	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		return fmt.Errorf("dial NOTIFY_SOCKET: %w", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(state)); err != nil {
		return fmt.Errorf("write NOTIFY_SOCKET: %w", err)
	}
	return nil
}

func (n *Notifier) StartWatchdog(ctx context.Context, onError func(error)) {
	if n == nil || !n.WatchdogEnabled() {
		return
	}
	n.watchdogOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(n.watchdogInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := n.Notify("WATCHDOG=1"); err != nil && onError != nil {
						onError(err)
					}
				}
		}()
	})
}
