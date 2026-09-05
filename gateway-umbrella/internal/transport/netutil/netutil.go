package netutil

import (
	"fmt"
	"net"
	"net/netip"
)

func ParsePrefixes(raw []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, item := range raw {
		p, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q", item)
		}
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}
func PeerAllowed(addr net.Addr, allowed []netip.Prefix) bool {
	if len(allowed) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	for _, p := range allowed {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
