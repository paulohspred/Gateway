package bridge

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/paulohspdev-cmyk/ProjetoGerador/gateway-umbrella/internal/transport/netutil"
)

type TLSOptions struct {
	Enabled           bool
	CAFile            string
	CertFile          string
	KeyFile           string
	ServerName        string
	RequireClientCert bool
}

type Endpoint struct {
	Name         string
	Mode         string
	Network      string
	Bind         string
	Address      string
	AllowedCIDRs []string
	DialTimeout  time.Duration
	Reconnect    time.Duration
	KeepAlive    time.Duration
	TLS          TLSOptions
}

type PairInfo struct {
	TunnelID       string
	PairID         string
	FieldLocal     string
	FieldRemote    string
	ConsumerLocal  string
	ConsumerRemote string
	OpenedAt       time.Time
}

type Hooks struct {
	OnOpen            func(PairInfo)
	OnBytes           func(pairID, direction string, n uint64)
	OnClose           func(PairInfo, error)
	OnPairWaitTimeout func(tunnelID string)
}

type Tunnel struct {
	ID           string
	Field        Endpoint
	Consumer     Endpoint
	Logger       *slog.Logger
	Hooks        Hooks
	PairTimeout  time.Duration
	WriteTimeout time.Duration
	DrainTimeout time.Duration
	counter      atomic.Uint64
}

type connectionSource interface {
	Acquire(context.Context) (net.Conn, error)
	Close() error
}

func (t *Tunnel) Run(ctx context.Context) error {
	if t.ID == "" {
		return fmt.Errorf("tunnel id is required")
	}
	if t.PairTimeout <= 0 {
		t.PairTimeout = 30 * time.Second
	}
	if t.WriteTimeout <= 0 {
		t.WriteTimeout = 30 * time.Second
	}
	if t.DrainTimeout <= 0 {
		t.DrainTimeout = 2 * time.Second
	}
	field, err := newSource(ctx, t.Field)
	if err != nil {
		return fmt.Errorf("tunnel %s field: %w", t.ID, err)
	}
	defer field.Close()
	consumer, err := newSource(ctx, t.Consumer)
	if err != nil {
		return fmt.Errorf("tunnel %s consumer: %w", t.ID, err)
	}
	defer consumer.Close()
	for {
		if ctx.Err() != nil {
			return nil
		}
		pairCtx, cancel := context.WithTimeout(ctx, t.PairTimeout)
		fieldConn, consumerConn, err := acquirePair(pairCtx, field, consumer, t.Field.Mode, t.Consumer.Mode)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				if t.Hooks.OnPairWaitTimeout != nil {
					t.Hooks.OnPairWaitTimeout(t.ID)
				}
				if t.Logger != nil {
					t.Logger.Warn("bridge pair wait timed out", "tunnel", t.ID, "timeout", t.PairTimeout)
				}
				continue
			}
			return fmt.Errorf("tunnel %s acquire pair: %w", t.ID, err)
		}
		pairID := fmt.Sprintf("%s-%d-%d", t.ID, time.Now().UnixNano(), t.counter.Add(1))
		info := PairInfo{TunnelID: t.ID, PairID: pairID, FieldLocal: addr(fieldConn.LocalAddr()), FieldRemote: addr(fieldConn.RemoteAddr()), ConsumerLocal: addr(consumerConn.LocalAddr()), ConsumerRemote: addr(consumerConn.RemoteAddr()), OpenedAt: time.Now().UTC()}
		if t.Hooks.OnOpen != nil {
			t.Hooks.OnOpen(info)
		}
		if t.Logger != nil {
			t.Logger.Info("bridge pair open", "tunnel", t.ID, "pair", pairID, "fieldRemote", info.FieldRemote, "consumerRemote", info.ConsumerRemote)
		}
		err = copyDuplex(ctx, pairID, fieldConn, consumerConn, t.Hooks, t.WriteTimeout, t.DrainTimeout)
		_ = fieldConn.Close()
		_ = consumerConn.Close()
		if t.Hooks.OnClose != nil {
			t.Hooks.OnClose(info, err)
		}
		if t.Logger != nil {
			attrs := []any{"tunnel", t.ID, "pair", pairID}
			if err != nil {
				attrs = append(attrs, "error", err)
			}
			t.Logger.Info("bridge pair closed", attrs...)
		}
	}
}

func acquirePair(ctx context.Context, field, consumer connectionSource, fieldMode, consumerMode string) (net.Conn, net.Conn, error) {
	switch {
	case fieldMode == "listen" && consumerMode == "connect":
		return acquireTriggered(ctx, "field", field, "consumer", consumer)
	case fieldMode == "connect" && consumerMode == "listen":
		return acquireTriggered(ctx, "consumer", consumer, "field", field)
	default:
		return acquireConcurrent(ctx, field, consumer)
	}
}
func acquireTriggered(ctx context.Context, firstName string, first connectionSource, secondName string, second connectionSource) (net.Conn, net.Conn, error) {
	firstConn, err := first.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", firstName, err)
	}
	secondConn, err := second.Acquire(ctx)
	if err != nil {
		_ = firstConn.Close()
		return nil, nil, fmt.Errorf("%s: %w", secondName, err)
	}
	if firstName == "field" {
		return firstConn, secondConn, nil
	}
	return secondConn, firstConn, nil
}
func acquireConcurrent(ctx context.Context, field, consumer connectionSource) (net.Conn, net.Conn, error) {
	type result struct {
		name string
		conn net.Conn
		err  error
	}
	ch := make(chan result, 2)
	go func() { c, e := field.Acquire(ctx); ch <- result{"field", c, e} }()
	go func() { c, e := consumer.Acquire(ctx); ch <- result{"consumer", c, e} }()
	var fc, cc net.Conn
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			if fc != nil {
				_ = fc.Close()
			}
			if cc != nil {
				_ = cc.Close()
			}
			return nil, nil, ctx.Err()
		case r := <-ch:
			if r.err != nil {
				if fc != nil {
					_ = fc.Close()
				}
				if cc != nil {
					_ = cc.Close()
				}
				return nil, nil, fmt.Errorf("%s: %w", r.name, r.err)
			}
			if r.name == "field" {
				fc = r.conn
			} else {
				cc = r.conn
			}
		}
	}
	return fc, cc, nil
}

type copyResult struct {
	direction string
	err       error
}

func copyDuplex(ctx context.Context, pairID string, field, consumer net.Conn, hooks Hooks, writeTimeout, drainTimeout time.Duration) error {
	if drainTimeout <= 0 {
		drainTimeout = 2 * time.Second
	}
	results := make(chan copyResult, 2)
	go func() {
		results <- copyResult{"field_to_consumer", copyDirection(pairID, "field_to_consumer", consumer, field, hooks, writeTimeout)}
	}()
	go func() {
		results <- copyResult{"consumer_to_field", copyDirection(pairID, "consumer_to_field", field, consumer, hooks, writeTimeout)}
	}()
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
func copyDirection(pairID, direction string, dst, src net.Conn, hooks Hooks, writeTimeout time.Duration) error {
	buf := make([]byte, 64*1024)
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			if writeTimeout > 0 {
				_ = dst.SetWriteDeadline(time.Now().Add(writeTimeout))
			}
			written, werr := writeAll(dst, buf[:n])
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
func writeAll(dst io.Writer, p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		n, err := dst.Write(p)
		total += n
		p = p[n:]
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}
func closeWrite(conn net.Conn) {
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}
func normalizeCopyError(err error) error {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func newSource(ctx context.Context, ep Endpoint) (connectionSource, error) {
	if ep.Network == "" {
		ep.Network = "tcp"
	}
	switch ep.Network {
	case "tcp":
		return newTCPSource(ctx, ep)
	case "unix":
		return newUnixSource(ctx, ep)
	default:
		return nil, fmt.Errorf("unsupported network %q", ep.Network)
	}
}
func newTCPSource(ctx context.Context, ep Endpoint) (connectionSource, error) {
	tlsConfig, err := buildTLSConfig(ep)
	if err != nil {
		return nil, err
	}
	switch ep.Mode {
	case "listen":
		allowed, err := netutil.ParsePrefixes(ep.AllowedCIDRs)
		if err != nil {
			return nil, err
		}
		ln, err := net.Listen("tcp", ep.Bind)
		if err != nil {
			return nil, err
		}
		s := &listenSource{ln: ln, allowed: allowed, keepAlive: ep.KeepAlive, tlsConfig: tlsConfig}
		go func() { <-ctx.Done(); _ = s.Close() }()
		return s, nil
	case "connect":
		if ep.DialTimeout <= 0 {
			ep.DialTimeout = 10 * time.Second
		}
		if ep.Reconnect <= 0 {
			ep.Reconnect = 5 * time.Second
		}
		if ep.KeepAlive <= 0 {
			ep.KeepAlive = 30 * time.Second
		}
		return &dialSource{network: "tcp", address: ep.Address, timeout: ep.DialTimeout, reconnect: ep.Reconnect, keepAlive: ep.KeepAlive, tlsConfig: tlsConfig}, nil
	default:
		return nil, fmt.Errorf("unsupported mode %q", ep.Mode)
	}
}

type listenSource struct {
	ln        net.Listener
	allowed   []netip.Prefix
	keepAlive time.Duration
	tlsConfig *tls.Config
}

func (s *listenSource) Acquire(ctx context.Context) (net.Conn, error) {
	if dl, ok := ctx.Deadline(); ok {
		if l, ok := s.ln.(interface{ SetDeadline(time.Time) error }); ok {
			_ = l.SetDeadline(dl)
			defer l.SetDeadline(time.Time{})
		}
	}
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return nil, context.DeadlineExceeded
			}
			if errors.Is(err, net.ErrClosed) {
				return nil, ctx.Err()
			}
			return nil, err
		}
		if len(s.allowed) > 0 && !netutil.PeerAllowed(conn.RemoteAddr(), s.allowed) {
			_ = conn.Close()
			continue
		}
		configureTCP(conn, s.keepAlive)
		if s.tlsConfig != nil {
			tlsConn := tls.Server(conn, s.tlsConfig.Clone())
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				_ = conn.Close()
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				continue
			}
			return tlsConn, nil
		}
		return conn, nil
	}
}
func (s *listenSource) Close() error { return s.ln.Close() }

type dialSource struct {
	network, address              string
	timeout, reconnect, keepAlive time.Duration
	tlsConfig                     *tls.Config
}

func (s *dialSource) Acquire(ctx context.Context) (net.Conn, error) {
	d := net.Dialer{Timeout: s.timeout, KeepAlive: s.keepAlive}
	for {
		conn, err := d.DialContext(ctx, s.network, s.address)
		if err == nil {
			configureTCP(conn, s.keepAlive)
			if s.tlsConfig != nil {
				tlsConn := tls.Client(conn, s.tlsConfig.Clone())
				if err := tlsConn.HandshakeContext(ctx); err == nil {
					return tlsConn, nil
				}
				_ = conn.Close()
			} else {
				return conn, nil
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		timer := time.NewTimer(s.reconnect)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
func (s *dialSource) Close() error { return nil }
func configureTCP(conn net.Conn, keepAlive time.Duration) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetNoDelay(true)
	if keepAlive > 0 {
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(keepAlive)
	}
}
func addr(a net.Addr) string {
	if a == nil {
		return ""
	}
	return a.String()
}
