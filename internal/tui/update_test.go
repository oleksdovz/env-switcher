package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/dolf/env-switcher/internal/config"
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
	next, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if next.(Model).Selected != "dev" {
		t.Fatal("Enter did not select project")
	}
}
