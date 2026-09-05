package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paulohspred/Gateway/internal/admin"
	"github.com/paulohspred/Gateway/internal/bridge"
	"github.com/paulohspred/Gateway/internal/config"
	"github.com/paulohspred/Gateway/internal/core"
	"github.com/paulohspred/Gateway/internal/datagram"
	"github.com/paulohspred/Gateway/internal/metrics"
	"github.com/paulohspred/Gateway/internal/provider/canbridge"
	"github.com/paulohspred/Gateway/internal/provider/serialbridge"
	"github.com/paulohspred/Gateway/internal/provider/usbhid"
	"github.com/paulohspred/Gateway/internal/systemdnotify"
)

const defaultMaxActivePairs = 1024

type Gateway struct {
	cfg        config.Config
	sessions   *core.SessionRegistry
	metrics    *metrics.Registry
	logger     *slog.Logger
	admin      *admin.Server
	pairActive atomic.Int64
	udpActive  atomic.Int64
	canActive  atomic.Int64
	hidActive  atomic.Int64
}

func New(cfg config.Config, logger *slog.Logger) *Gateway {
	m := metrics.New()
	sessions := core.NewSessionRegistry()
	a := &admin.Server{Bind: cfg.Admin.Bind, NodeID: cfg.NodeID, Sessions: sessions, Metrics: m}
	return &Gateway{cfg: cfg, sessions: sessions, metrics: m, logger: logger, admin: a}
}

func (g *Gateway) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	notifier := systemdnotify.FromEnv()
	defer func() {
		if err := notifier.Stopping(); err != nil && g.logger != nil {
			g.logger.Warn("systemd stopping notification failed", "error", err)
		}
	}()

	maxActivePairs := g.cfg.Limits.MaxActivePairs
	if maxActivePairs <= 0 {
		// Config loaded through LoadStrict is already defaulted. Keep New/Run
		// robust for tests and embedders that construct Config directly.
		maxActivePairs = defaultMaxActivePairs
	}
	pairLimiter, err := bridge.NewPairLimiter(maxActivePairs)
	if err != nil {
		return fmt.Errorf("stream pair limiter: %w", err)
	}

	var wg sync.WaitGroup
	componentCount := 1 + len(g.cfg.Tunnels) + len(g.cfg.SerialProviders) + len(g.cfg.USBHIDProviders) + len(g.cfg.CANProviders) + len(g.cfg.UDPTunnels)
	errCh := make(chan error, componentCount)
	readyCh := make(chan string, componentCount)

	g.admin.SetReady(false)
	g.metrics.Set("rc_gateway_ready", 0)
	g.metrics.Set("rc_gateway_configured_tunnels", int64(len(g.cfg.Tunnels)))
	g.metrics.Set("rc_gateway_configured_udp_tunnels", int64(len(g.cfg.UDPTunnels)))
	g.metrics.Set("rc_gateway_configured_serial_providers", int64(len(g.cfg.SerialProviders)))
	g.metrics.Set("rc_gateway_configured_usb_hid_providers", int64(len(g.cfg.USBHIDProviders)))
	g.metrics.Set("rc_gateway_configured_can_providers", int64(len(g.cfg.CANProviders)))
	g.metrics.Set("rc_gateway_max_active_pairs", int64(maxActivePairs))

	g.admin.OnListening = func() { readyCh <- "admin" }
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := g.admin.Run(runCtx); err != nil && runCtx.Err() == nil {
			sendErr(errCh, fmt.Errorf("admin: %w", err))
		}
	}()

	for _, p := range g.cfg.SerialProviders {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := serialbridge.Config{ID: p.ID, Socket: p.Socket, Device: p.Device, Standard: p.Standard, BaudRate: p.BaudRate, DataBits: p.DataBits, Parity: p.Parity, StopBits: p.StopBits, ReadTimeout: time.Duration(p.ReadTimeoutMS) * time.Millisecond, RTS: p.RTS, DTR: p.DTR}
			hooks := serialbridge.Hooks{OnReady: func(string) { readyCh <- "serial:" + p.ID }}
			if err := serialbridge.Run(runCtx, cfg, g.logger, hooks); err != nil && runCtx.Err() == nil {
				sendErr(errCh, fmt.Errorf("serial provider %s: %w", p.ID, err))
			}
		}()
	}

	for _, p := range g.cfg.USBHIDProviders {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := usbhid.Config{ID: p.ID, Socket: p.Socket, Device: p.Device, VendorID: p.VendorID, ProductID: p.ProductID, SerialNumber: p.SerialNumber, MaxReportBytes: p.MaxReportBytes, AllowWrite: p.AllowWrite}
			hooks := g.hidHooks(p.ID, usbHIDLabel(p))
			hooks.OnReady = func(string) { readyCh <- "usb-hid:" + p.ID }
			if err := usbhid.Run(runCtx, cfg, g.logger, hooks); err != nil && runCtx.Err() == nil {
				sendErr(errCh, fmt.Errorf("USB HID provider %s: %w", p.ID, err))
			}
		}()
	}

	for _, p := range g.cfg.CANProviders {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := canbridge.Config{ID: p.ID, Interface: p.Interface, Socket: p.Socket, EnableFD: p.EnableFD, ReceiveOwn: p.ReceiveOwn, AllowTransmit: p.AllowTransmit}
			hooks := g.canHooks(p.ID, p.Interface)
			hooks.OnReady = func(string) { readyCh <- "can:" + p.ID }
			if err := canbridge.Run(runCtx, cfg, g.logger, hooks); err != nil && runCtx.Err() == nil {
				sendErr(errCh, fmt.Errorf("CAN provider %s: %w", p.ID, err))
			}
		}()
	}

	for _, cfgTunnel := range g.cfg.Tunnels {
		cfgTunnel := cfgTunnel
		hooks := g.tunnelHooks(cfgTunnel.ID)
		hooks.OnReady = func(string) { readyCh <- "stream:" + cfgTunnel.ID }
		maxConcurrentPairs := cfgTunnel.MaxConcurrentPairs
		if maxConcurrentPairs <= 0 {
			maxConcurrentPairs = 1
		}
		g.metrics.Set("rc_gateway_tunnel_"+cfgTunnel.ID+"_max_concurrent_pairs", int64(maxConcurrentPairs))
		tunnel := &bridge.Tunnel{
			ID:                 cfgTunnel.ID,
			Field:              bridgeEndpoint("field", cfgTunnel.Field),
			Consumer:           bridgeEndpoint("consumer", cfgTunnel.Consumer),
			PacketFraming:      cfgTunnel.PacketFraming,
			Logger:             g.logger,
			Hooks:              hooks,
			PairTimeout:        time.Duration(cfgTunnel.PairTimeoutS) * time.Second,
			WriteTimeout:       time.Duration(cfgTunnel.WriteTimeoutS) * time.Second,
			DrainTimeout:       time.Duration(cfgTunnel.DrainTimeoutS) * time.Second,
			MaxConcurrentPairs: maxConcurrentPairs,
			GlobalPairLimiter:  pairLimiter,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tunnel.Run(runCtx); err != nil && runCtx.Err() == nil {
				sendErr(errCh, fmt.Errorf("tunnel %s: %w", cfgTunnel.ID, err))
			}
		}()
	}

	for _, cfgTunnel := range g.cfg.UDPTunnels {
		cfgTunnel := cfgTunnel
		hooks := g.udpHooks(cfgTunnel.ID)
		hooks.OnReady = func(string) { readyCh <- "udp:" + cfgTunnel.ID }
		tunnel := &datagram.Tunnel{
			ID:               cfgTunnel.ID,
			Field:            udpEndpoint(cfgTunnel.Field),
			Consumer:         udpEndpoint(cfgTunnel.Consumer),
			IdleTimeout:      time.Duration(cfgTunnel.IdleTimeoutS) * time.Second,
			MaxSessions:      cfgTunnel.MaxSessions,
			MaxDatagramBytes: cfgTunnel.MaxDatagramBytes,
			Logger:           g.logger,
			Hooks:            hooks,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tunnel.Run(runCtx); err != nil && runCtx.Err() == nil {
				sendErr(errCh, fmt.Errorf("udp tunnel %s: %w", cfgTunnel.ID, err))
			}
		}()
	}

	for ready := 0; ready < componentCount; ready++ {
		select {
		case <-readyCh:
		case <-ctx.Done():
			cancel()
			wg.Wait()
			return nil
		case err := <-errCh:
			cancel()
			wg.Wait()
			return err
		}
	}

	g.admin.SetReady(true)
	g.metrics.Set("rc_gateway_ready", 1)
	if g.logger != nil {
		g.logger.Info("bridge runtime ready", "nodeId", g.cfg.NodeID, "tunnels", len(g.cfg.Tunnels), "udpTunnels", len(g.cfg.UDPTunnels), "serialProviders", len(g.cfg.SerialProviders), "usbHidProviders", len(g.cfg.USBHIDProviders), "canProviders", len(g.cfg.CANProviders), "maxActivePairs", maxActivePairs)
	}
	if err := notifier.Ready(); err != nil {
		g.admin.SetReady(false)
		g.metrics.Set("rc_gateway_ready", 0)
		cancel()
		wg.Wait()
		return fmt.Errorf("systemd readiness notification: %w", err)
	}
	notifier.StartWatchdog(runCtx, func(err error) {
		if g.logger != nil {
			g.logger.Warn("systemd watchdog notification failed", "error", err)
		}
	})

	select {
	case <-ctx.Done():
		g.admin.SetReady(false)
		g.metrics.Set("rc_gateway_ready", 0)
		cancel()
		wg.Wait()
		return nil
	case err := <-errCh:
		g.admin.SetReady(false)
		g.metrics.Set("rc_gateway_ready", 0)
		cancel()
		wg.Wait()
		return err
	}
}

func bridgeEndpoint(name string, ep config.Endpoint) bridge.Endpoint {
	return bridge.Endpoint{Name: name, Mode: ep.Mode, Network: ep.Network, Bind: ep.Bind, Address: ep.Address, AllowedCIDRs: ep.AllowedCIDRs, DialTimeout: time.Duration(ep.DialTimeoutS) * time.Second, Reconnect: time.Duration(ep.ReconnectS) * time.Second, KeepAlive: time.Duration(ep.KeepAliveS) * time.Second, TLS: bridge.TLSOptions{Enabled: ep.TLS.Enabled, CAFile: ep.TLS.CAFile, CertFile: ep.TLS.CertFile, KeyFile: ep.TLS.KeyFile, ServerName: ep.TLS.ServerName, RequireClientCert: ep.TLS.RequireClientCert}}
}

func usbHIDLabel(p config.USBHIDProvider) string {
	if p.Device != "" {
		return p.Device
	}
	label := "hid:" + p.VendorID + ":" + p.ProductID
	if p.SerialNumber != "" {
		label += ":" + p.SerialNumber
	}
	return label
}

func udpEndpoint(ep config.UDPEndpoint) datagram.Endpoint {
	return datagram.Endpoint{Mode: ep.Mode, Bind: ep.Bind, Address: ep.Address, AllowedCIDRs: ep.AllowedCIDRs}
}

func (g *Gateway) tunnelHooks(tunnelID string) bridge.Hooks {
	prefix := "rc_gateway_tunnel_" + tunnelID
	var active atomic.Int64
	return bridge.Hooks{
		OnOpen: func(info bridge.PairInfo) {
			g.sessions.Open(core.Session{ID: info.PairID, ListenerID: info.TunnelID, Transport: "raw_bridge", RemoteAddr: info.FieldRemote, LocalAddr: info.ConsumerRemote, OpenedAt: info.OpenedAt, LastSeenAt: info.OpenedAt})
			g.metrics.Inc("rc_gateway_pairs_opened_total")
			g.metrics.Inc(prefix + "_pairs_opened_total")
			g.metrics.Set("rc_gateway_active_pairs", g.pairActive.Add(1))
			g.metrics.Set("rc_gateway_active_sessions", int64(g.sessions.Count()))
			g.metrics.Set(prefix+"_active", active.Add(1))
		},
		OnBytes: func(pairID, direction string, n uint64) {
			g.sessions.Touch(pairID, direction, int(n), time.Now().UTC())
			g.metrics.Add("rc_gateway_bytes_forwarded_total", n)
			g.metrics.Add(prefix+"_bytes_forwarded_total", n)
			g.metrics.Add(prefix+"_"+direction+"_bytes_total", n)
		},
		OnClose: func(info bridge.PairInfo, err error) {
			g.sessions.Close(info.PairID)
			g.metrics.Inc("rc_gateway_pairs_closed_total")
			g.metrics.Inc(prefix + "_pairs_closed_total")
			g.metrics.Set("rc_gateway_active_pairs", g.pairActive.Add(-1))
			g.metrics.Set("rc_gateway_active_sessions", int64(g.sessions.Count()))
			g.metrics.Set(prefix+"_active", active.Add(-1))
			if err != nil {
				g.metrics.Inc("rc_gateway_bridge_errors_total")
				g.metrics.Inc(prefix + "_errors_total")
			}
		},
		OnPairWaitTimeout: func(_ string) {
			g.metrics.Inc("rc_gateway_pair_wait_timeouts_total")
			g.metrics.Inc(prefix + "_pair_wait_timeouts_total")
		},
	}
}

func (g *Gateway) udpHooks(tunnelID string) datagram.Hooks {
	prefix := "rc_gateway_udp_tunnel_" + tunnelID
	var active atomic.Int64
	return datagram.Hooks{
		OnOpen: func(info datagram.SessionInfo) {
			g.sessions.Open(core.Session{ID: info.SessionID, ListenerID: info.TunnelID, Transport: "udp_bridge", RemoteAddr: info.Peer, LocalAddr: info.Target, OpenedAt: info.OpenedAt, LastSeenAt: info.OpenedAt})
			g.metrics.Inc("rc_gateway_udp_sessions_opened_total")
			g.metrics.Inc(prefix + "_sessions_opened_total")
			g.metrics.Set("rc_gateway_udp_active_sessions", g.udpActive.Add(1))
			g.metrics.Set("rc_gateway_active_sessions", int64(g.sessions.Count()))
			g.metrics.Set(prefix+"_active_sessions", active.Add(1))
		},
		OnDatagram: func(sessionID, direction string, n uint64) {
			g.sessions.Touch(sessionID, direction, int(n), time.Now().UTC())
			g.metrics.Inc("rc_gateway_udp_datagrams_total")
			g.metrics.Inc(prefix + "_datagrams_total")
			g.metrics.Add("rc_gateway_udp_bytes_total", n)
			g.metrics.Add(prefix+"_bytes_total", n)
			g.metrics.Add(prefix+"_"+direction+"_bytes_total", n)
		},
		OnClose: func(info datagram.SessionInfo, err error) {
			g.sessions.Close(info.SessionID)
			g.metrics.Inc("rc_gateway_udp_sessions_closed_total")
			g.metrics.Inc(prefix + "_sessions_closed_total")
			g.metrics.Set("rc_gateway_udp_active_sessions", g.udpActive.Add(-1))
			g.metrics.Set("rc_gateway_active_sessions", int64(g.sessions.Count()))
			g.metrics.Set(prefix+"_active_sessions", active.Add(-1))
			if err != nil {
				g.metrics.Inc("rc_gateway_udp_errors_total")
				g.metrics.Inc(prefix + "_errors_total")
			}
		},
		OnDrop: func(_ string, reason string) {
			g.metrics.Inc("rc_gateway_udp_drops_total")
			g.metrics.Inc(prefix + "_drops_total")
			g.metrics.Inc(prefix + "_drops_" + reason + "_total")
		},
	}
}

func (g *Gateway) hidHooks(providerID, device string) usbhid.Hooks {
	prefix := "rc_gateway_usb_hid_provider_" + providerID
	return usbhid.Hooks{
		OnOpen: func(sessionID string) {
			now := time.Now().UTC()
			g.sessions.Open(core.Session{ID: sessionID, ListenerID: providerID, Transport: "usb_hid", RemoteAddr: device, LocalAddr: "unixpacket", OpenedAt: now, LastSeenAt: now})
			g.metrics.Inc("rc_gateway_usb_hid_sessions_opened_total")
			g.metrics.Inc(prefix + "_sessions_opened_total")
			g.metrics.Set("rc_gateway_usb_hid_active_sessions", g.hidActive.Add(1))
			g.metrics.Set("rc_gateway_active_sessions", int64(g.sessions.Count()))
			g.metrics.Set(prefix+"_active", 1)
		},
		OnReport: func(sessionID, direction string, n uint64) {
			g.sessions.Touch(sessionID, direction, int(n), time.Now().UTC())
			g.metrics.Inc("rc_gateway_usb_hid_reports_total")
			g.metrics.Inc(prefix + "_reports_total")
			g.metrics.Add("rc_gateway_usb_hid_bytes_total", n)
			g.metrics.Add(prefix+"_bytes_total", n)
			g.metrics.Add(prefix+"_"+direction+"_bytes_total", n)
		},
		OnClose: func(sessionID string, err error) {
			g.sessions.Close(sessionID)
			g.metrics.Inc("rc_gateway_usb_hid_sessions_closed_total")
			g.metrics.Inc(prefix + "_sessions_closed_total")
			g.metrics.Set("rc_gateway_usb_hid_active_sessions", g.hidActive.Add(-1))
			g.metrics.Set("rc_gateway_active_sessions", int64(g.sessions.Count()))
			g.metrics.Set(prefix+"_active", 0)
			if err != nil {
				g.metrics.Inc("rc_gateway_usb_hid_errors_total")
				g.metrics.Inc(prefix + "_errors_total")
			}
		},
	}
}

func (g *Gateway) canHooks(providerID, interfaceName string) canbridge.Hooks {
	prefix := "rc_gateway_can_provider_" + providerID
	return canbridge.Hooks{
		OnOpen: func(sessionID string) {
			now := time.Now().UTC()
			g.sessions.Open(core.Session{ID: sessionID, ListenerID: providerID, Transport: "socketcan", RemoteAddr: interfaceName, LocalAddr: "unixpacket", OpenedAt: now, LastSeenAt: now})
			g.metrics.Inc("rc_gateway_can_sessions_opened_total")
			g.metrics.Inc(prefix + "_sessions_opened_total")
			g.metrics.Set("rc_gateway_can_active_sessions", g.canActive.Add(1))
			g.metrics.Set("rc_gateway_active_sessions", int64(g.sessions.Count()))
			g.metrics.Set(prefix+"_active", 1)
		},
		OnFrame: func(sessionID, direction string, n uint64) {
			g.sessions.Touch(sessionID, direction, int(n), time.Now().UTC())
			g.metrics.Inc("rc_gateway_can_frames_total")
			g.metrics.Inc(prefix + "_frames_total")
			g.metrics.Add("rc_gateway_can_bytes_total", n)
			g.metrics.Add(prefix+"_bytes_total", n)
			g.metrics.Add(prefix+"_"+direction+"_bytes_total", n)
		},
		OnClose: func(sessionID string, err error) {
			g.sessions.Close(sessionID)
			g.metrics.Inc("rc_gateway_can_sessions_closed_total")
			g.metrics.Inc(prefix + "_sessions_closed_total")
			g.metrics.Set("rc_gateway_can_active_sessions", g.canActive.Add(-1))
			g.metrics.Set("rc_gateway_active_sessions", int64(g.sessions.Count()))
			g.metrics.Set(prefix+"_active", 0)
			if err != nil {
				g.metrics.Inc("rc_gateway_can_errors_total")
				g.metrics.Inc(prefix + "_errors_total")
			}
		},
	}
}

func sendErr(ch chan<- error, err error) {
	select {
	case ch <- err:
	default:
	}
}
