package gateway

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/paulohspred/Gateway/internal/config"
)

func TestGatewayReadinessLifecycle(t *testing.T) {
	cfg := config.Config{
		NodeID: "gw-test",
		Admin:  config.Admin{Bind: "127.0.0.1:0"},
		Tunnels: []config.Tunnel{
			{
				ID:       "local-pair",
				Field:    config.Endpoint{Mode: "listen", Bind: "127.0.0.1:0"},
				Consumer: config.Endpoint{Mode: "listen", Bind: "127.0.0.1:0"},
			},
		},
	}
	g := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	waitForMetric(t, g, "rc_gateway_ready 1")
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error on cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("gateway did not stop after cancellation")
	}
	waitForMetric(t, g, "rc_gateway_ready 0")
}

func TestGatewayReturnsAdminStartupError(t *testing.T) {
	cfg := config.Config{NodeID: "gw-test", Admin: config.Admin{Bind: "127.0.0.1:not-a-port"}}
	g := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := g.Run(ctx); err == nil || !strings.Contains(err.Error(), "admin") {
		t.Fatalf("expected admin startup error, got %v", err)
	}
}

func waitForMetric(t *testing.T, g *Gateway, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var buf bytes.Buffer
		g.metrics.WritePrometheus(&buf)
		if strings.Contains(buf.String(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	var buf bytes.Buffer
	g.metrics.WritePrometheus(&buf)
	t.Fatalf("metric %q not observed; metrics=%q", want, buf.String())
}
