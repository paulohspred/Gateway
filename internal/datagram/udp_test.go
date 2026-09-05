package datagram

import (
	"context"
	"net"
	"testing"
	"time"
)

func startUDPEcho(t *testing.T) (*net.UDPConn, string) {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		buf := make([]byte, 65535)
		for {
			n, peer, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = conn.WriteToUDP(buf[:n], peer)
		}
	}()
	return conn, conn.LocalAddr().String()
}

func freeUDPAddr(t *testing.T) string {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	addr := conn.LocalAddr().String()
	_ = conn.Close()
	return addr
}

func runTunnel(t *testing.T, tunnel *Tunnel) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tunnel.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("udp tunnel shutdown: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("udp tunnel did not stop")
		}
	})
	time.Sleep(30 * time.Millisecond)
	return cancel, done
}

func udpClient(t *testing.T, address string) *net.UDPConn {
	t.Helper()
	target, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, target)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func exchange(t *testing.T, conn *net.UDPConn, payload []byte) []byte {
	t.Helper()
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte(nil), buf[:n]...)
}

func TestUDPTunnelPreservesDatagramBoundaries(t *testing.T) {
	echo, target := startUDPEcho(t)
	defer echo.Close()
	listen := freeUDPAddr(t)
	tunnel := &Tunnel{
		ID:               "udp-boundary",
		Field:            Endpoint{Mode: "listen", Bind: listen},
		Consumer:         Endpoint{Mode: "connect", Address: target},
		IdleTimeout:      time.Second,
		MaxSessions:      16,
		MaxDatagramBytes: 1024,
	}
	runTunnel(t, tunnel)
	client := udpClient(t, listen)

	first := []byte{0x00, 0x01, 0x02, 0xff}
	second := []byte("second-datagram")
	gotFirst := exchange(t, client, first)
	gotSecond := exchange(t, client, second)
	if string(gotFirst) != string(first) || string(gotSecond) != string(second) {
		t.Fatalf("datagram corruption first=%x second=%x", gotFirst, gotSecond)
	}
}

func TestUDPTunnelIsolatesMultiplePeers(t *testing.T) {
	echo, target := startUDPEcho(t)
	defer echo.Close()
	listen := freeUDPAddr(t)
	tunnel := &Tunnel{
		ID:               "udp-peers",
		Field:            Endpoint{Mode: "listen", Bind: listen},
		Consumer:         Endpoint{Mode: "connect", Address: target},
		IdleTimeout:      time.Second,
		MaxSessions:      16,
		MaxDatagramBytes: 1024,
	}
	runTunnel(t, tunnel)
	clientA := udpClient(t, listen)
	clientB := udpClient(t, listen)

	if got := string(exchange(t, clientA, []byte("peer-a"))); got != "peer-a" {
		t.Fatalf("peer A got %q", got)
	}
	if got := string(exchange(t, clientB, []byte("peer-b"))); got != "peer-b" {
		t.Fatalf("peer B got %q", got)
	}
}

func TestUDPTunnelExpiresIdleSession(t *testing.T) {
	echo, target := startUDPEcho(t)
	defer echo.Close()
	listen := freeUDPAddr(t)
	opened := make(chan struct{}, 1)
	closed := make(chan struct{}, 1)
	tunnel := &Tunnel{
		ID:               "udp-idle",
		Field:            Endpoint{Mode: "listen", Bind: listen},
		Consumer:         Endpoint{Mode: "connect", Address: target},
		IdleTimeout:      100 * time.Millisecond,
		MaxSessions:      4,
		MaxDatagramBytes: 1024,
		Hooks: Hooks{
			OnOpen: func(SessionInfo) { opened <- struct{}{} },
			OnClose: func(SessionInfo, error) {
				select {
				case closed <- struct{}{}:
				default:
				}
			},
		},
	}
	runTunnel(t, tunnel)
	client := udpClient(t, listen)
	_ = exchange(t, client, []byte("touch"))
	select {
	case <-opened:
	case <-time.After(time.Second):
		t.Fatal("session did not open")
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("idle session did not close")
	}
}

func TestUDPTunnelEnforcesMaxSessions(t *testing.T) {
	echo, target := startUDPEcho(t)
	defer echo.Close()
	listen := freeUDPAddr(t)
	drops := make(chan string, 4)
	tunnel := &Tunnel{
		ID:               "udp-limit",
		Field:            Endpoint{Mode: "listen", Bind: listen},
		Consumer:         Endpoint{Mode: "connect", Address: target},
		IdleTimeout:      time.Second,
		MaxSessions:      1,
		MaxDatagramBytes: 1024,
		Hooks:            Hooks{OnDrop: func(_ string, reason string) { drops <- reason }},
	}
	runTunnel(t, tunnel)
	clientA := udpClient(t, listen)
	clientB := udpClient(t, listen)
	_ = exchange(t, clientA, []byte("first"))

	if _, err := clientB.Write([]byte("second")); err != nil {
		t.Fatal(err)
	}
	_ = clientB.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	buf := make([]byte, 32)
	if _, err := clientB.Read(buf); err == nil {
		t.Fatal("second peer unexpectedly received a response")
	}
	select {
	case reason := <-drops:
		if reason != "session_limit_or_dial" {
			t.Fatalf("unexpected drop reason %q", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("expected session limit drop")
	}
}

func TestUDPTunnelDropsOversizedDatagram(t *testing.T) {
	echo, target := startUDPEcho(t)
	defer echo.Close()
	listen := freeUDPAddr(t)
	dropped := make(chan string, 1)
	tunnel := &Tunnel{
		ID:               "udp-oversize",
		Field:            Endpoint{Mode: "listen", Bind: listen},
		Consumer:         Endpoint{Mode: "connect", Address: target},
		IdleTimeout:      time.Second,
		MaxSessions:      4,
		MaxDatagramBytes: 4,
		Hooks:            Hooks{OnDrop: func(_ string, reason string) { dropped <- reason }},
	}
	runTunnel(t, tunnel)
	client := udpClient(t, listen)
	if _, err := client.Write([]byte{1, 2, 3, 4, 5}); err != nil {
		t.Fatal(err)
	}
	select {
	case reason := <-dropped:
		if reason != "oversize_peer" {
			t.Fatalf("unexpected drop reason %q", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("oversized datagram was not dropped")
	}
}

func TestUDPTunnelEnforcesAllowlist(t *testing.T) {
	echo, target := startUDPEcho(t)
	defer echo.Close()
	listen := freeUDPAddr(t)
	dropped := make(chan string, 1)
	tunnel := &Tunnel{
		ID: "udp-allowlist",
		Field: Endpoint{
			Mode:         "listen",
			Bind:         listen,
			AllowedCIDRs: []string{"192.0.2.0/24"},
		},
		Consumer:         Endpoint{Mode: "connect", Address: target},
		IdleTimeout:      time.Second,
		MaxSessions:      4,
		MaxDatagramBytes: 1024,
		Hooks:            Hooks{OnDrop: func(_ string, reason string) { dropped <- reason }},
	}
	runTunnel(t, tunnel)
	client := udpClient(t, listen)
	if _, err := client.Write([]byte("blocked")); err != nil {
		t.Fatal(err)
	}
	select {
	case reason := <-dropped:
		if reason != "allowlist" {
			t.Fatalf("unexpected drop reason %q", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("allowlist did not block peer")
	}
}
