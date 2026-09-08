package bridge

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

type unixListenSource struct {
	*listenSource
	path string
}

func (s *unixListenSource) Close() error {
	err := s.listenSource.Close()
	_ = os.Remove(s.path)
	return err
}

func newUnixSource(ctx context.Context, ep Endpoint) (connectionSource, error) {
	if ep.TLS.Enabled || ep.TLS.CAFile != "" || ep.TLS.CertFile != "" || ep.TLS.KeyFile != "" || ep.TLS.ServerName != "" || ep.TLS.RequireClientCert {
		return nil, fmt.Errorf("TLS options are not valid on unix endpoints")
	}
	network := ep.Network
	if network == "" {
		network = "unix"
	}
	if network != "unix" && network != "unixpacket" {
		return nil, fmt.Errorf("unsupported unix network %q", network)
	}
	switch ep.Mode {
	case "listen":
		if !filepath.IsAbs(ep.Bind) {
			return nil, fmt.Errorf("%s listen path must be absolute", network)
		}
		if err := os.MkdirAll(filepath.Dir(ep.Bind), 0o750); err != nil {
			return nil, fmt.Errorf("create unix socket dir: %w", err)
		}
		if err := removeStaleSocket(ep.Bind); err != nil {
			return nil, err
		}
		ln, err := net.Listen(network, ep.Bind)
		if err != nil {
			return nil, err
		}
		if err := os.Chmod(ep.Bind, 0o660); err != nil {
			_ = ln.Close()
			_ = os.Remove(ep.Bind)
			return nil, fmt.Errorf("chmod unix socket: %w", err)
		}
		base := &listenSource{ln: ln}
		s := &unixListenSource{listenSource: base, path: ep.Bind}
		go func() { <-ctx.Done(); _ = s.Close() }()
		return s, nil
	case "connect":
		if !filepath.IsAbs(ep.Address) {
			return nil, fmt.Errorf("%s connect path must be absolute", network)
		}
		if ep.DialTimeout <= 0 {
			ep.DialTimeout = 10 * time.Second
		}
		if ep.Reconnect <= 0 {
			ep.Reconnect = 5 * time.Second
		}
		return &dialSource{network: network, address: ep.Address, timeout: ep.DialTimeout, reconnect: ep.Reconnect}, nil
	default:
		return nil, fmt.Errorf("unsupported mode %q", ep.Mode)
	}
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket path %s", path)
	}
	return os.Remove(path)
}
