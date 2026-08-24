package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
)

// TestConfigurationOperationsNeverExecuteFunctions proves loading/validating settings never
// executes a configured shell-function's body — that only ever happens after a project is
// actually selected (see internal/shell.Render), no matter how many times the configuration
// itself is parsed.
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
	if _, err := config.Load(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("configured function executed during non-activation operation")
	}
}
