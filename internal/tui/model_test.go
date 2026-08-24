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
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
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

// TestReloadRebuildsEmptyList covers the case a previously non-empty reload result is dropped:
// huh.Select.Options is a no-op when passed zero options (it only ever grows/replaces a
// populated field), so the reload path must rebuild the field from scratch rather than calling
// Options in place, or a reload down to zero environments would leave stale options on screen.
func TestReloadRebuildsEmptyList(t *testing.T) {
	old := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"only": {Project: "/tmp"}}}
	m := New(old, "/tmp/settings", Services{Reload: func() (*config.Settings, error) {
		return &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{}}, nil
	}})
	next, cmd := m.Update(key("r"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if len(m.Settings.Envs) != 0 {
		t.Fatal("reload did not shrink to zero environments")
	}
	if strings.Contains(m.render(), "only") {
		t.Fatal("stale option survived reload to an empty list")
	}
}

func key(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: text, Code: []rune(text)[0]})
}

// drive executes cmd (and anything it returns, recursively — huh's own field→group→form
// completion travels through a couple of these command/message round trips, e.g. Enter yields a
// nextFieldMsg command, which in turn yields a nextGroupMsg command) the same way the real Bubble
// Tea runtime would: run the command, feed the resulting message back into Update, repeat.
func drive(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			// The real runtime stops the program here instead of delivering QuitMsg to
			// Update; do the same, or a completed/aborted form's unconditional "quit again"
			// response (see forwardToForm) would keep this loop spinning forever.
			return m
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				m = drive(t, m, c)
			}
			return m
		}
		next, nextCmd := m.Update(msg)
		var ok bool
		m, ok = next.(Model)
		if !ok {
			t.Fatalf("Update returned unexpected model type %T", next)
		}
		cmd = nextCmd
	}
	return m
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
	if v, _ := m.field.GetValue().(string); v != "a" {
		t.Fatalf("initial hover = %q, want a", v)
	}
	next, _ := m.Update(key("j"))
	m = next.(Model)
	if v, _ := m.field.GetValue().(string); v != "b" {
		t.Fatalf("j did not move focus to b, hover=%q", v)
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
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = next.(Model)
	// Not a tight bound: huh lays its own help/border/description rows out around the capped
	// option viewport. The point of this test is that a small terminal height doesn't make the
	// view grow unboundedly with the environment count (100 environments would otherwise dwarf
	// this), not that it matches the old hand-drawn box's exact line budget.
	if lines := strings.Count(m.render(), "\n"); lines > m.Height+10 {
		t.Fatalf("small view has %d lines for height %d", lines, m.Height)
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
	if cmd == nil {
		t.Fatal("Enter did not schedule a follow-up command")
	}
	selected = drive(t, selected, cmd)
	if selected.Selected != "dev" {
		t.Fatal("Enter did not confirm selection")
	}
}

// TestSlashDoesNotEnterFilterMode guards the fix in newForm/noFilterKeyMap: huh.Select's "/"
// filter is a real feature, but leaving it enabled would mean this app's own v/e/r/i/q shortcuts
// stop working the moment a user starts typing a search (the letters would go to the filter's
// text box instead), which the pre-huh picker never had to worry about.
func TestSlashDoesNotEnterFilterMode(t *testing.T) {
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}, "staging": {Project: "/tmp"}}}, "/tmp/settings", Services{})
	next, _ := m.Update(key("/"))
	m = next.(Model)
	if m.field.GetFiltering() {
		t.Fatal("\"/\" entered huh's filter mode")
	}
	next, _ = m.Update(key("v"))
	m = next.(Model)
	if m.mode != "view-warning" {
		t.Fatal("v shortcut did not fire after pressing /")
	}
}

func TestEmptyEnvironmentListRendersAndIgnoresEnter(t *testing.T) {
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{}}, "/tmp/settings", Services{})
	view := m.render()
	if !strings.Contains(view, "F10/q") {
		t.Fatalf("empty list did not render footer: %s", view)
	}
	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = next.(Model)
	if m.Selected != "" || cmd != nil {
		t.Fatal("Enter on an empty environment list selected or scheduled a command")
	}
	// Quit must still work with nothing to select.
	next, _ = m.Update(key("q"))
	if _, ok := next.(Model); !ok {
		t.Fatal("quit on empty list did not return a Model")
	}
}

func TestQuitAndCtrlCAbortWithoutSelecting(t *testing.T) {
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, "/tmp/settings", Services{})
	next, cmd := m.Update(key("q"))
	if next.(Model).Selected != "" || cmd == nil {
		t.Fatal("q did not schedule quit without selecting")
	}
}

func TestInvalidConfigurationSurfacesAsStatusNotCrash(t *testing.T) {
	m := New(&config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/tmp"}}}, "/tmp/settings",
		Services{Reload: func() (*config.Settings, error) {
			return nil, errors.New("yaml: line 3: mapping values are not allowed")
		}})
	next, cmd := m.Update(key("r"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !strings.Contains(m.Status, "mapping values") {
		t.Fatalf("invalid configuration error not surfaced: %q", m.Status)
	}
	// Still renders afterward instead of leaving the model unusable.
	if !strings.Contains(m.render(), "F10/q") {
		t.Fatal("model unusable after invalid configuration reload")
	}
}
