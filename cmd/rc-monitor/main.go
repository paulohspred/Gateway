package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/fake"
	"github.com/paulohspred/Gateway/internal/monitor/httpapi"
)

var version = "dev"

func main() {
	bind := flag.String("bind", "127.0.0.1:18100", "HTTP listen address (loopback only in this development stage)")
	providerName := flag.String("provider", "fake", "data provider (currently only fake)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("rc-monitor %s\n", version)
		return
	}
	if err := validateLoopbackBind(*bind); err != nil {
		log.Fatalf("invalid bind: %v", err)
	}
	if *providerName != "fake" {
		log.Fatalf("unsupported provider %q; only fake is available in MON-002", *providerName)
	}

	provider := fake.NewProvider(fake.Options{})
	service, err := monitor.NewService(provider)
	if err != nil {
		log.Fatal(err)
	}
	api, err := httpapi.New(service)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              *bind,
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("rc-monitor %s listening on %s with development fake provider", version, *bind)
		errCh <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}
}

func validateLoopbackBind(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("expected host:port: %w", err)
	}
	if port == "" {
		return errors.New("port is required")
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("host %q is not loopback", host)
	}
	return nil
}
