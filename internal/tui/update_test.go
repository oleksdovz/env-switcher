package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/upgrade"
)

func TestInstallRequiresConfirmation(t *testing.T) {
	called := 0
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, "/tmp/settings", Services{Install: func() error { called++; return nil }})
	next, cmd := m.Update(key("i"))
	m = next.(Model)
	if m.mode != "install-warning" || cmd != nil || called != 0 {
		t.Fatal("install ran without confirmation")
	}
	next, cmd = m.Update(key("y"))
	m = next.(Model)
	if cmd == nil || called != 0 {
		t.Fatal("confirmed install command not scheduled correctly")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if called != 1 || m.Status == "" {
		t.Fatal("confirmed install did not complete")
	}
}

func TestF6RequiresConfirmationAndInvokesSharedUpgradeService(t *testing.T) {
	called := 0
	want := upgrade.Result{OldVersion: "v1.0.0", NewVersion: "v1.1.0", InstalledPath: "/tmp/env-switcher"}
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, "/tmp/settings",
		Services{Upgrade: func() (upgrade.Result, error) { called++; return want, nil }})
	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF6}))
	m = next.(Model)
	if m.mode != "upgrade-warning" || cmd != nil || called != 0 {
		t.Fatal("F6 upgraded without confirmation")
	}
	next, cmd = m.Update(key("y"))
	m = next.(Model)
	if cmd == nil || called != 0 {
		t.Fatal("confirmed upgrade command not scheduled correctly")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if called != 1 {
		t.Fatal("confirmed F6 did not call Services.Upgrade")
	}
	if m.Status != "upgraded v1.0.0 -> v1.1.0" {
		t.Fatalf("unexpected status: %q", m.Status)
	}
}

func TestF6AlreadyCurrentReportsStatusWithoutError(t *testing.T) {
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, "/tmp/settings",
		Services{Upgrade: func() (upgrade.Result, error) {
			return upgrade.Result{NewVersion: "v1.0.0", AlreadyCurrent: true}, nil
		}})
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF6}))
	m = next.(Model)
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.Status != "already up to date (v1.0.0)" {
		t.Fatalf("unexpected status: %q", m.Status)
	}
}

func TestF6SurfacesUpgradeErrorAsStatus(t *testing.T) {
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, "/tmp/settings",
		Services{Upgrade: func() (upgrade.Result, error) {
			return upgrade.Result{}, errors.New("no compatible asset for windows/amd64")
		}})
	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF6}))
	m = next.(Model)
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.Status != "no compatible asset for windows/amd64" {
		t.Fatalf("unexpected status: %q", m.Status)
	}
}

func TestEditorActionReturnsSuspendCommand(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "editor")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", fake)
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, filepath.Join(dir, "settings"), Services{})
	_, cmd := m.Update(key("e"))
	if cmd == nil {
		t.Fatal("editor action did not return Bubble Tea suspend command")
	}
}

func TestSelectionPayloadIntentOnlyAfterEnter(t *testing.T) {
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, "/tmp/settings", Services{})
	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.Selected != "" {
		t.Fatal("navigation selected project")
	}
	next, _ = m.Update(key("q"))
	if next.(Model).Selected != "" {
		t.Fatal("exit selected project")
	}
	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = drive(t, next.(Model), cmd)
	if m.Selected != "dev" {
		t.Fatal("Enter did not select project")
	}
}
