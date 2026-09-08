package bridge

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestRapidSCADAModbusTCPReadWriteAndExceptionTransparency(t *testing.T) {
	vectors := []struct {
		name     string
		request  []byte
		response []byte
	}{
		{
			name:     "read-holding-registers",
			request:  []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x02},
			response: []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x07, 0x01, 0x03, 0x04, 0x00, 0x0a, 0x00, 0x14},
		},
		{
			name:     "write-single-register",
			request:  []byte{0x00, 0x02, 0x00, 0x00, 0x00, 0x06, 0x01, 0x06, 0x00, 0x01, 0x00, 0x64},
			response: []byte{0x00, 0x02, 0x00, 0x00, 0x00, 0x06, 0x01, 0x06, 0x00, 0x01, 0x00, 0x64},
		},
		{
			name:     "write-multiple-registers",
			request:  []byte{0x00, 0x03, 0x00, 0x00, 0x00, 0x0b, 0x01, 0x10, 0x00, 0x10, 0x00, 0x02, 0x04, 0x00, 0x64, 0x00, 0xc8},
			response: []byte{0x00, 0x03, 0x00, 0x00, 0x00, 0x06, 0x01, 0x10, 0x00, 0x10, 0x00, 0x02},
		},
		{
			name:     "exception-response",
			request:  []byte{0x00, 0x04, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0xff, 0xff, 0x00, 0x01},
			response: []byte{0x00, 0x04, 0x00, 0x00, 0x00, 0x03, 0x01, 0x83, 0x02},
		},
	}

	for _, tc := range vectors {
		t.Run(tc.name, func(t *testing.T) {
			field, consumer := newRapidSCADADuplex(t)
			rapidSCADAExchangeExact(t, consumer, field, tc.request)
			rapidSCADAExchangeExact(t, field, consumer, tc.response)
		})
	}
}

func TestRapidSCADARTUOverTCPPreservesCRCAndCommands(t *testing.T) {
	field, consumer := newRapidSCADADuplex(t)
	frames := []struct {
		request  []byte
		response []byte
	}{
		{
			request:  rapidSCADARTUFrame([]byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x02}),
			response: rapidSCADARTUFrame([]byte{0x01, 0x03, 0x04, 0x00, 0x0a, 0x00, 0x14}),
		},
		{
			request:  rapidSCADARTUFrame([]byte{0x01, 0x06, 0x00, 0x01, 0x00, 0x64}),
			response: rapidSCADARTUFrame([]byte{0x01, 0x06, 0x00, 0x01, 0x00, 0x64}),
		},
		{
			request:  rapidSCADARTUFrame([]byte{0x01, 0x10, 0x00, 0x10, 0x00, 0x02, 0x04, 0x00, 0x64, 0x00, 0xc8}),
			response: rapidSCADARTUFrame([]byte{0x01, 0x10, 0x00, 0x10, 0x00, 0x02}),
		},
	}

	for i, frame := range frames {
		if !rapidSCADAValidRTUCRC(frame.request) || !rapidSCADAValidRTUCRC(frame.response) {
			t.Fatalf("invalid test vector CRC at index %d", i)
		}
		rapidSCADAExchangeExact(t, consumer, field, frame.request)
		rapidSCADAExchangeExact(t, field, consumer, frame.response)
	}
}

func TestRapidSCADARS485SharedConnectionFiveUnitIDs(t *testing.T) {
	field, consumer := newRapidSCADADuplex(t)

	for unitID := byte(1); unitID <= 5; unitID++ {
		request := rapidSCADARTUFrame([]byte{unitID, 0x03, 0x00, 0x00, 0x00, 0x01})
		response := rapidSCADARTUFrame([]byte{unitID, 0x03, 0x02, 0x00, unitID})
		rapidSCADAExchangeExact(t, consumer, field, request)
		rapidSCADAExchangeExact(t, field, consumer, response)
	}
}

func TestRapidSCADATCPFragmentationAndCoalescingPreserveStream(t *testing.T) {
	field, consumer := newRapidSCADADuplex(t)
	first := rapidSCADATCPReadRequest(0x0101, 1, 0, 1)
	second := rapidSCADATCPReadRequest(0x0102, 2, 10, 2)
	batch := append(append([]byte(nil), first...), second...)

	deadline := time.Now().Add(3 * time.Second)
	_ = field.SetReadDeadline(deadline)
	_ = consumer.SetWriteDeadline(deadline)
	errCh := make(chan error, 1)
	go func() {
		for i := 0; i < len(batch); {
			chunk := 1 + (i % 5)
			if i+chunk > len(batch) {
				chunk = len(batch) - i
			}
			if _, err := consumer.Write(batch[i : i+chunk]); err != nil {
				errCh <- err
				return
			}
			i += chunk
		}
		errCh <- nil
	}()

	got := make([]byte, len(batch))
	if _, err := io.ReadFull(field, got); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, batch) {
		t.Fatalf("fragmented/coalesced stream changed: got %x want %x", got, batch)
	}
}

func TestRapidSCADASustained1000PollCycles(t *testing.T) {
	field, consumer := newRapidSCADADuplex(t)
	deadline := time.Now().Add(15 * time.Second)
	_ = field.SetDeadline(deadline)
	_ = consumer.SetDeadline(deadline)

	deviceDone := make(chan error, 1)
	go func() {
		request := make([]byte, 12)
		for i := 0; i < 1000; i++ {
			if _, err := io.ReadFull(field, request); err != nil {
				deviceDone <- fmt.Errorf("device read cycle %d: %w", i, err)
				return
			}
			if binary.BigEndian.Uint16(request[2:4]) != 0 || binary.BigEndian.Uint16(request[4:6]) != 6 || request[7] != 0x03 {
				deviceDone <- fmt.Errorf("unexpected Modbus TCP request at cycle %d: %x", i, request)
				return
			}
			value := uint16(i)
			response := make([]byte, 11)
			copy(response[0:2], request[0:2])
			binary.BigEndian.PutUint16(response[4:6], 5)
			response[6] = request[6]
			response[7] = 0x03
			response[8] = 0x02
			binary.BigEndian.PutUint16(response[9:11], value)
			if _, err := field.Write(response); err != nil {
				deviceDone <- fmt.Errorf("device write cycle %d: %w", i, err)
				return
			}
		}
		deviceDone <- nil
	}()

	response := make([]byte, 11)
	for i := 0; i < 1000; i++ {
		transactionID := uint16(i + 1)
		request := rapidSCADATCPReadRequest(transactionID, 1, uint16(i%100), 1)
		if _, err := consumer.Write(request); err != nil {
			t.Fatalf("Rapid SCADA write cycle %d: %v", i, err)
		}
		if _, err := io.ReadFull(consumer, response); err != nil {
			t.Fatalf("Rapid SCADA read cycle %d: %v", i, err)
		}
		if binary.BigEndian.Uint16(response[0:2]) != transactionID {
			t.Fatalf("transaction id changed at cycle %d: %x", i, response)
		}
		if response[6] != 1 || response[7] != 0x03 || response[8] != 0x02 {
			t.Fatalf("invalid response at cycle %d: %x", i, response)
		}
		if got := binary.BigEndian.Uint16(response[9:11]); got != uint16(i) {
			t.Fatalf("value changed at cycle %d: got %d", i, got)
		}
	}

	if err := <-deviceDone; err != nil {
		t.Fatal(err)
	}
}

func newRapidSCADADuplex(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	fieldApp, fieldGateway := net.Pipe()
	consumerGateway, consumerApp := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- copyDuplex(context.Background(), "rapid-scada-contract", fieldGateway, consumerGateway, Hooks{}, 3*time.Second, 100*time.Millisecond)
	}()

	t.Cleanup(func() {
		_ = fieldApp.Close()
		_ = consumerApp.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Rapid SCADA duplex stopped with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Rapid SCADA duplex did not stop")
		}
	})
	return fieldApp, consumerApp
}

func rapidSCADAExchangeExact(t *testing.T, src, dst net.Conn, payload []byte) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	_ = src.SetWriteDeadline(deadline)
	_ = dst.SetReadDeadline(deadline)
	if _, err := src.Write(payload); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(dst, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload changed: got %x want %x", got, payload)
	}
	_ = src.SetWriteDeadline(time.Time{})
	_ = dst.SetReadDeadline(time.Time{})
}

func rapidSCADATCPReadRequest(transactionID uint16, unitID byte, address, quantity uint16) []byte {
	request := make([]byte, 12)
	binary.BigEndian.PutUint16(request[0:2], transactionID)
	binary.BigEndian.PutUint16(request[2:4], 0)
	binary.BigEndian.PutUint16(request[4:6], 6)
	request[6] = unitID
	request[7] = 0x03
	binary.BigEndian.PutUint16(request[8:10], address)
	binary.BigEndian.PutUint16(request[10:12], quantity)
	return request
}

func rapidSCADARTUFrame(payload []byte) []byte {
	frame := append([]byte(nil), payload...)
	crc := rapidSCADAModbusCRC16(frame)
	return append(frame, byte(crc), byte(crc>>8))
}

func rapidSCADAValidRTUCRC(frame []byte) bool {
	if len(frame) < 3 {
		return false
	}
	payload := frame[:len(frame)-2]
	want := rapidSCADAModbusCRC16(payload)
	got := uint16(frame[len(frame)-2]) | uint16(frame[len(frame)-1])<<8
	return got == want
}

func rapidSCADAModbusCRC16(data []byte) uint16 {
	crc := uint16(0xffff)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xa001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
