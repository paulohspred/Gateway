package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/paulohspdev-cmyk/ProjetoGerador/gateway-umbrella/internal/config"
	"github.com/paulohspdev-cmyk/ProjetoGerador/gateway-umbrella/internal/gateway"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	configPath := flag.String("config", "configs/gateway.example.json", "path to gateway JSON config")
	checkConfig := flag.Bool("check-config", false, "validate configuration and exit without opening transports")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("rc-gateway %s commit=%s built=%s go=%s\n", version, commit, buildDate, runtime.Version())
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.LoadStrict(*configPath)
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(2)
	}
	if *checkConfig {
		fmt.Printf("configuration OK: nodeId=%s tunnels=%d udpTunnels=%d serialProviders=%d canProviders=%d\n", cfg.NodeID, len(cfg.Tunnels), len(cfg.UDPTunnels), len(cfg.SerialProviders), len(cfg.CANProviders))
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("gateway starting", "nodeId", cfg.NodeID, "version", version, "commit", commit, "tunnels", len(cfg.Tunnels), "mode", "bridge-first")
	if err := gateway.New(cfg, logger).Run(ctx); err != nil {
		logger.Error("gateway stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("gateway stopped")
}
