package config

import "testing"

func FuzzLoadRawNeverPanics(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"schema":3,"nodeId":"gw","tunnels":[]}`),
		[]byte(`{}`),
		[]byte(`null`),
		[]byte(`{"schema":3,"nodeId":"gw","limits":{"maxActivePairs":1024},"tunnels":[{"id":"x","field":{"mode":"listen","network":"tcp","bind":"127.0.0.1:1"},"consumer":{"mode":"connect","network":"tcp","address":"127.0.0.1:2"}}]}`),
		[]byte{0xff, 0x00, '{', '}'},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		_, _ = loadRaw(raw)
	})
}

func FuzzNormalizeUSBIDNeverPanics(f *testing.F) {
	for _, seed := range []string{"1234", "0x1A2b", "", "ffff", "10000", "zzzz", "  3c  "} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		_, _ = normalizeUSBID(value)
	})
}
