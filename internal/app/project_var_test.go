package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/testutil"
)

// TestDirectSwitchWritesProjectVar covers direct CLI selection (`env-switcher <project>`):
// switchCommand goes through config.Bootstrap/Load and environment.Resolve exactly like the real
// CLI dispatch does, so this proves _PROJECT reaches the actual shell payload, not just Resolve's
// return value.
func TestDirectSwitchWritesProjectVar(t *testing.T) {
	home := testutil.IsolatedHome(t)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	settings := "version: 1\nenvs:\n  dev:\n    project: $HOME/projects/my\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := switchCommand("bash", "dev", nil, &stdout, nil); err != nil {
		t.Fatal(err)
	}

	currentEnv, err := os.ReadFile(filepath.Join(dir, "current-env"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "projects/my")
	if !strings.Contains(string(currentEnv), "export _PROJECT="+"'"+want+"'") {
		t.Fatalf("current-env missing expected _PROJECT export: %q", currentEnv)
	}
}

// TestSwitchingBetweenProjectsUpdatesProjectVarInPayload proves each switch's payload reflects
// only that switch's own project — there is no leftover/merged state from a previous selection,
// matching the "updated on every switch" half of the managed-state contract (there being nothing
// to separately remove, since no variable is ever explicitly unset by this project — see
// internal/environment.Resolve's doc comment).
func TestSwitchingBetweenProjectsUpdatesProjectVarInPayload(t *testing.T) {
	home := testutil.IsolatedHome(t)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	settings := "version: 1\nenvs:\n  a:\n    project: $HOME/projects/a\n  b:\n    project: $HOME/projects/b\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := switchCommand("bash", "a", nil, &stdout, nil); err != nil {
		t.Fatal(err)
	}
	afterA, err := os.ReadFile(filepath.Join(dir, "current-env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(afterA), filepath.Join(home, "projects/a")) {
		t.Fatalf("current-env after switching to a missing its own _PROJECT: %q", afterA)
	}

	if err := switchCommand("bash", "b", nil, &stdout, nil); err != nil {
		t.Fatal(err)
	}
	afterB, err := os.ReadFile(filepath.Join(dir, "current-env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(afterB), filepath.Join(home, "projects/a")) {
		t.Fatalf("current-env after switching to b still references a's project: %q", afterB)
	}
	if !strings.Contains(string(afterB), filepath.Join(home, "projects/b")) {
		t.Fatalf("current-env after switching to b missing its own _PROJECT: %q", afterB)
	}
}

// TestSwitchRejectsUnresolvedProject proves an unresolvable `project` path fails the switch
// itself (not just Resolve in isolation), through the same path the CLI and TUI both use.
func TestSwitchRejectsUnresolvedProject(t *testing.T) {
	home := testutil.IsolatedHome(t)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	settings := "version: 1\nenvs:\n  dev:\n    project: relative/path\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := switchCommand("bash", "dev", nil, &stdout, nil); err == nil {
		t.Fatal("expected switching to fail for an unresolved project path")
	}
}

// TestSwitchOverwritesManuallyDeclaredProjectVar proves settings that still manually declare
// _PROJECT (written before it existed, or left over from working around its absence) load and
// switch normally, with the payload always reflecting the computed value, never the stale
// declared one.
func TestSwitchOverwritesManuallyDeclaredProjectVar(t *testing.T) {
	home := testutil.IsolatedHome(t)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	settings := "version: 1\nenvs:\n  dev:\n    project: $HOME/projects/my\n    env-vars:\n      _PROJECT: /stale/leftover/value\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(filepath.Join(dir, "settings.yaml")); err != nil {
		t.Fatalf("settings with a manually declared _PROJECT should still load: %v", err)
	}
	var stdout bytes.Buffer
	if err := switchCommand("bash", "dev", nil, &stdout, nil); err != nil {
		t.Fatal(err)
	}
	currentEnv, err := os.ReadFile(filepath.Join(dir, "current-env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(currentEnv), "/stale/leftover/value") {
		t.Fatalf("stale declared _PROJECT leaked into the payload: %q", currentEnv)
	}
	want := filepath.Join(home, "projects/my")
	if !strings.Contains(string(currentEnv), "export _PROJECT='"+want+"'") {
		t.Fatalf("current-env missing the computed _PROJECT: %q", currentEnv)
	}
}
