package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
)

func TestConfigurationOperationsNeverExecuteFunctions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(home, "executed")
	yaml := `version: 1
shared:
  shell-functions:
    dangerous: |
      touch ` + marker + `
envs:
  dev:
    project: /tmp
`
	path := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := config.FunctionDigest(settings)
	if digest == "" {
		t.Fatal("empty function digest")
	}
	if err := config.Acknowledge(digest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("configured function executed during non-activation operation")
	}
	ackPath, _ := config.AcknowledgementPath()
	b, _ := os.ReadFile(ackPath)
	if strings.Contains(string(b), "touch") || strings.Contains(string(b), marker) {
		t.Fatal("function body persisted in acknowledgement")
	}
}
