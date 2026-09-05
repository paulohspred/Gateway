//go:build linux

package systemdnotify

import (
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
	if err := server.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	nRead, _, err := server.ReadFromUnix(buf)
	if err != nil {
		t.Fatal(err)
	}
	got := string(buf[:nRead])
	if !strings.Contains(got, "READY=1") || !strings.Contains(got, "STATUS=") {
		t.Fatalf("unexpected notification %q", got)
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
