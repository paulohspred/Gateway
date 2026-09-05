package datagram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paulohspdev-cmyk/ProjetoGerador/gateway-umbrella/internal/transport/netutil"
)

type Endpoint struct {
	Mode         string
	Bind         string
	Address      string
	AllowedCIDRs []string
}

type SessionInfo struct {
	TunnelID  string
	SessionID string
	Peer      string
	Target    string
	OpenedAt  time.Time
}

type Hooks struct {
	OnOpen     func(SessionInfo)
	OnDatagram func(sessionID, direction string, n uint64)
	OnClose    func(SessionInfo, error)
	OnDrop     func(tunnelID, reason string)
}

type Tunnel struct {
	ID               string
	Field            Endpoint
	Consumer         Endpoint
	IdleTimeout      time.Duration
	MaxSessions      int
	MaxDatagramBytes int
	Logger           *slog.Logger
	Hooks            Hooks
	counter          atomic.Uint64
}

type udpSession struct {
	info      SessionInfo
	peer      *net.UDPAddr
	target    *net.UDPConn
	lastSeen  atomic.Int64
	closeOnce sync.Once
}

func (s *udpSession) touch() {
	s.lastSeen.Store(time.Now().UnixNano())
}

func (s *udpSession) lastSeenAt() time.Time {
	return time.Unix(0, s.lastSeen.Load())
}

func (s *udpSession) close() {
	s.closeOnce.Do(func() { _ = s.target.Close() })
}

type sessionTable struct {
	mu       sync.Mutex
	sessions map[string]*udpSession
}

func (t *Tunnel) Run(ctx context.Context) error {
	if t.ID == "" {
		return fmt.Errorf("udp tunnel id is required")
	}
	if t.IdleTimeout <= 0 {
		t.IdleTimeout = 60 * time.Second
	}
	if t.MaxSessions <= 0 {
		t.MaxSessions = 1024
	}
	if t.MaxDatagramBytes <= 0 {
		t.MaxDatagramBytes = 65507
	}

	var listener Endpoint
	var target Endpoint
	var outboundDirection string
	var inboundDirection string
	switch {
	case t.Field.Mode == "listen" && t.Consumer.Mode == "connect":
		listener = t.Field
		target = t.Consumer
		outboundDirection = "field_to_consumer"
		inboundDirection = "consumer_to_field"
	case t.Consumer.Mode == "listen" && t.Field.Mode == "connect":
		listener = t.Consumer
		target = t.Field
		outboundDirection = "consumer_to_field"
		inboundDirection = "field_to_consumer"
	default:
		return fmt.Errorf("udp tunnel %s requires exactly one listen endpoint and one connect endpoint", t.ID)
	}

	listenAddr, err := net.ResolveUDPAddr("udp", listener.Bind)
	if err != nil {
		return fmt.Errorf("udp tunnel %s resolve listen: %w", t.ID, err)
	}
	targetAddr, err := net.ResolveUDPAddr("udp", target.Address)
	if err != nil {
		return fmt.Errorf("udp tunnel %s resolve target: %w", t.ID, err)
	}
	allowed, err := netutil.ParsePrefixes(listener.AllowedCIDRs)
	if err != nil {
		return fmt.Errorf("udp tunnel %s allowlist: %w", t.ID, err)
	}

	listenerConn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return fmt.Errorf("udp tunnel %s listen: %w", t.ID, err)
	}
	defer listenerConn.Close()

	table := &sessionTable{sessions: make(map[string]*udpSession)}
	var workers sync.WaitGroup

	closeAll := func(reason error) {
		table.mu.Lock()
		all := make([]*udpSession, 0, len(table.sessions))
		for key, session := range table.sessions {
			delete(table.sessions, key)
			all = append(all, session)
		}
		table.mu.Unlock()
		for _, session := range all {
			session.close()
			if t.Hooks.OnClose != nil {
				t.Hooks.OnClose(session.info, reason)
			}
		}
	}

	removeSession := func(key string, expected *udpSession, reason error) bool {
		table.mu.Lock()
		current, ok := table.sessions[key]
		if !ok || current != expected {
			table.mu.Unlock()
			return false
		}
		delete(table.sessions, key)
		table.mu.Unlock()
		expected.close()
		if t.Hooks.OnClose != nil {
			t.Hooks.OnClose(expected.info, reason)
		}
		return true
	}

	ctxDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = listenerConn.Close()
		case <-ctxDone:
		}
	}()
	defer close(ctxDone)

	cleanupInterval := t.IdleTimeout / 2
	if cleanupInterval < 100*time.Millisecond {
		cleanupInterval = 100 * time.Millisecond
	}
	if cleanupInterval > 30*time.Second {
		cleanupInterval = 30 * time.Second
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				table.mu.Lock()
				stale := make([]struct {
					key string
					s   *udpSession
				}, 0)
				for key, session := range table.sessions {
					if now.Sub(session.lastSeenAt()) >= t.IdleTimeout {
						stale = append(stale, struct {
							key string
							s   *udpSession
						}{key: key, s: session})
					}
				}
				table.mu.Unlock()
				for _, item := range stale {
					removeSession(item.key, item.s, nil)
				}
			}
		}
	}()

	getOrCreate := func(peer *net.UDPAddr) (*udpSession, string, error) {
		key := peer.String()
		table.mu.Lock()
		if existing := table.sessions[key]; existing != nil {
			table.mu.Unlock()
			return existing, key, nil
		}
		if len(table.sessions) >= t.MaxSessions {
			table.mu.Unlock()
			return nil, key, fmt.Errorf("max sessions reached")
		}
		targetConn, err := net.DialUDP("udp", nil, targetAddr)
		if err != nil {
			table.mu.Unlock()
			return nil, key, err
		}
		peerCopy := &net.UDPAddr{IP: append(net.IP(nil), peer.IP...), Port: peer.Port, Zone: peer.Zone}
		now := time.Now().UTC()
		session := &udpSession{
			info: SessionInfo{
				TunnelID:  t.ID,
				SessionID: fmt.Sprintf("%s-%d-%d", t.ID, now.UnixNano(), t.counter.Add(1)),
				Peer:      peerCopy.String(),
				Target:    targetAddr.String(),
				OpenedAt:  now,
			},
			peer:   peerCopy,
			target: targetConn,
		}
		session.touch()
		table.sessions[key] = session
		table.mu.Unlock()

		if t.Hooks.OnOpen != nil {
			t.Hooks.OnOpen(session.info)
		}
		if t.Logger != nil {
			t.Logger.Info("udp session open", "tunnel", t.ID, "session", session.info.SessionID, "peer", session.info.Peer, "target", session.info.Target)
		}

		workers.Add(1)
		go func() {
			defer workers.Done()
			buf := make([]byte, t.MaxDatagramBytes+1)
			for {
				n, readErr := session.target.Read(buf)
				if n > t.MaxDatagramBytes {
					if t.Hooks.OnDrop != nil {
						t.Hooks.OnDrop(t.ID, "oversize_target")
					}
					continue
				}
				if n > 0 {
					session.touch()
					written, writeErr := listenerConn.WriteToUDP(buf[:n], session.peer)
					if written > 0 && t.Hooks.OnDatagram != nil {
						t.Hooks.OnDatagram(session.info.SessionID, inboundDirection, uint64(written))
					}
					if writeErr != nil || written != n {
						removeSession(key, session, writeErr)
						return
					}
				}
				if readErr != nil {
					if ctx.Err() == nil && !errors.Is(readErr, net.ErrClosed) {
						removeSession(key, session, readErr)
					} else {
						removeSession(key, session, nil)
					}
					return
				}
			}
		}()
		return session, key, nil
	}

	buf := make([]byte, t.MaxDatagramBytes+1)
	for {
		n, peer, readErr := listenerConn.ReadFromUDP(buf)
		if readErr != nil {
			if ctx.Err() != nil || errors.Is(readErr, net.ErrClosed) {
				closeAll(nil)
				workers.Wait()
				return nil
			}
			closeAll(readErr)
			workers.Wait()
			return fmt.Errorf("udp tunnel %s read: %w", t.ID, readErr)
		}
		if len(allowed) > 0 && !netutil.PeerAllowed(peer, allowed) {
			if t.Hooks.OnDrop != nil {
				t.Hooks.OnDrop(t.ID, "allowlist")
			}
			continue
		}
		if n > t.MaxDatagramBytes {
			if t.Hooks.OnDrop != nil {
				t.Hooks.OnDrop(t.ID, "oversize_peer")
			}
			continue
		}
		session, key, sessionErr := getOrCreate(peer)
		if sessionErr != nil {
			if t.Hooks.OnDrop != nil {
				t.Hooks.OnDrop(t.ID, "session_limit_or_dial")
			}
			continue
		}
		session.touch()
		written, writeErr := session.target.Write(buf[:n])
		if written > 0 && t.Hooks.OnDatagram != nil {
			t.Hooks.OnDatagram(session.info.SessionID, outboundDirection, uint64(written))
		}
		if writeErr != nil || written != n {
			removeSession(key, session, writeErr)
			if t.Hooks.OnDrop != nil {
				t.Hooks.OnDrop(t.ID, "target_write")
			}
		}
	}
}
