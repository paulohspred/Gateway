package bridge

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestStressConcurrentDuplexPairs(t *testing.T) {
	if os.Getenv("RC_GATEWAY_STRESS") != "1" {
		t.Skip("set RC_GATEWAY_STRESS=1 to run production stress gate")
	}
	pairs := stressInt("RC_GATEWAY_STRESS_PAIRS", 1000)
	if pairs < 1 || pairs > 5000 {
		t.Fatalf("unsafe RC_GATEWAY_STRESS_PAIRS=%d", pairs)
	}

	baselineG := runtime.NumGoroutine()
	errCh := make(chan error, pairs)
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < pairs; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := exerciseInMemoryPair(i); err != nil {
				errCh <- fmt.Errorf("pair %d: %w", i, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	runtime.GC()
	time.Sleep(250 * time.Millisecond)
	afterG := runtime.NumGoroutine()
	if afterG > baselineG+64 {
		t.Fatalf("possible goroutine leak after %d concurrent pairs: baseline=%d after=%d", pairs, baselineG, afterG)
	}
}

func exerciseInMemoryPair(id int) error {
	fieldApp, fieldGW := net.Pipe()
	consumerGW, consumerApp := net.Pipe()
	defer fieldApp.Close()
	defer consumerApp.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(ctx, fmt.Sprintf("stress-%d", id), fieldGW, consumerGW, Hooks{}, 3*time.Second, 50*time.Millisecond)
	}()

	fieldPayload := stressPayload(id, 257)
	writeErr := make(chan error, 1)
	go func() {
		_, err := fieldApp.Write(fieldPayload)
		writeErr <- err
	}()
	gotField := make([]byte, len(fieldPayload))
	if _, err := io.ReadFull(consumerApp, gotField); err != nil {
		return err
	}
	if string(gotField) != string(fieldPayload) {
		return fmt.Errorf("field_to_consumer payload changed")
	}
	if err := <-writeErr; err != nil {
		return err
	}

	consumerPayload := stressPayload(id+7919, 193)
	go func() {
		_, err := consumerApp.Write(consumerPayload)
		writeErr <- err
	}()
	gotConsumer := make([]byte, len(consumerPayload))
	if _, err := io.ReadFull(fieldApp, gotConsumer); err != nil {
		return err
	}
	if string(gotConsumer) != string(consumerPayload) {
		return fmt.Errorf("consumer_to_field payload changed")
	}
	if err := <-writeErr; err != nil {
		return err
	}

	_ = fieldApp.Close()
	_ = consumerApp.Close()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		return fmt.Errorf("copyDuplex did not stop")
	}
}

func TestStressSocketChurnNoFDLeak(t *testing.T) {
	if os.Getenv("RC_GATEWAY_STRESS") != "1" {
		t.Skip("set RC_GATEWAY_STRESS=1 to run production stress gate")
	}
	if _, err := os.Stat("/proc/self/fd"); err != nil {
		t.Skip("/proc/self/fd unavailable")
	}
	cycles := stressInt("RC_GATEWAY_CHURN_CYCLES", 1000)
	if cycles < 1 || cycles > 10000 {
		t.Fatalf("unsafe RC_GATEWAY_CHURN_CYCLES=%d", cycles)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fieldSource, fieldAddr := testListenSource(t, ctx)
	defer fieldSource.Close()
	consumerSource, consumerAddr := testListenSource(t, ctx)
	defer consumerSource.Close()

	baselineFD := countProcessFDs(t)
	baselineG := runtime.NumGoroutine()

	for i := 0; i < cycles; i++ {
		pairCh := make(chan pairResult, 1)
		go func() {
			f, c, err := acquirePair(ctx, fieldSource, consumerSource, "listen", "listen")
			pairCh <- pairResult{f, c, err}
		}()
		fieldPeer := dialTestPeer(t, fieldAddr)
		consumerPeer := dialTestPeer(t, consumerAddr)
		pair := waitPair(t, pairCh)

		done := make(chan error, 1)
		go func() {
			done <- copyDuplex(ctx, fmt.Sprintf("churn-%d", i), pair.field, pair.consumer, Hooks{}, time.Second, 20*time.Millisecond)
		}()
		b := []byte{byte(i), byte(i >> 8), 0xA5, 0x5A}
		if _, err := fieldPeer.Write(b); err != nil {
			t.Fatal(err)
		}
		got := make([]byte, len(b))
		if _, err := io.ReadFull(consumerPeer, got); err != nil {
			t.Fatal(err)
		}
		if string(got) != string(b) {
			t.Fatalf("cycle %d payload changed", i)
		}

		_ = fieldPeer.Close()
		_ = consumerPeer.Close()
		waitCopy(t, done)
		_ = pair.field.Close()
		_ = pair.consumer.Close()
	}

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	afterFD := countProcessFDs(t)
	afterG := runtime.NumGoroutine()
	if afterFD > baselineFD+8 {
		t.Fatalf("possible fd leak after %d churn cycles: baseline=%d after=%d", cycles, baselineFD, afterFD)
	}
	if afterG > baselineG+32 {
		t.Fatalf("possible goroutine leak after %d churn cycles: baseline=%d after=%d", cycles, baselineG, afterG)
	}
}

func stressPayload(seed, size int) []byte {
	p := make([]byte, size)
	for i := range p {
		p[i] = byte((seed*31 + i*17 + 13) & 0xff)
	}
	return p
}

func stressInt(name string, fallback int) int {
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

func countProcessFDs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}
