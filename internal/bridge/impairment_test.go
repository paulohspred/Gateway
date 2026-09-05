package bridge

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type impairedConn struct {
	net.Conn
	maxRead   int
	maxWrite  int
	baseDelay time.Duration
	readSeq   atomic.Uint64
	writeSeq  atomic.Uint64
}

func (c *impairedConn) Read(p []byte) (int, error) {
	if c.maxRead > 0 && len(p) > c.maxRead {
		p = p[:c.maxRead]
	}
	c.delay(c.readSeq.Add(1))
	return c.Conn.Read(p)
}

func (c *impairedConn) Write(p []byte) (int, error) {
	if c.maxWrite > 0 && len(p) > c.maxWrite {
		p = p[:c.maxWrite]
	}
	c.delay(c.writeSeq.Add(1))
	return c.Conn.Write(p)
}

func (c *impairedConn) delay(seq uint64) {
	if c.baseDelay <= 0 {
		return
	}
	jitter := time.Duration(seq%5) * c.baseDelay / 3
	time.Sleep(c.baseDelay + jitter)
}

func TestImpairedDuplexPreservesStream(t *testing.T) {
	fieldApp, fieldGWRaw := net.Pipe()
	consumerGWRaw, consumerApp := net.Pipe()
	defer fieldApp.Close()
	defer consumerApp.Close()

	fieldGW := &impairedConn{Conn: fieldGWRaw, maxRead: 7, maxWrite: 11, baseDelay: 50 * time.Microsecond}
	consumerGW := &impairedConn{Conn: consumerGWRaw, maxRead: 13, maxWrite: 5, baseDelay: 40 * time.Microsecond}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(ctx, "impaired", fieldGW, consumerGW, Hooks{}, 2*time.Second, 100*time.Millisecond)
	}()

	if err := roundTripPayload(fieldApp, consumerApp, stressPayload(41, 4096)); err != nil {
		t.Fatal(err)
	}
	if err := roundTripPayload(consumerApp, fieldApp, stressPayload(73, 3073)); err != nil {
		t.Fatal(err)
	}

	_ = fieldApp.Close()
	_ = consumerApp.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("impaired bridge did not stop")
	}
}

func TestSoakImpairedReconnect(t *testing.T) {
	if os.Getenv("RC_GATEWAY_SOAK") != "1" {
		t.Skip("set RC_GATEWAY_SOAK=1 to run soak gate")
	}
	seconds := soakInt("RC_GATEWAY_SOAK_SECONDS", 30)
	if seconds < 1 || seconds > 7*24*60*60 {
		t.Fatalf("unsafe RC_GATEWAY_SOAK_SECONDS=%d", seconds)
	}

	baselineG := runtime.NumGoroutine()
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	iterations := 0
	for time.Now().Before(deadline) {
		if err := exerciseImpairedReconnect(iterations); err != nil {
			t.Fatalf("iteration %d: %v", iterations, err)
		}
		iterations++
	}
	if iterations < 10 {
		t.Fatalf("soak executed too few iterations: %d", iterations)
	}

	runtime.GC()
	time.Sleep(250 * time.Millisecond)
	afterG := runtime.NumGoroutine()
	if afterG > baselineG+32 {
		t.Fatalf("possible goroutine leak after soak: baseline=%d after=%d iterations=%d", baselineG, afterG, iterations)
	}
	t.Logf("soak completed: seconds=%d iterations=%d", seconds, iterations)
}

func exerciseImpairedReconnect(iteration int) error {
	fieldApp, fieldGWRaw := net.Pipe()
	consumerGWRaw, consumerApp := net.Pipe()
	defer fieldApp.Close()
	defer consumerApp.Close()

	fieldGW := &impairedConn{Conn: fieldGWRaw, maxRead: 3 + iteration%17, maxWrite: 5 + iteration%19, baseDelay: time.Duration(20+iteration%7*10) * time.Microsecond}
	consumerGW := &impairedConn{Conn: consumerGWRaw, maxRead: 4 + iteration%13, maxWrite: 6 + iteration%11, baseDelay: time.Duration(15+iteration%5*10) * time.Microsecond}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(ctx, fmt.Sprintf("soak-%d", iteration), fieldGW, consumerGW, Hooks{}, 2*time.Second, 50*time.Millisecond)
	}()

	if err := roundTripPayload(fieldApp, consumerApp, stressPayload(iteration+11, 257+iteration%769)); err != nil {
		return err
	}
	if err := roundTripPayload(consumerApp, fieldApp, stressPayload(iteration+1009, 193+iteration%521)); err != nil {
		return err
	}
	_ = fieldApp.Close()
	_ = consumerApp.Close()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		return fmt.Errorf("bridge did not stop after reconnect cycle")
	}
}

func roundTripPayload(src, dst net.Conn, payload []byte) error {
	writeCh := make(chan error, 1)
	go func() {
		_, err := src.Write(payload)
		writeCh <- err
	}()
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(dst, got); err != nil {
		return err
	}
	if string(got) != string(payload) {
		return fmt.Errorf("payload changed under impairment")
	}
	return <-writeCh
}

func soakInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return -1
	}
	return n
}
