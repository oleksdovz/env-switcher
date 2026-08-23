package config

import (
	"os"
	"strings"
	"testing"
)

func TestAcknowledgementContainsOnlyDigest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	digest := "abc123"
	if err := Acknowledge(digest); err != nil {
		t.Fatal(err)
	}
	if !IsAcknowledged(digest) {
		t.Fatal("digest not acknowledged")
	}
	path, _ := AcknowledgementPath()
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "secret body") || strings.Contains(string(b), "function") {
		t.Fatal("unexpected content")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	if err := Acknowledge("changed"); err != nil {
		t.Fatal(err)
	}
	if !IsAcknowledged("changed") {
		t.Fatal("atomic replacement did not persist new digest")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if IsAcknowledged("changed") {
		t.Fatal("unsafe metadata permissions accepted")
	}
}
