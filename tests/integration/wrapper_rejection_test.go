package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	installer "github.com/dolf/env-switcher/internal/install"
)

// TestWrapperLeavesEnvironmentUnchangedWhenSwitchFails proves the new contract: the installed
// wrapper always sources whatever is at current-env after running the binary, so a failed
// switch (unknown project) must leave that file untouched rather than emit anything to fake a
// success. Re-sourcing the same unchanged file is what "no change on failure" means once
// activation no longer round-trips through the wrapper for validation.
func TestWrapperLeavesEnvironmentUnchangedWhenSwitchFails(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		wrapper, _ := installer.Wrapper(name)
		tc := struct{ name, wrapper string }{name, wrapper}
		t.Run(tc.name, func(t *testing.T) {
			if _, err := exec.LookPath(tc.name); err != nil {
				t.Skip(tc.name + " unavailable")
			}
			home := t.TempDir()
			dir := filepath.Join(home, ".env-switcher")
			if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(dir, "bin", "env-switcher")
			// Simulates a failed switch: non-zero exit, and it must not touch current-env.
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(dir, "current-env")
			if err := os.WriteFile(sentinel, []byte("export BEFORE=unchanged\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(home, "profile")
			if err := os.WriteFile(profile, []byte(tc.wrapper), 0o600); err != nil {
				t.Fatal(err)
			}
			script := `source "$HOME/profile"
env-switcher unknown-project
code=$?
test "$code" -ne 0 || exit 90
test "$BEFORE" = unchanged || exit 91
`
			args := []string{"-f", "-c", script}
			if tc.name == "bash" {
				args = []string{"--noprofile", "--norc", "-c", script}
			}
			cmd := exec.Command(tc.name, args...)
			cmd.Env = append(os.Environ(), "HOME="+home)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("failed switch changed the environment: %v %s", err, out)
			}
			if b, err := os.ReadFile(sentinel); err != nil || string(b) != "export BEFORE=unchanged\n" {
				t.Fatalf("current-env was modified after a failed switch: %q %v", b, err)
			}
		})
	}
}

func TestWrapperSkipsSourceWhenCurrentEnvMissing(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		wrapper, _ := installer.Wrapper(name)
		tc := struct{ name, wrapper string }{name, wrapper}
		t.Run(tc.name, func(t *testing.T) {
			if _, err := exec.LookPath(tc.name); err != nil {
				t.Skip(tc.name + " unavailable")
			}
			home := t.TempDir()
			dir := filepath.Join(home, ".env-switcher")
			if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(dir, "bin", "env-switcher")
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(home, "profile")
			if err := os.WriteFile(profile, []byte(tc.wrapper), 0o600); err != nil {
				t.Fatal(err)
			}
			script := `source "$HOME/profile"
env-switcher version
`
			args := []string{"-f", "-c", script}
			if tc.name == "bash" {
				args = []string{"--noprofile", "--norc", "-c", script}
			}
			cmd := exec.Command(tc.name, args...)
			cmd.Env = append(os.Environ(), "HOME="+home)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("wrapper failed with no current-env file present: %v %s", err, out)
			}
		})
	}
}
