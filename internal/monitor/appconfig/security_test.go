package appconfig

import "testing"

func TestRapidWebConfigRejectsRemoteEndpointOffline(t *testing.T) {
	config := RapidWebConfig{
		BaseURL:        "https://example.com/",
		UsernameEnv:    "RC_RAPID_USER",
		PasswordEnv:    "RC_RAPID_PASSWORD",
		TimeoutSeconds: 10,
	}
	if err := config.Validate(); err == nil {
		t.Fatal("expected remote Rapid endpoint to fail validation")
	}
}

func TestRapidWebConfigAcceptsLoopbackEndpoint(t *testing.T) {
	config := RapidWebConfig{
		BaseURL:        "http://127.0.0.1/",
		UsernameEnv:    "RC_RAPID_USER",
		PasswordEnv:    "RC_RAPID_PASSWORD",
		TimeoutSeconds: 10,
	}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
}
