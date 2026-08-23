package tui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/dolf/env-switcher/internal/config"
)

func TestViewShowsSortedProjectsAndKeys(t *testing.T) {
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"z": {Project: "/tmp"}, "a": {Project: "/tmp"}}}
	m := New(s, "/tmp/settings", Services{})
	view := m.render()
	if strings.Index(view, "a") > strings.Index(view, "z") || !strings.Contains(view, "F10/q") {
		t.Fatalf("unexpected view: %s", view)
	}
}

func TestTypicalRenderUnder100ms(t *testing.T) {
	envs := map[string]config.ProjectEnvironment{}
	for i := 0; i < 100; i++ {
		envs[string(rune('Ā'+i))] = config.ProjectEnvironment{Project: "/tmp"}
	}
	m := New(&config.Settings{Version: 1, Envs: envs}, "/tmp/settings", Services{})
	started := time.Now()
	_ = m.render()
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("TUI render took %s", elapsed)
	}
}

func TestReloadRetainsPreviousModelOnFailureAndWarnsOnChangedFunctions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"old": {Project: "/tmp"}}}
	m := New(old, "/tmp/settings", Services{Reload: func() (*config.Settings, error) { return nil, errors.New("invalid settings") }})
	next, cmd := m.Update(key("r"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if _, ok := m.Settings.Envs["old"]; !ok || !strings.Contains(m.Status, "invalid") {
		t.Fatal("failed reload replaced valid model")
	}
	body := "echo trusted"
	m.services.Reload = func() (*config.Settings, error) {
		return &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"new": {Project: "/tmp", ShellFunctions: map[string]string{"f": body}}}}, nil
	}
	next, cmd = m.Update(key("r"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if _, ok := m.Settings.Envs["new"]; !ok || m.mode != "trust" {
		t.Fatal("valid changed-function reload did not replace and warn")
	}
}

func key(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: text, Code: []rune(text)[0]})
}

func TestKeyboardNavigationSelectionAndF2Warning(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.yaml"
	content := "version: 1\nSECRET: canary\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"a": {Project: "/tmp"}, "b": {Project: "/tmp"}}}
	m := New(s, path, Services{})
	next, _ := m.Update(key("j"))
	m = next.(Model)
	if m.Focus != 1 {
		t.Fatal("j did not move focus")
	}
	next, _ = m.Update(key("v"))
	m = next.(Model)
	if m.mode != "view-warning" || strings.Contains(m.render(), "canary") {
		t.Fatal("F2 disclosed before confirmation")
	}
	next, _ = m.Update(key("y"))
	m = next.(Model)
	if m.mode != "view" || !strings.Contains(m.render(), "canary") {
		t.Fatal("confirmed F2 did not show full file")
	}
	next, _ = m.Update(key("n"))
	m = next.(Model)
	if strings.Contains(m.render(), "canary") {
		t.Fatal("F2 content remained after closing")
	}
}

func TestSmallTerminalViewIsBounded(t *testing.T) {
	envs := map[string]config.ProjectEnvironment{}
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		envs[name] = config.ProjectEnvironment{Project: "/tmp"}
	}
	m := New(&config.Settings{Version: 1, Envs: envs}, "/tmp/settings", Services{})
	m.Height = 8
	if lines := strings.Count(m.render(), "\n"); lines > 9 {
		t.Fatalf("small view has %d lines", lines)
	}
}

func TestStableKeyAlternativesAndSelection(t *testing.T) {
	pairs := map[string]string{"f2": "view", "v": "view", "f3": "edit", "e": "edit", "f4": "reload", "r": "reload", "f5": "install", "i": "install", "f10": "quit", "q": "quit"}
	for input, want := range pairs {
		if got := action(input); got != want {
			t.Fatalf("action(%s)=%s want %s", input, got, want)
		}
	}
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, "/tmp/settings", Services{})
	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	selected := next.(Model)
	if selected.Selected != "dev" || cmd == nil {
		t.Fatal("Enter did not confirm selection")
	}
}
