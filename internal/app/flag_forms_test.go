package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/testutil"
)

// TestEveryDocumentedCommandAcceptsItsFlagForm guards the whole class of bug where a command's
// switch-case in Run only matched its bare word: any word that doesn't hit a case falls through
// to switchDispatch and is treated as a project name, so a missing "--flag" case doesn't error at
// compile time — it silently misroutes to "project \"--flag\" is not configured" (or, worse, an
// actual switch attempt) instead of running the command. help.go documents every command as
// having a "--flag" equivalent; this proves each one actually does, not just the ones covered by
// other tests incidentally.
func TestEveryDocumentedCommandAcceptsItsFlagForm(t *testing.T) {
	home := testutil.IsolatedHome(t)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	settings := "version: 1\nenvs:\n  dev:\n    project: /tmp\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeEditor := filepath.Join(home, "editor")
	if err := os.WriteFile(fakeEditor, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", fakeEditor)

	cases := []struct {
		name string
		args []string
		// stdin feeds a decline ("n") to any command that would otherwise prompt (install,
		// rollback, uninstall, view) — this test only cares that dispatch reached the right
		// command, not that a destructive action ran to completion.
		stdin string
	}{
		{"list", []string{"--list"}, ""},
		{"get", []string{"--get", "dev"}, ""},
		{"edit", []string{"--edit"}, ""},
		{"validate", []string{"--validate"}, ""},
		{"reload", []string{"--reload"}, ""},
		{"view", []string{"--view"}, "n\n"},
		{"install", []string{"--install", "--shell", "bash"}, "n\n"},
		{"rollback", []string{"--rollback", "--shell", "bash"}, "n\n"},
		{"uninstall", []string{"--uninstall", "--shell", "bash"}, "n\n"},
		{"version", []string{"--version"}, ""},
		{"help", []string{"--help"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			a := New(BuildInfo{Version: "test"})
			code := a.Run(context.Background(), c.args, strings.NewReader(c.stdin), &stdout, &stderr)
			combined := stdout.String() + stderr.String()
			if strings.Contains(combined, "is not configured") {
				t.Fatalf("--flag form fell through to switch-to-project dispatch (exit %d): %s", code, combined)
			}
			if strings.HasPrefix(combined, "switch:") {
				t.Fatalf("--flag form was misrouted to the switch command (exit %d): %s", code, combined)
			}
		})
	}
}

// TestBareAndFlagFormsProduceIdenticalOutcomes strengthens the above: for the commands that don't
// need extra confirmation/state, the bare word and its "--flag" form must behave identically, not
// merely "both avoid the switch path".
func TestBareAndFlagFormsProduceIdenticalOutcomes(t *testing.T) {
	pairs := [][2]string{
		{"list", "--list"},
		{"validate", "--validate"},
		{"reload", "--reload"},
		{"version", "--version"},
		{"help", "--help"},
	}
	for _, p := range pairs {
		t.Run(p[0], func(t *testing.T) {
			home := testutil.IsolatedHome(t)
			dir := filepath.Join(home, ".env-switcher")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			settings := "version: 1\nenvs:\n  dev:\n    project: /tmp\n"
			if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
				t.Fatal(err)
			}
			var bareOut, flagOut bytes.Buffer
			bareCode := New(BuildInfo{Version: "test"}).Run(context.Background(), []string{p[0]}, nil, &bareOut, &bareOut)
			flagCode := New(BuildInfo{Version: "test"}).Run(context.Background(), []string{p[1]}, nil, &flagOut, &flagOut)
			if bareCode != flagCode || bareOut.String() != flagOut.String() {
				t.Fatalf("%s (%d, %q) != %s (%d, %q)", p[0], bareCode, bareOut.String(), p[1], flagCode, flagOut.String())
			}
		})
	}
}
