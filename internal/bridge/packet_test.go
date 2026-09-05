package bridge

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnixpacketEndpointsPreserveMessageBoundaries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "packet.sock")
	serverSource, err := newSource(ctx, Endpoint{Mode: "listen", Network: "unixpacket", Bind: path})
	if err != nil {
		t.Fatal(err)
	}
	defer serverSource.Close()
	clientSource, err := newSource(ctx, Endpoint{Mode: "connect", Network: "unixpacket", Address: path, DialTimeout: time.Second, Reconnect: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer clientSource.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acquireErr := serverSource.Acquire(ctx)
		if acquireErr == nil {
			accepted <- conn
		}
	}()
	client, err := clientSource.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := <-accepted
	defer server.Close()

	packets := [][]byte{{0x01, 0x02, 0x03}, {0xaa, 0xbb}}
	for _, packet := range packets {
		if _, err := client.Write(packet); err != nil {
			t.Fatal(err)
		}
	}
	buf := make([]byte, 32)
	for i, want := range packets {
		n, err := server.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if string(buf[:n]) != string(want) {
			t.Fatalf("packet %d changed: got %x want %x", i, buf[:n], want)
		}
	}
}

func TestPacketToStreamLength32BEPreservesFrames(t *testing.T) {
	packetServer, packetClient := unixpacketPair(t)
	defer packetServer.Close()
	defer packetClient.Close()
	streamServer, streamClient := net.Pipe()
	defer streamServer.Close()
	defer streamClient.Close()

	done := make(chan error, 1)
	go func() {
		done <- copyPacketToStream("pair", "field_to_consumer", streamServer, packetServer, Hooks{}, time.Second)
	}()

	packets := [][]byte{{0x10, 0x20}, {0x30, 0x40, 0x50}}
	for _, packet := range packets {
		if _, err := packetClient.Write(packet); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range packets {
		header := make([]byte, 4)
		if _, err := io.ReadFull(streamClient, header); err != nil {
			t.Fatal(err)
		}
		length := int(binary.BigEndian.Uint32(header))
		if length != len(want) {
			t.Fatalf("frame %d length=%d want=%d", i, length, len(want))
		}
		got := make([]byte, length)
		if _, err := io.ReadFull(streamClient, got); err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("frame %d changed: got %x want %x", i, got, want)
		}
	}
	_ = packetClient.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("packet-to-stream copy did not stop")
	}
}

func TestStreamToPacketLength32BEDecodesOnePacketPerFrame(t *testing.T) {
	packetServer, packetClient := unixpacketPair(t)
	defer packetServer.Close()
	defer packetClient.Close()
	streamServer, streamClient := net.Pipe()
	defer streamServer.Close()
	defer streamClient.Close()

	done := make(chan error, 1)
	go func() {
		done <- copyStreamToPacket("pair", "consumer_to_field", packetServer, streamServer, Hooks{}, time.Second)
	}()

	packets := [][]byte{{0x01}, {0x02, 0x03, 0x04}}
	for _, packet := range packets {
		header := make([]byte, 4)
		binary.BigEndian.PutUint32(header, uint32(len(packet)))
		if _, err := streamClient.Write(append(header, packet...)); err != nil {
			t.Fatal(err)
		}
	}
	buf := make([]byte, 32)
	for i, want := range packets {
		n, err := packetClient.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		if string(buf[:n]) != string(want) {
			t.Fatalf("packet %d changed: got %x want %x", i, buf[:n], want)
		}
	}
	_ = streamClient.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream-to-packet copy did not stop")
	}
}

func unixpacketPair(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pair.sock")
	addr := &net.UnixAddr{Name: path, Net: "unixpacket"}
	ln, err := net.ListenUnix("unixpacket", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = os.Remove(path)
	})
	accepted := make(chan *net.UnixConn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.AcceptUnix()
		if acceptErr != nil {
			errCh <- acceptErr
			return
		}
		accepted <- conn
	}()
	client, err := net.DialUnix("unixpacket", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case server := <-accepted:
		return server, client
	case err := <-errCh:
		_ = client.Close()
		t.Fatal(err)
	case <-time.After(time.Second):
		_ = client.Close()
		t.Fatal("accept unixpacket timeout")
	}
	return nil, nil
}
