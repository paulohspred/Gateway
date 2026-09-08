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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/paulohspred/Gateway/internal/control"
)

var version = "dev"

func main() {
	bind := flag.String("bind", "127.0.0.1:18110", "loopback HTTP listen address")
	state := flag.String("state", "/var/lib/rc-admin/state.json", "persistent state file")
	showVersion := flag.Bool("version", false, "print version")
	check := flag.Bool("check-config", false, "validate runtime configuration")
	flag.Parse()
	if *showVersion {
		fmt.Printf("rc-admin %s\n", version)
		return
	}
	if err := validateLoopback(*bind); err != nil {
		log.Fatalf("invalid bind: %v", err)
	}
	secure := true
	if v := strings.TrimSpace(os.Getenv("RC_ADMIN_COOKIE_SECURE")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			log.Fatal("RC_ADMIN_COOKIE_SECURE must be boolean")
		}
		secure = b
	}
	if *check {
		fmt.Println("rc-admin configuration valid")
		return
	}
	store, err := control.NewStore(*state, os.Getenv("RC_ADMIN_BOOTSTRAP_USER"), os.Getenv("RC_ADMIN_BOOTSTRAP_PASSWORD"))
	if err != nil {
		log.Fatalf("initialize state: %v", err)
	}
	api, err := control.NewServer(store, control.ServerOptions{CookieSecure: secure, SessionTTL: 8 * time.Hour, Version: version})
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Addr: *bind, Handler: api, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() { log.Printf("rc-admin %s listening on %s", version, *bind); errCh <- srv.ListenAndServe() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			log.Printf("shutdown: %v", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}
}
func validateLoopback(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if port == "" {
		return errors.New("port required")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("host %q is not loopback", host)
	}
	return nil
}
