package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHomeTildeForms(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := ExpandHome("~")
	if err != nil || got != home {
		t.Fatalf("~ = %q, %v; want %q", got, err, home)
	}
	got, err = ExpandHome("~/projects/my")
	if err != nil || got != filepath.Join(home, "projects/my") {
		t.Fatalf("~/projects/my = %q, %v", got, err)
	}
	if _, err := ExpandHome("~someone/x"); err == nil {
		t.Fatal("expected an error for ~someone (only ~/ is supported)")
	}
}

func TestExpandHomeDollarForms(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cases := map[string]string{
		"$HOME/projects/my":     filepath.Join(home, "projects/my"),
		"${HOME}/projects/my":   filepath.Join(home, "projects/my"),
		"$HOME":                 home,
		"prefix-$HOME-suffix":   "prefix-" + home + "-suffix",
		"$HOMEBREW/opt":         "$HOMEBREW/opt", // must not match inside a longer identifier
		"no expansion here":     "no expansion here",
		"/already/absolute/dir": "/already/absolute/dir",
	}
	for in, want := range cases {
		got, err := ExpandHome(in)
		if err != nil {
			t.Errorf("ExpandHome(%q): unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ExpandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExpandHomeDoesNotLookUpHomeWhenNotNeeded(t *testing.T) {
	// HOME is deliberately left unset/empty: a path with no ~ or $HOME reference must not fail
	// just because home-directory resolution would.
	t.Setenv("HOME", "")
	got, err := ExpandHome("/no/home/reference/needed")
	if err != nil || got != "/no/home/reference/needed" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestExpandVarWordBoundaries(t *testing.T) {
	cases := []struct{ s, name, value, want string }{
		{"$_PROJECT/.codex", "_PROJECT", "/p", "/p/.codex"},
		{"${_PROJECT}/.codex", "_PROJECT", "/p", "/p/.codex"},
		{"$_PROJECT2/.codex", "_PROJECT", "/p", "$_PROJECT2/.codex"}, // longer identifier, no match
		{"no reference", "_PROJECT", "/p", "no reference"},
		{"$_PROJECT", "_PROJECT", "/p", "/p"},
		{"a$_PROJECTb", "_PROJECT", "/p", "a$_PROJECTb"}, // "b" continues the identifier
		{"$_PROJECT $_PROJECT", "_PROJECT", "/p", "/p /p"},
	}
	for _, c := range cases {
		if got := ExpandVar(c.s, c.name, c.value); got != c.want {
			t.Errorf("ExpandVar(%q, %q, %q) = %q, want %q", c.s, c.name, c.value, got, c.want)
		}
	}
}

func TestExpandVarDoesNotTouchCommandSubstitutionOrOtherVars(t *testing.T) {
	in := `$(echo pwned) ` + "`echo also-pwned`" + ` $OTHER_VAR`
	got := ExpandVar(in, "_PROJECT", "/p")
	if got != in {
		t.Fatalf("ExpandVar must leave unrelated shell syntax untouched: got %q", got)
	}
}

func TestExpandHomePreservesUnicodeAndSpaces(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := ExpandHome("$HOME/projects/Україна 🇺🇦/dev with spaces")
	want := filepath.Join(home, "projects/Україна 🇺🇦/dev with spaces")
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestExpandHomeErrorWhenHomeUnset(t *testing.T) {
	old, hadOld := os.LookupEnv("HOME")
	if hadOld {
		defer os.Setenv("HOME", old)
	}
	os.Unsetenv("HOME")
	if _, err := ExpandHome("$HOME/x"); err == nil {
		t.Fatal("expected an error when $HOME cannot be resolved")
	}
}
