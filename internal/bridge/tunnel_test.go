package bridge

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestCopyDuplexPreservesBytesBothDirections(t *testing.T) {
	fieldApp, fieldGW := net.Pipe()
	consumerGW, consumerApp := net.Pipe()
	defer fieldApp.Close()
	defer consumerApp.Close()
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(context.Background(), "test-pair", fieldGW, consumerGW, Hooks{}, time.Second, 100*time.Millisecond)
	}()
	assertDuplexBytes(t, fieldApp, consumerApp)
	_ = fieldApp.Close()
	waitCopy(t, done)
}

func TestOnBytesIsUpdatedBeforeSessionClose(t *testing.T) {
	fieldApp, fieldGW := net.Pipe()
	consumerGW, consumerApp := net.Pipe()
	defer fieldApp.Close()
	defer consumerApp.Close()
	var seen atomic.Uint64
	hooks := Hooks{OnBytes: func(_ string, direction string, n uint64) {
		if direction == "field_to_consumer" {
			seen.Add(n)
		}
	}}
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(context.Background(), "meter", fieldGW, consumerGW, hooks, time.Second, 100*time.Millisecond)
	}()
	payload := []byte("live-metrics")
	go func() { _, _ = fieldApp.Write(payload) }()
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(consumerApp, got); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for seen.Load() != uint64(len(payload)) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if seen.Load() != uint64(len(payload)) {
		t.Fatalf("OnBytes not updated live: got %d", seen.Load())
	}
	_ = fieldApp.Close()
	waitCopy(t, done)
}

func TestSlowPeerWriteTimeoutTerminatesPair(t *testing.T) {
	fieldApp, fieldGW := net.Pipe()
	consumerGW, consumerApp := net.Pipe()
	defer fieldApp.Close()
	defer consumerApp.Close()
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(context.Background(), "slow", fieldGW, consumerGW, Hooks{}, 50*time.Millisecond, 50*time.Millisecond)
	}()
	writeDone := make(chan error, 1)
	go func() { _, err := fieldApp.Write(make([]byte, 128*1024)); writeDone <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected slow peer timeout error")
		}
		var ne net.Error
		if !errors.As(err, &ne) || !ne.Timeout() {
			t.Fatalf("expected net timeout, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("slow peer did not time out")
	}
}

func TestListenPairDeadlineDoesNotDestroyListener(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fieldSource, fieldAddr := testListenSource(t, ctx)
	defer fieldSource.Close()
	consumerSource, _ := testListenSource(t, ctx)
	defer consumerSource.Close()
	peer := dialTestPeer(t, fieldAddr)
	defer peer.Close()
	pairCtx, pairCancel := context.WithTimeout(ctx, 80*time.Millisecond)
	_, _, err := acquirePair(pairCtx, fieldSource, consumerSource, "listen", "listen")
	pairCancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected pair deadline, got %v", err)
	}
	next := dialTestPeer(t, fieldAddr)
	defer next.Close()
}

func TestRepeatedReconnectChurn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fieldSource, fieldAddr := testListenSource(t, ctx)
	defer fieldSource.Close()
	consumerSource, consumerAddr := testListenSource(t, ctx)
	defer consumerSource.Close()
	for i := 0; i < 50; i++ {
		pairCh := make(chan pairResult, 1)
		go func() {
			f, c, e := acquirePair(ctx, fieldSource, consumerSource, "listen", "listen")
			pairCh <- pairResult{f, c, e}
		}()
		fp := dialTestPeer(t, fieldAddr)
		cp := dialTestPeer(t, consumerAddr)
		pair := waitPair(t, pairCh)
		done := make(chan error, 1)
		go func() {
			done <- copyDuplex(ctx, "churn", pair.field, pair.consumer, Hooks{}, time.Second, 20*time.Millisecond)
		}()
		b := []byte{byte(i)}
		if _, err := fp.Write(b); err != nil {
			t.Fatal(err)
		}
		got := make([]byte, 1)
		if _, err := io.ReadFull(cp, got); err != nil {
			t.Fatal(err)
		}
		if got[0] != b[0] {
			t.Fatalf("iteration %d payload changed", i)
		}
		_ = fp.Close()
		_ = cp.Close()
		waitCopy(t, done)
	}
}

func TestListenListenTunnelUsesRealTCPSockets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fieldSource, fieldAddr := testListenSource(t, ctx)
	defer fieldSource.Close()
	consumerSource, consumerAddr := testListenSource(t, ctx)
	defer consumerSource.Close()
	pairCh := make(chan pairResult, 1)
	go func() {
		f, c, e := acquirePair(ctx, fieldSource, consumerSource, "listen", "listen")
		pairCh <- pairResult{f, c, e}
	}()
	fieldPeer := dialTestPeer(t, fieldAddr)
	defer fieldPeer.Close()
	consumerPeer := dialTestPeer(t, consumerAddr)
	defer consumerPeer.Close()
	pair := waitPair(t, pairCh)
	defer pair.field.Close()
	defer pair.consumer.Close()
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(ctx, "listen-listen", pair.field, pair.consumer, Hooks{}, time.Second, 100*time.Millisecond)
	}()
	assertDuplexBytes(t, fieldPeer, consumerPeer)
	_ = fieldPeer.Close()
	waitCopy(t, done)
}

func TestConnectListenWaitsForInboundPeerBeforeDialingField(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deviceListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer deviceListener.Close()
	tcpDeviceListener := deviceListener.(*net.TCPListener)
	fieldSource, err := newSource(ctx, Endpoint{Mode: "connect", Network: "tcp", Address: deviceListener.Addr().String(), DialTimeout: time.Second, Reconnect: 20 * time.Millisecond, KeepAlive: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer fieldSource.Close()
	consumerSource, consumerAddr := testListenSource(t, ctx)
	defer consumerSource.Close()
	pairCh := make(chan pairResult, 1)
	go func() {
		f, c, e := acquirePair(ctx, fieldSource, consumerSource, "connect", "listen")
		pairCh <- pairResult{f, c, e}
	}()
	if err := tcpDeviceListener.SetDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	unexpected, err := tcpDeviceListener.Accept()
	if err == nil {
		_ = unexpected.Close()
		t.Fatal("field was dialed before consumer connected")
	}
	if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("expected timeout got %v", err)
	}
	_ = tcpDeviceListener.SetDeadline(time.Time{})
	consumerPeer := dialTestPeer(t, consumerAddr)
	defer consumerPeer.Close()
	fieldPeer, err := deviceListener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer fieldPeer.Close()
	pair := waitPair(t, pairCh)
	defer pair.field.Close()
	defer pair.consumer.Close()
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(ctx, "connect-listen", pair.field, pair.consumer, Hooks{}, time.Second, 100*time.Millisecond)
	}()
	assertDuplexBytes(t, fieldPeer, consumerPeer)
	_ = consumerPeer.Close()
	waitCopy(t, done)
}

type pairResult struct {
	field, consumer net.Conn
	err             error
}

func testListenSource(t *testing.T, ctx context.Context) (connectionSource, string) {
	t.Helper()
	s, err := newSource(ctx, Endpoint{Mode: "listen", Network: "tcp", Bind: "127.0.0.1:0", KeepAlive: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	l := s.(*listenSource)
	return s, l.ln.Addr().String()
}

func dialTestPeer(t *testing.T, address string) net.Conn {
	t.Helper()
	c, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func waitPair(t *testing.T, ch <-chan pairResult) pairResult {
	t.Helper()
	select {
	case p := <-ch:
		if p.err != nil {
			t.Fatal(p.err)
		}
		return p
	case <-time.After(3 * time.Second):
		t.Fatal("pair acquisition timed out")
		return pairResult{}
	}
}

func waitCopy(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("copyDuplex returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("copyDuplex did not stop")
	}
}

func assertDuplexBytes(t *testing.T, fieldPeer, consumerPeer net.Conn) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = fieldPeer.SetDeadline(deadline)
	_ = consumerPeer.SetDeadline(deadline)
	defer fieldPeer.SetDeadline(time.Time{})
	defer consumerPeer.SetDeadline(time.Time{})
	fp := []byte{1, 3, 0, 100, 255, 0, 126}
	if _, err := fieldPeer.Write(fp); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(fp))
	if _, err := io.ReadFull(consumerPeer, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(fp) {
		t.Fatalf("changed %x != %x", got, fp)
	}
	cp := []byte{0, 1, 0, 0, 0, 6, 1, 3, 128, 255}
	if _, err := consumerPeer.Write(cp); err != nil {
		t.Fatal(err)
	}
	got2 := make([]byte, len(cp))
	if _, err := io.ReadFull(fieldPeer, got2); err != nil {
		t.Fatal(err)
	}
	if string(got2) != string(cp) {
		t.Fatalf("changed %x != %x", got2, cp)
	}
}
