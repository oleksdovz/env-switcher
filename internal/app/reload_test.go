package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
)

func TestReloadReplacesOnlyWithValidCandidate(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".env-switcher")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.yaml")
	current := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"old": {Project: "/tmp"}}}
	if err := os.WriteFile(path, []byte("invalid: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := reloadSettings(path, current)
	if err == nil || got != current {
		t.Fatal("invalid reload replaced current state")
	}
	valid := []byte("version: 1\nenvs:\n  new:\n    project: /tmp\n")
	if err := os.WriteFile(path, valid, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = reloadSettings(path, current)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Envs["new"]; !ok {
		t.Fatal("valid reload not applied")
	}
}
