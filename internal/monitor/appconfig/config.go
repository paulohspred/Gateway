package appconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const SchemaVersion = 1

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Config struct {
	Schema       int               `json:"schema"`
	Bind         string            `json:"bind"`
	Provider     string            `json:"provider"`
	DraftCatalog string            `json:"draftCatalog,omitempty"`
	RapidWeb     *RapidWebConfig   `json:"rapidWeb,omitempty"`
	Generators   []GeneratorConfig `json:"generators,omitempty"`
}

type RapidWebConfig struct {
	BaseURL        string `json:"baseUrl"`
	UsernameEnv    string `json:"usernameEnv"`
	PasswordEnv    string `json:"passwordEnv"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type GeneratorConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SiteID       string `json:"siteId"`
	ProfileID    string `json:"profileId,omitempty"`
	ProfileDir   string `json:"profileDir,omitempty"`
	RapidBinding string `json:"rapidBinding"`
	Firmware     string `json:"firmware,omitempty"`
	Hardware     string `json:"hardwareVersion,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode rc-monitor config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("rc-monitor config contains multiple JSON values")
		}
		return Config{}, err
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	config.resolveRelativePaths(filepath.Dir(path))
	return config, nil
}

func (c Config) Validate() error {
	if c.Schema != SchemaVersion {
		return fmt.Errorf("rc-monitor config schema must be %d, got %d", SchemaVersion, c.Schema)
	}
	if err := validateLoopbackBind(c.Bind); err != nil {
		return fmt.Errorf("bind: %w", err)
	}
	switch c.Provider {
	case "fake":
		if c.RapidWeb != nil || len(c.Generators) > 0 || c.DraftCatalog != "" {
			return errors.New("fake provider must not include rapidWeb, generators or draftCatalog")
		}
	case "rapid-web":
		if c.RapidWeb == nil {
			return errors.New("rapid-web provider requires rapidWeb config")
		}
		if err := c.RapidWeb.Validate(); err != nil {
			return err
		}
		if len(c.Generators) == 0 {
			return errors.New("rapid-web provider requires at least one generator")
		}
		seen := make(map[string]struct{}, len(c.Generators))
		for i, generator := range c.Generators {
			if err := generator.Validate(); err != nil {
				return fmt.Errorf("generators[%d]: %w", i, err)
			}
			if _, ok := seen[generator.ID]; ok {
				return fmt.Errorf("duplicate generator id %q", generator.ID)
			}
			seen[generator.ID] = struct{}{}
			if generator.ProfileID != "" && c.DraftCatalog == "" {
				return fmt.Errorf("generator %q uses profileId but draftCatalog is empty", generator.ID)
			}
		}
	default:
		return fmt.Errorf("unsupported provider %q", c.Provider)
	}
	return nil
}

func (c RapidWebConfig) Validate() error {
	if err := validateLoopbackURL(c.BaseURL); err != nil {
		return fmt.Errorf("rapidWeb.baseUrl: %w", err)
	}
	if !envNamePattern.MatchString(c.UsernameEnv) {
		return errors.New("rapidWeb.usernameEnv must be an environment variable name")
	}
	if !envNamePattern.MatchString(c.PasswordEnv) {
		return errors.New("rapidWeb.passwordEnv must be an environment variable name")
	}
	if c.UsernameEnv == c.PasswordEnv {
		return errors.New("rapidWeb username and password environment variables must differ")
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 60 {
		return errors.New("rapidWeb.timeoutSeconds must be between 1 and 60")
	}
	return nil
}

func (g GeneratorConfig) Validate() error {
	if strings.TrimSpace(g.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(g.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(g.SiteID) == "" {
		return errors.New("siteId is required")
	}
	if (g.ProfileID == "") == (g.ProfileDir == "") {
		return errors.New("exactly one of profileId or profileDir is required")
	}
	if strings.TrimSpace(g.RapidBinding) == "" {
		return errors.New("rapidBinding is required")
	}
	return nil
}

func (c *Config) resolveRelativePaths(base string) {
	if c.DraftCatalog != "" && !filepath.IsAbs(c.DraftCatalog) {
		c.DraftCatalog = filepath.Clean(filepath.Join(base, c.DraftCatalog))
	}
	for i := range c.Generators {
		if c.Generators[i].ProfileDir != "" && !filepath.IsAbs(c.Generators[i].ProfileDir) {
			c.Generators[i].ProfileDir = filepath.Clean(filepath.Join(base, c.Generators[i].ProfileDir))
		}
		if c.Generators[i].RapidBinding != "" && !filepath.IsAbs(c.Generators[i].RapidBinding) {
			c.Generators[i].RapidBinding = filepath.Clean(filepath.Join(base, c.Generators[i].RapidBinding))
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
	if !isLoopbackHost(host) {
		return fmt.Errorf("host %q is not loopback", host)
	}
	return nil
}

func validateLoopbackURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("must use http or https")
	}
	if parsed.User != nil {
		return errors.New("must not contain user info")
	}
	if parsed.Hostname() == "" {
		return errors.New("host is required")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("must not contain query or fragment")
	}
	if !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("host %q is not loopback", parsed.Hostname())
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
