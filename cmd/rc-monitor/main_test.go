package main

import "testing"

func TestValidateLoopbackBind(t *testing.T) {
	for _, address := range []string{"127.0.0.1:18100", "[::1]:18100", "localhost:18100"} {
		if err := validateLoopbackBind(address); err != nil {
			t.Fatalf("expected %q to be accepted: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:18100", "10.10.10.1:18100", ":18100", "127.0.0.1"} {
		if err := validateLoopbackBind(address); err == nil {
			t.Fatalf("expected %q to be rejected", address)
		}
	}
}
