//go:build linux

package canbridge

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type fakeFrameDevice struct {
	reads  chan []byte
	writes chan []byte
	closed chan struct{}
	once   sync.Once
}

func newFakeFrameDevice() *fakeFrameDevice {
	return &fakeFrameDevice{reads: make(chan []byte, 4), writes: make(chan []byte, 4), closed: make(chan struct{})}
}

func (f *fakeFrameDevice) Read(p []byte) (int, error) {
	select {
	case frame := <-f.reads:
		return copy(p, frame), nil
	case <-f.closed:
		return 0, io.EOF
	}
}

func (f *fakeFrameDevice) Write(p []byte) (int, error) {
	select {
	case f.writes <- append([]byte(nil), p...):
		return len(p), nil
	case <-f.closed:
		return 0, io.ErrClosedPipe
	}
}

func (f *fakeFrameDevice) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func unixPacketPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "can.sock")
	addr := &net.UnixAddr{Name: path, Net: "unixpacket"}
	ln, err := net.ListenUnix("unixpacket", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(path) })
	acceptCh := make(chan *net.UnixConn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.AcceptUnix()
		if err != nil {
			errCh <- err
			return
		}
		acceptCh <- conn
	}()
	client, err := net.DialUnix("unixpacket", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case server := <-acceptCh:
		t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
		return server, client
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("unixpacket accept timeout")
	}
	return nil, nil
}

func TestValidFrameSize(t *testing.T) {
	for _, n := range []int{CANMTU, CANFDMTU} {
		if !validFrameSize(n) {
			t.Fatalf("expected %d to be valid", n)
		}
	}
	for _, n := range []int{0, 15, 17, 71, 73} {
		if validFrameSize(n) {
			t.Fatalf("expected %d to be invalid", n)
		}
	}
}

func TestBridgeFramesPreservesClassicAndFD(t *testing.T) {
	server, client := unixPacketPair(t)
	device := newFakeFrameDevice()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var directions []string
	hooks := Hooks{OnFrame: func(_ string, direction string, _ uint64) {
		mu.Lock()
		directions = append(directions, direction)
		mu.Unlock()
	}}
	done := make(chan error, 1)
	go func() { done <- bridgeFrames(ctx, server, device, true, "test", hooks) }()

	classic := make([]byte, CANMTU)
	for i := range classic {
		classic[i] = byte(i + 1)
	}
	fd := make([]byte, CANFDMTU)
	for i := range fd {
		fd[i] = byte(255 - i)
	}

	device.reads <- classic
	assertUnixPacketRead(t, client, classic)
	device.reads <- fd
	assertUnixPacketRead(t, client, fd)

	if _, err := client.Write(classic); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-device.writes:
		if string(got) != string(classic) {
			t.Fatalf("classic TX changed: got=%x want=%x", got, classic)
		}
	case <-time.After(time.Second):
		t.Fatal("classic TX timeout")
	}

	if _, err := client.Write(fd); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-device.writes:
		if string(got) != string(fd) {
			t.Fatal("FD TX changed")
		}
	case <-time.After(time.Second):
		t.Fatal("FD TX timeout")
	}

	cancel()
	_ = client.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("frame bridge did not stop")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(directions) != 4 {
		t.Fatalf("expected 4 frame hook calls, got %d (%v)", len(directions), directions)
	}
}

func TestBridgeFramesRejectsInvalidConsumerPacket(t *testing.T) {
	server, client := unixPacketPair(t)
	device := newFakeFrameDevice()
	done := make(chan error, 1)
	go func() { done <- bridgeFrames(context.Background(), server, device, true, "test", Hooks{}) }()
	if _, err := client.Write(make([]byte, 20)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected invalid frame size error")
		}
	case <-time.After(time.Second):
		t.Fatal("invalid frame was not rejected")
	}
}

func TestBridgeFramesBlocksTransmitByDefault(t *testing.T) {
	server, client := unixPacketPair(t)
	device := newFakeFrameDevice()
	done := make(chan error, 1)
	go func() { done <- bridgeFrames(context.Background(), server, device, false, "test", Hooks{}) }()
	if _, err := client.Write(make([]byte, CANMTU)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrTransmitDisabled) {
			t.Fatalf("expected transmit disabled error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("transmit policy was not enforced")
	}
	select {
	case <-device.writes:
		t.Fatal("frame reached CAN device while transmit disabled")
	default:
	}
}

func TestVCANRoundTripClassicAndFD(t *testing.T) {
	if os.Getenv("RC_GATEWAY_TEST_VCAN") != "1" {
		t.Skip("set RC_GATEWAY_TEST_VCAN=1 after creating vcan0")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	providerSocket := filepath.Join(t.TempDir(), "vcan-provider.sock")
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Config{ID: "vcan-test", Interface: "vcan0", Socket: providerSocket, EnableFD: true, AllowTransmit: true}, nil, Hooks{})
	}()
	consumer := dialUnixPacketRetry(t, providerSocket, 2*time.Second)
	defer consumer.Close()
	peerFD := openTestCANSocket(t, "vcan0", true)
	defer unix.Close(peerFD)

	classic := makeClassicFrame(0x123, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	if n, err := unix.Write(peerFD, classic); err != nil || n != len(classic) {
		t.Fatalf("write classic to vcan: n=%d err=%v", n, err)
	}
	assertUnixPacketRead(t, consumer, classic)

	fdFrame := makeFDFrame(0x456, 0x01, []byte("gateway-can-fd-roundtrip"))
	if n, err := unix.Write(peerFD, fdFrame); err != nil || n != len(fdFrame) {
		t.Fatalf("write FD to vcan: n=%d err=%v", n, err)
	}
	assertUnixPacketRead(t, consumer, fdFrame)

	returnClassic := makeClassicFrame(0x321, []byte{8, 7, 6, 5, 4, 3, 2, 1})
	if _, err := consumer.Write(returnClassic); err != nil {
		t.Fatal(err)
	}
	assertCANRead(t, peerFD, returnClassic)

	returnFD := makeFDFrame(0x654, 0x01, []byte("consumer-to-vcan-fd"))
	if _, err := consumer.Write(returnFD); err != nil {
		t.Fatal(err)
	}
	assertCANRead(t, peerFD, returnFD)

	cancel()
	_ = consumer.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("provider stop: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not stop")
	}
}

func dialUnixPacketRetry(t *testing.T, path string, timeout time.Duration) *net.UnixConn {
	t.Helper()
	deadline := time.Now().Add(timeout)
	addr := &net.UnixAddr{Name: path, Net: "unixpacket"}
	for time.Now().Before(deadline) {
		conn, err := net.DialUnix("unixpacket", nil, addr)
		if err == nil {
			return conn
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout dialing unixpacket %s", path)
	return nil
}

func openTestCANSocket(t *testing.T, interfaceName string, enableFD bool) int {
	t.Helper()
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.CAN_RAW)
	if err != nil {
		t.Fatal(err)
	}
	if enableFD {
		if err := unix.SetsockoptInt(fd, unix.SOL_CAN_RAW, unix.CAN_RAW_FD_FRAMES, 1); err != nil {
			_ = unix.Close(fd)
			t.Fatal(err)
		}
	}
	if err := unix.Bind(fd, &unix.SockaddrCAN{Ifindex: iface.Index}); err != nil {
		_ = unix.Close(fd)
		t.Fatal(err)
	}
	return fd
}

func makeClassicFrame(id uint32, data []byte) []byte {
	frame := make([]byte, CANMTU)
	binary.LittleEndian.PutUint32(frame[0:4], id)
	if len(data) > 8 {
		data = data[:8]
	}
	frame[4] = byte(len(data))
	copy(frame[8:], data)
	return frame
}

func makeFDFrame(id uint32, flags byte, data []byte) []byte {
	frame := make([]byte, CANFDMTU)
	binary.LittleEndian.PutUint32(frame[0:4], id)
	if len(data) > 64 {
		data = data[:64]
	}
	frame[4] = byte(len(data))
	frame[5] = flags
	copy(frame[8:], data)
	return frame
}

func assertUnixPacketRead(t *testing.T, conn net.Conn, want []byte) {
	t.Helper()
	buf := make([]byte, CANFDMTU+1)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(want) || string(buf[:n]) != string(want) {
		t.Fatalf("unixpacket frame mismatch: n=%d got=%x want=%x", n, buf[:n], want)
	}
}

func assertCANRead(t *testing.T, fd int, want []byte) {
	t.Helper()
	pollFD := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	ready, err := unix.Poll(pollFD, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if ready == 0 {
		t.Fatalf("timeout waiting for CAN frame %x", want)
	}
	buf := make([]byte, CANFDMTU)
	n, err := unix.Read(fd, buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(want) || string(buf[:n]) != string(want) {
		t.Fatalf("CAN frame mismatch: n=%d got=%x want=%x", n, buf[:n], want)
	}
}
