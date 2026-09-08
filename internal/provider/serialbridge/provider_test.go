package serialbridge

import (
	"path/filepath"
	"testing"
	"time"
)

func TestValidateAcceptsIndustrialSerialModes(t *testing.T) {
	for _, standard := range []string{"rs232", "rs422", "rs485"} {
		cfg := Config{ID: standard, Socket: filepath.Join(t.TempDir(), "serial.sock"), Device: "/dev/ttyUSB0", Standard: standard, BaudRate: 9600, DataBits: 8, Parity: "even", StopBits: "1", ReadTimeout: 250 * time.Millisecond}
		if err := Validate(cfg); err != nil {
			t.Fatalf("%s rejected: %v", standard, err)
		}
	}
}

func TestValidateRejectsUnsafeSerialConfig(t *testing.T) {
	cfg := Config{ID: "bad", Socket: filepath.Join(t.TempDir(), "serial.sock"), Device: "/dev/ttyUSB0", Standard: "rs485", BaudRate: 0, DataBits: 8, Parity: "none", StopBits: "1"}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected invalid baud rate to fail")
	}
}

func TestParityAndStopBits(t *testing.T) {
	for _, v := range []string{"none", "odd", "even", "mark", "space"} {
		if _, err := parseParity(v); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []string{"1", "1.5", "2"} {
		if _, err := parseStopBits(v); err != nil {
			t.Fatal(err)
		}
	}
}
