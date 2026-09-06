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
	"strings"
	"syscall"
	"time"

	"github.com/paulohspred/Gateway/internal/monitor"
	"github.com/paulohspred/Gateway/internal/monitor/appconfig"
	"github.com/paulohspred/Gateway/internal/monitor/fake"
	"github.com/paulohspred/Gateway/internal/monitor/httpapi"
	"github.com/paulohspred/Gateway/internal/monitor/profile"
	"github.com/paulohspred/Gateway/internal/monitor/rapid"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "strict rc-monitor JSON configuration file")
	checkConfig := flag.Bool("check-config", false, "validate configuration and exit without opening listeners or Rapid connections")
	bind := flag.String("bind", "127.0.0.1:18100", "development fake-provider HTTP listen address")
	providerName := flag.String("provider", "fake", "development provider when -config is not used")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("rc-monitor %s\n", version)
		return
	}

	var (
		listenAddress string
		provider      monitor.Provider
		runtimeName   string
		err           error
	)

	if *configPath == "" {
		if *checkConfig {
			log.Fatal("-check-config requires -config")
		}
		if err := validateLoopbackBind(*bind); err != nil {
			log.Fatalf("invalid bind: %v", err)
		}
		if *providerName != "fake" {
			log.Fatalf("unsupported development provider %q; use -config for rapid-web", *providerName)
		}
		listenAddress = *bind
		provider = fake.NewProvider(fake.Options{})
		runtimeName = "fake"
	} else {
		config, loadErr := appconfig.Load(*configPath)
		if loadErr != nil {
			log.Fatalf("invalid config: %v", loadErr)
		}
		configs, configErr := loadRapidGeneratorConfigs(config)
		if configErr != nil {
			log.Fatalf("invalid config: %v", configErr)
		}
		if *checkConfig {
			fmt.Println("rc-monitor configuration valid")
			return
		}
		listenAddress = config.Bind
		runtimeName = config.Provider
		switch config.Provider {
		case "fake":
			provider = fake.NewProvider(fake.Options{})
		case "rapid-web":
			provider, err = newRapidWebProvider(config, configs)
			if err != nil {
				log.Fatalf("initialize rapid-web provider: %v", err)
			}
		default:
			log.Fatalf("unsupported provider %q", config.Provider)
		}
	}

	service, err := monitor.NewService(provider)
	if err != nil {
		log.Fatal(err)
	}
	api, err := httpapi.New(service)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("rc-monitor %s listening on %s with %s provider", version, listenAddress, runtimeName)
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

func loadRapidGeneratorConfigs(config appconfig.Config) ([]rapid.GeneratorConfig, error) {
	if config.Provider == "fake" {
		return nil, nil
	}

	var draftBundles map[string]profile.Bundle
	if config.DraftCatalog != "" {
		var err error
		draftBundles, err = profile.LoadDraftCatalog(config.DraftCatalog)
		if err != nil {
			return nil, fmt.Errorf("load draft controller catalog: %w", err)
		}
	}

	configs := make([]rapid.GeneratorConfig, 0, len(config.Generators))
	for _, generatorConfig := range config.Generators {
		var (
			bundle profile.Bundle
			err    error
		)
		if generatorConfig.ProfileDir != "" {
			bundle, err = profile.LoadDir(generatorConfig.ProfileDir)
			if err != nil {
				return nil, fmt.Errorf("generator %q profile: %w", generatorConfig.ID, err)
			}
		} else {
			var ok bool
			bundle, ok = draftBundles[generatorConfig.ProfileID]
			if !ok {
				return nil, fmt.Errorf("generator %q profileId %q not found", generatorConfig.ID, generatorConfig.ProfileID)
			}
		}

		binding, err := rapid.LoadBinding(generatorConfig.RapidBinding, bundle)
		if err != nil {
			return nil, fmt.Errorf("generator %q rapid binding: %w", generatorConfig.ID, err)
		}
		generator := monitor.Generator{
			ID:     generatorConfig.ID,
			Name:   generatorConfig.Name,
			SiteID: generatorConfig.SiteID,
			Controller: monitor.ControllerRef{
				Manufacturer:    bundle.Manifest.Manufacturer,
				Model:           bundle.Manifest.Model,
				Firmware:        generatorConfig.Firmware,
				HardwareVersion: generatorConfig.Hardware,
				SerialNumber:    generatorConfig.SerialNumber,
			},
		}
		if err := generator.Validate(); err != nil {
			return nil, fmt.Errorf("generator %q: %w", generatorConfig.ID, err)
		}
		configs = append(configs, rapid.GeneratorConfig{
			Generator: generator,
			Profile:   bundle,
			Binding:   binding,
		})
	}
	return configs, nil
}

func newRapidWebProvider(config appconfig.Config, configs []rapid.GeneratorConfig) (monitor.Provider, error) {
	if config.RapidWeb == nil {
		return nil, errors.New("rapidWeb config is required")
	}
	username, ok := os.LookupEnv(config.RapidWeb.UsernameEnv)
	if !ok || strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("environment variable %s is required", config.RapidWeb.UsernameEnv)
	}
	password, ok := os.LookupEnv(config.RapidWeb.PasswordEnv)
	if !ok || password == "" {
		return nil, fmt.Errorf("environment variable %s is required", config.RapidWeb.PasswordEnv)
	}

	reader, err := rapid.NewWebReader(rapid.WebReaderOptions{
		BaseURL:  config.RapidWeb.BaseURL,
		Username: username,
		Password: password,
		Timeout:  time.Duration(config.RapidWeb.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return rapid.NewProvider(reader, configs, rapid.Options{})
}

func validateLoopbackBind(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("expected host:port: %w", err)
	}
	if port == "" {
		return errors.New("port is required")
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
