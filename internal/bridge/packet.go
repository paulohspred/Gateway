package bridge

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"syscall"
	"time"
)

const maxFramedPacketBytes = 64 * 1024

// copyPacketFramedDuplex bridges exactly one SOCK_SEQPACKET endpoint to a
// stream endpoint. Each packet is encoded on the stream as uint32 big-endian
// payload length followed by the unmodified payload. The reverse direction
// decodes the same framing and emits exactly one packet per frame.
func copyPacketFramedDuplex(ctx context.Context, pairID string, field, consumer net.Conn, fieldIsPacket bool, hooks Hooks, writeTimeout, drainTimeout time.Duration) error {
	if drainTimeout <= 0 {
		drainTimeout = 2 * time.Second
	}
	results := make(chan copyResult, 2)
	if fieldIsPacket {
		go func() {
			results <- copyResult{"field_to_consumer", copyPacketToStream(pairID, "field_to_consumer", consumer, field, hooks, writeTimeout)}
		}()
		go func() {
			results <- copyResult{"consumer_to_field", copyStreamToPacket(pairID, "consumer_to_field", field, consumer, hooks, writeTimeout)}
		}()
	} else {
		go func() {
			results <- copyResult{"field_to_consumer", copyStreamToPacket(pairID, "field_to_consumer", consumer, field, hooks, writeTimeout)}
		}()
		go func() {
			results <- copyResult{"consumer_to_field", copyPacketToStream(pairID, "consumer_to_field", field, consumer, hooks, writeTimeout)}
		}()
	}

	var first copyResult
	select {
	case <-ctx.Done():
		_ = field.Close()
		_ = consumer.Close()
		return nil
	case first = <-results:
	}
	if first.err != nil {
		_ = field.Close()
		_ = consumer.Close()
		select {
		case <-results:
		case <-time.After(drainTimeout):
		}
		return first.err
	}

	timer := time.NewTimer(drainTimeout)
	defer timer.Stop()
	select {
	case second := <-results:
		return second.err
	case <-ctx.Done():
		_ = field.Close()
		_ = consumer.Close()
		return nil
	case <-timer.C:
		_ = field.Close()
		_ = consumer.Close()
		select {
		case second := <-results:
			return second.err
		default:
			return nil
		}
	}
}

func copyPacketToStream(pairID, direction string, dst, src net.Conn, hooks Hooks, writeTimeout time.Duration) error {
	buf := make([]byte, maxFramedPacketBytes)
	header := make([]byte, 4)
	for {
		n, rerr := readPacket(src, buf)
		if n > 0 {
			binary.BigEndian.PutUint32(header, uint32(n))
			if writeTimeout > 0 {
				_ = dst.SetWriteDeadline(time.Now().Add(writeTimeout))
			}
			_, herr := writeAll(dst, header)
			written, werr := 0, herr
			if herr == nil {
				written, werr = writeAll(dst, buf[:n])
			}
			if writeTimeout > 0 {
				_ = dst.SetWriteDeadline(time.Time{})
			}
			if written > 0 && hooks.OnBytes != nil {
				hooks.OnBytes(pairID, direction, uint64(written))
			}
			if werr != nil {
				return normalizeCopyError(werr)
			}
			if written != n {
				return io.ErrShortWrite
			}
		}
		if rerr != nil {
			err := normalizeCopyError(rerr)
			if err == nil {
				closeWrite(dst)
			}
			return err
		}
	}
}

func copyStreamToPacket(pairID, direction string, dst, src net.Conn, hooks Hooks, writeTimeout time.Duration) error {
	header := make([]byte, 4)
	buf := make([]byte, maxFramedPacketBytes)
	for {
		if _, err := io.ReadFull(src, header); err != nil {
			return normalizeCopyError(err)
		}
		length := int(binary.BigEndian.Uint32(header))
		if length <= 0 || length > maxFramedPacketBytes {
			return fmt.Errorf("packet frame length %d outside 1..%d", length, maxFramedPacketBytes)
		}
		if _, err := io.ReadFull(src, buf[:length]); err != nil {
			return normalizeCopyError(err)
		}
		if writeTimeout > 0 {
			_ = dst.SetWriteDeadline(time.Now().Add(writeTimeout))
		}
		written, err := dst.Write(buf[:length])
		if writeTimeout > 0 {
			_ = dst.SetWriteDeadline(time.Time{})
		}
		if err != nil {
			return normalizeCopyError(err)
		}
		if written != length {
			return io.ErrShortWrite
		}
		if hooks.OnBytes != nil {
			hooks.OnBytes(pairID, direction, uint64(written))
		}
	}
}

func readPacket(src net.Conn, buf []byte) (int, error) {
	if unixConn, ok := src.(*net.UnixConn); ok {
		n, _, flags, _, err := unixConn.ReadMsgUnix(buf, nil)
		if flags&syscall.MSG_TRUNC != 0 {
			return n, fmt.Errorf("unixpacket payload exceeds %d bytes", len(buf))
		}
		return n, err
	}
	return src.Read(buf)
}
