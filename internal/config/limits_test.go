package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLimitsConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStreamPairLimitsDefaultFailClosed(t *testing.T) {
	path := writeLimitsConfig(t, `{"schema":3,"nodeId":"gw","tunnels":[{"id":"t","field":{"mode":"listen","network":"tcp","bind":"127.0.0.1:15001"},"consumer":{"mode":"connect","network":"tcp","address":"127.0.0.1:25001"}}]}`)
	cfg, err := LoadStrict(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limits.MaxActivePairs != 1024 {
		t.Fatalf("maxActivePairs=%d want 1024", cfg.Limits.MaxActivePairs)
	}
	if cfg.Tunnels[0].MaxConcurrentPairs != 1 {
		t.Fatalf("maxConcurrentPairs=%d want 1", cfg.Tunnels[0].MaxConcurrentPairs)
	}
}

func TestParallelPairsRequireTriggeredTopology(t *testing.T) {
	path := writeLimitsConfig(t, `{"schema":3,"nodeId":"gw","limits":{"maxActivePairs":8},"tunnels":[{"id":"ambiguous","maxConcurrentPairs":2,"field":{"mode":"listen","network":"tcp","bind":"127.0.0.1:15001"},"consumer":{"mode":"listen","network":"tcp","bind":"127.0.0.1:25001"}}]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "exactly one listen endpoint and one connect endpoint") {
		t.Fatalf("expected ambiguous parallel topology rejection, got %v", err)
	}
}

func TestParallelPairsRespectGlobalLimit(t *testing.T) {
	path := writeLimitsConfig(t, `{"schema":3,"nodeId":"gw","limits":{"maxActivePairs":2},"tunnels":[{"id":"t","maxConcurrentPairs":3,"field":{"mode":"listen","network":"tcp","bind":"127.0.0.1:15001"},"consumer":{"mode":"connect","network":"tcp","address":"127.0.0.1:25001"}}]}`)
	_, err := LoadStrict(path)
	if err == nil || !strings.Contains(err.Error(), "exceeds limits.maxActivePairs") {
		t.Fatalf("expected global limit rejection, got %v", err)
	}
}

func TestStreamPairLimitsAcceptBoundedParallelTopology(t *testing.T) {
	path := writeLimitsConfig(t, `{"schema":3,"nodeId":"gw","limits":{"maxActivePairs":16},"tunnels":[{"id":"t","maxConcurrentPairs":8,"field":{"mode":"listen","network":"tcp","bind":"127.0.0.1:15001"},"consumer":{"mode":"connect","network":"tcp","address":"127.0.0.1:25001"}}]}`)
	cfg, err := LoadStrict(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limits.MaxActivePairs != 16 || cfg.Tunnels[0].MaxConcurrentPairs != 8 {
		t.Fatalf("unexpected limits: global=%d tunnel=%d", cfg.Limits.MaxActivePairs, cfg.Tunnels[0].MaxConcurrentPairs)
	}
}
