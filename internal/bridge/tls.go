package bridge

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
)

func buildTLSConfig(ep Endpoint) (*tls.Config, error) {
	if !ep.TLS.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS13}
	if ep.TLS.CAFile != "" {
		pool, err := loadCertPool(ep.TLS.CAFile)
		if err != nil {
			return nil, err
		}
		if ep.Mode == "listen" {
			cfg.ClientCAs = pool
		} else {
			cfg.RootCAs = pool
		}
	}
	if (ep.TLS.CertFile == "") != (ep.TLS.KeyFile == "") {
		return nil, fmt.Errorf("tls certFile and keyFile must be configured together")
	}
	if ep.TLS.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(ep.TLS.CertFile, ep.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load tls keypair: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if ep.Mode == "listen" {
		if len(cfg.Certificates) == 0 {
			return nil, fmt.Errorf("tls listen endpoint requires certFile/keyFile")
		}
		if ep.TLS.RequireClientCert {
			if cfg.ClientCAs == nil {
				return nil, fmt.Errorf("mTLS listener requires caFile")
			}
			cfg.ClientAuth = tls.RequireAndVerifyClientCert
		}
		return cfg, nil
	}
	if ep.Mode == "connect" {
		cfg.ServerName = ep.TLS.ServerName
		if cfg.ServerName == "" {
			host, _, err := net.SplitHostPort(ep.Address)
			if err == nil {
				cfg.ServerName = host
			}
		}
		if cfg.ServerName == "" {
			return nil, fmt.Errorf("tls connect endpoint requires serverName or host:port address")
		}
		return cfg, nil
	}
	return nil, fmt.Errorf("tls unsupported mode %q", ep.Mode)
}
func loadCertPool(path string) (*x509.CertPool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(raw) {
		return nil, fmt.Errorf("CA file %s contains no valid certificates", path)
	}
	return pool, nil
}
