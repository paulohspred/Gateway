package netutil

import (
	"net"
	"testing"
)

func TestPeerAllowedAcceptsIPv4MappedAddress(t *testing.T) {
	allowed, err := ParsePrefixes([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	addr := &net.TCPAddr{IP: net.ParseIP("::ffff:10.20.30.40"), Port: 502}
	if !PeerAllowed(addr, allowed) {
		t.Fatal("expected IPv4-mapped peer to match IPv4 allowlist")
	}
}

func TestPeerAllowedFailsClosedOnNilAddress(t *testing.T) {
	allowed, err := ParsePrefixes([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	if PeerAllowed(nil, allowed) {
		t.Fatal("nil peer must not bypass a configured allowlist")
	}
}
