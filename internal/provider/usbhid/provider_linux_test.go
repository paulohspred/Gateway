//go:build linux

package usbhid

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type fakeDevice struct {
	mu       sync.Mutex
	readData []byte
	readDone bool
	writes   [][]byte
}

func (d *fakeDevice) Read(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.readDone {
		return 0, io.EOF
	}
	d.readDone = true
	return copy(p, d.readData), nil
}

func (d *fakeDevice) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writes = append(d.writes, append([]byte(nil), p...))
	return len(p), nil
}

func (d *fakeDevice) Close() error { return nil }

func TestCopyReportsToConsumerPreservesReport(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	dev := &fakeDevice{readData: []byte{0x00, 0x11, 0x22, 0x33}}
	done := make(chan error, 1)
	go func() {
		done <- copyReportsToConsumer(server, dev, 64, "session", Hooks{})
	}()

	got := make([]byte, 4)
	if _, err := io.ReadFull(client, got); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !bytes.Equal(got, dev.readData) {
		t.Fatalf("report changed: got %x want %x", got, dev.readData)
	}
	_ = server.Close()
	if err := <-done; err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
		t.Fatalf("copyReportsToConsumer: %v", err)
	}
}

func TestCopyReportsToDeviceRejectsWriteWhenDisabled(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	dev := &fakeDevice{}
	done := make(chan error, 1)
	go func() {
		done <- copyReportsToDevice(dev, server, false, 64, "session", Hooks{})
	}()
	if _, err := client.Write([]byte{0x01, 0x02}); err != nil {
		t.Fatalf("write client report: %v", err)
	}
	if err := <-done; !errors.Is(err, ErrWriteDisabled) {
		t.Fatalf("expected ErrWriteDisabled, got %v", err)
	}
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if len(dev.writes) != 0 {
		t.Fatalf("device received %d writes while disabled", len(dev.writes))
	}
}

func TestRemoveStaleSocketRefusesRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.sock")
	if err := os.WriteFile(path, []byte("do not delete"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(path); err == nil {
		t.Fatal("expected regular file rejection")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("regular file was removed: %v", err)
	}
}
