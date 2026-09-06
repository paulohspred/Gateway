package appconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRapidWebConfigStrictAndResolvesPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rc-monitor.json")
	content := `{
  "schema": 1,
  "bind": "127.0.0.1:18100",
  "provider": "rapid-web",
  "draftCatalog": "controllers.json",
  "rapidWeb": {
    "baseUrl": "http://127.0.0.1/",
    "usernameEnv": "RC_RAPID_USER",
    "passwordEnv": "RC_RAPID_PASSWORD",
    "timeoutSeconds": 10
  },
  "generators": [{
    "id": "gen-1",
    "name": "Generator 1",
    "siteId": "site-1",
    "profileId": "rc.comap.generic",
    "rapidBinding": "gen-1-channels.json"
  }]
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(config.DraftCatalog) || !filepath.IsAbs(config.Generators[0].RapidBinding) {
		t.Fatal("expected relative paths to be resolved")
	}
}

func TestConfigRejectsPlaintextOrUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	content := `{
  "schema": 1,
  "bind": "127.0.0.1:18100",
  "provider": "rapid-web",
  "rapidWeb": {
    "baseUrl": "http://127.0.0.1/",
    "usernameEnv": "RC_RAPID_USER",
    "passwordEnv": "RC_RAPID_PASSWORD",
    "timeoutSeconds": 10,
    "password": "secret"
  },
  "generators": []
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown plaintext password field to fail, got %v", err)
	}
}

func TestConfigRejectsRemoteBindAndAmbiguousProfile(t *testing.T) {
	config := Config{
		Schema:   SchemaVersion,
		Bind:     "0.0.0.0:18100",
		Provider: "rapid-web",
		RapidWeb: &RapidWebConfig{
			BaseURL:        "http://127.0.0.1/",
			UsernameEnv:    "USER_ENV",
			PasswordEnv:    "PASS_ENV",
			TimeoutSeconds: 10,
		},
		Generators: []GeneratorConfig{{
			ID:           "g",
			Name:         "G",
			SiteID:       "s",
			ProfileID:    "one",
			ProfileDir:   "two",
			RapidBinding: "b.json",
		}},
	}
	if err := config.Validate(); err == nil || !strings.Contains(err.Error(), "bind") {
		t.Fatalf("expected remote bind to fail, got %v", err)
	}

	config.Bind = "127.0.0.1:18100"
	if err := config.Validate(); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected ambiguous profile selection to fail, got %v", err)
	}
}
