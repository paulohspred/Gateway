package bridge

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestTCPHalfCloseAllowsPendingResponse(t *testing.T) {
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
	consumerPeer := dialTestPeer(t, consumerAddr)
	defer fieldPeer.Close()
	defer consumerPeer.Close()
	pair := waitPair(t, pairCh)
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(ctx, "half-close", pair.field, pair.consumer, Hooks{}, time.Second, time.Second)
	}()
	request := []byte("request-before-closewrite")
	if _, err := fieldPeer.Write(request); err != nil {
		t.Fatal(err)
	}
	if err := fieldPeer.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	gotRequest := make([]byte, len(request))
	if _, err := io.ReadFull(consumerPeer, gotRequest); err != nil {
		t.Fatal(err)
	}
	response := []byte("response-after-half-close")
	if _, err := consumerPeer.Write(response); err != nil {
		t.Fatal(err)
	}
	if err := consumerPeer.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	gotResponse := make([]byte, len(response))
	if _, err := io.ReadFull(fieldPeer, gotResponse); err != nil {
		t.Fatal(err)
	}
	if string(gotResponse) != string(response) {
		t.Fatalf("response changed after half-close: %q", gotResponse)
	}
	waitCopy(t, done)
}
func TestTCPResetTerminatesPair(t *testing.T) {
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
	consumerPeer := dialTestPeer(t, consumerAddr)
	defer consumerPeer.Close()
	pair := waitPair(t, pairCh)
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(ctx, "rst", pair.field, pair.consumer, Hooks{}, time.Second, 100*time.Millisecond)
	}()
	if err := fieldPeer.(*net.TCPConn).SetLinger(0); err != nil {
		t.Fatal(err)
	}
	_ = fieldPeer.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RST did not terminate pair")
	}
}
