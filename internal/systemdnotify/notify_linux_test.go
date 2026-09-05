//go:build linux

package systemdnotify

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestReadyNotification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.sock")
	addr := &net.UnixAddr{Name: path, Net: "unixgram"}
	server, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	t.Setenv("NOTIFY_SOCKET", path)
	t.Setenv("WATCHDOG_USEC", "")
	t.Setenv("WATCHDOG_PID", "")

	n := FromEnv()
	if !n.Enabled() {
		t.Fatal("notifier should be enabled")
	}
	if err := n.Ready(); err != nil {
		t.Fatal(err)
	}
	got := readDatagram(t, server, time.Second)
	if !strings.Contains(got, "READY=1") || !strings.Contains(got, "STATUS=") {
		t.Fatalf("unexpected notification %q", got)
	}
}

func TestAbstractReadyNotification(t *testing.T) {
	abstract := "rc-gateway-notify-" + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	server, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: "\x00" + abstract, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	t.Setenv("NOTIFY_SOCKET", "@"+abstract)
	t.Setenv("WATCHDOG_USEC", "")
	t.Setenv("WATCHDOG_PID", "")

	n := FromEnv()
	if err := n.Ready(); err != nil {
		t.Fatal(err)
	}
	if got := readDatagram(t, server, time.Second); !strings.Contains(got, "READY=1") {
		t.Fatalf("unexpected abstract-socket notification %q", got)
	}
}

func TestWatchdogEnvironmentAppliesToCurrentPID(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "/run/does-not-need-to-exist")
	t.Setenv("WATCHDOG_USEC", "30000000")
	t.Setenv("WATCHDOG_PID", strconv.Itoa(os.Getpid()))
	n := FromEnv()
	if !n.WatchdogEnabled() || n.watchdogInterval != 15*time.Second {
		t.Fatalf("unexpected watchdog interval %v enabled=%v", n.watchdogInterval, n.WatchdogEnabled())
	}

	t.Setenv("WATCHDOG_PID", strconv.Itoa(os.Getpid()+1))
	n = FromEnv()
	if n.WatchdogEnabled() {
		t.Fatal("watchdog should ignore a different WATCHDOG_PID")
	}
}

func TestWatchdogSendsWithinConfiguredInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watchdog.sock")
	server, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	t.Setenv("NOTIFY_SOCKET", path)
	t.Setenv("WATCHDOG_USEC", "100000")
	t.Setenv("WATCHDOG_PID", strconv.Itoa(os.Getpid()))

	n := FromEnv()
	if n.watchdogInterval != 50*time.Millisecond {
		t.Fatalf("unexpected watchdog interval %v", n.watchdogInterval)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	n.StartWatchdog(ctx, func(err error) { errCh <- err })

	if got := readDatagram(t, server, 500*time.Millisecond); got != "WATCHDOG=1" {
		t.Fatalf("unexpected watchdog notification %q", got)
	}
	select {
	case err := <-errCh:
		t.Fatalf("watchdog returned error: %v", err)
	default:
	}
}

func TestWatchdogRejectsOverflowDuration(t *testing.T) {
	t.Setenv("NOTIFY_SOCKET", "/run/does-not-need-to-exist")
	t.Setenv("WATCHDOG_USEC", "18446744073709551615")
	t.Setenv("WATCHDOG_PID", strconv.Itoa(os.Getpid()))
	if n := FromEnv(); n.WatchdogEnabled() {
		t.Fatal("overflowing watchdog duration must be rejected")
	}
}

func readDatagram(t *testing.T, server *net.UnixConn, timeout time.Duration) string {
	t.Helper()
	if err := server.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	nRead, _, err := server.ReadFromUnix(buf)
	if err != nil {
		t.Fatal(err)
	}
	return string(buf[:nRead])
}
