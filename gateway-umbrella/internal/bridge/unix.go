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
	if ep.TLS.Enabled {
		return nil, fmt.Errorf("TLS is not valid on unix endpoints")
	}
	switch ep.Mode {
	case "listen":
		if !filepath.IsAbs(ep.Bind) {
			return nil, fmt.Errorf("unix listen path must be absolute")
		}
		if err := removeStaleSocket(ep.Bind); err != nil {
			return nil, err
		}
		ln, err := net.Listen("unix", ep.Bind)
		if err != nil {
			return nil, err
		}
		base := &listenSource{ln: ln}
		s := &unixListenSource{listenSource: base, path: ep.Bind}
		go func() { <-ctx.Done(); _ = s.Close() }()
		return s, nil
	case "connect":
		if !filepath.IsAbs(ep.Address) {
			return nil, fmt.Errorf("unix connect path must be absolute")
		}
		if ep.DialTimeout <= 0 {
			ep.DialTimeout = 10 * time.Second
		}
		if ep.Reconnect <= 0 {
			ep.Reconnect = 5 * time.Second
		}
		return &dialSource{network: "unix", address: ep.Address, timeout: ep.DialTimeout, reconnect: ep.Reconnect}, nil
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
