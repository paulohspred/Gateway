//go:build linux

package canbridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveStaleSocketRefusesRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "can.sock")
	if err := os.WriteFile(path, []byte("do-not-delete"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(path); err == nil {
		t.Fatal("expected regular file protection")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("regular file was removed: %v", err)
	}
	if string(raw) != "do-not-delete" {
		t.Fatal("regular file content changed")
	}
}
