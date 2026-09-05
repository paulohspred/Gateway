package bridge

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestTunnelParallelTCPPairsUseRealSockets(t *testing.T) {
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()

	fieldAddr := freeTCPAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	accepted := make(chan struct{}, 16)
	releaseBackend := make(chan struct{})
	backendErr := make(chan error, 1)
	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				select {
				case backendErr <- err:
				default:
				}
				return
			}
			accepted <- struct{}{}
			go func(c net.Conn) {
				defer c.Close()
				select {
				case <-releaseBackend:
				case <-ctx.Done():
					return
				}
				payload := make([]byte, 32)
				n, err := c.Read(payload)
				if err != nil {
					return
				}
				_, _ = c.Write(payload[:n])
			}(conn)
		}
	}()

	ready := make(chan struct{})
	var readyOnce sync.Once
	limiter, err := NewPairLimiter(8)
	if err != nil {
		t.Fatal(err)
	}
	tunnel := &Tunnel{
		ID: "parallel-real-tcp",
		Field: Endpoint{
			Mode:      "listen",
			Network:   "tcp",
			Bind:      fieldAddr,
			KeepAlive: time.Second,
		},
		Consumer: Endpoint{
			Mode:        "connect",
			Network:     "tcp",
			Address:     backend.Addr().String(),
			DialTimeout: time.Second,
			Reconnect:   10 * time.Millisecond,
			KeepAlive:   time.Second,
		},
		PairTimeout:        2 * time.Second,
		WriteTimeout:       time.Second,
		DrainTimeout:       50 * time.Millisecond,
		MaxConcurrentPairs: 4,
		GlobalPairLimiter:  limiter,
		Hooks: Hooks{OnReady: func(string) { readyOnce.Do(func() { close(ready) }) }},
	}
	tunnelDone := make(chan error, 1)
	go func() { tunnelDone <- tunnel.Run(ctx) }()

	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("tunnel did not become ready")
	}

	clientErr := make(chan error, 8)
	for i := 0; i < 8; i++ {
		i := i
		go func() {
			conn, err := net.DialTimeout("tcp", fieldAddr, time.Second)
			if err != nil {
				clientErr <- err
				return
			}
			defer conn.Close()
			payload := []byte(fmt.Sprintf("pair-%02d", i))
			if _, err := conn.Write(payload); err != nil {
				clientErr <- err
				return
			}
			got := make([]byte, len(payload))
			if _, err := io.ReadFull(conn, got); err != nil {
				clientErr <- err
				return
			}
			if string(got) != string(payload) {
				clientErr <- fmt.Errorf("payload changed: got %q want %q", got, payload)
				return
			}
			clientErr <- nil
		}()
	}

	// The backend intentionally blocks I/O until four independent connections
	// have been established. A sequential Tunnel.Run cannot satisfy this gate.
	for i := 0; i < 4; i++ {
		select {
		case <-accepted:
		case err := <-backendErr:
			t.Fatalf("backend accept: %v", err)
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d parallel backend connections established", i)
		}
	}
	if limiter.Active() != 4 {
		t.Fatalf("expected four active pair slots before release, got %d", limiter.Active())
	}
	close(releaseBackend)

	for i := 0; i < 8; i++ {
		select {
		case err := <-clientErr:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("parallel client timed out")
		}
	}

	cancel()
	_ = backend.Close()
	select {
	case err := <-tunnelDone:
		if err != nil {
			t.Fatalf("tunnel shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tunnel did not stop")
	}
	if limiter.Active() != 0 {
		t.Fatalf("pair limiter leaked %d slots", limiter.Active())
	}
}

func freeTCPAddress(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}
