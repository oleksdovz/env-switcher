package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	installer "github.com/dolf/env-switcher/internal/install"
)

// TestWrapperDoesNotReactivateStaleStateOnFailedSwitch proves the fixed contract: current-env is
// only ever a signal for "this invocation just switched", never "some previous invocation once
// did". The wrapper clears it before running the binary, and switchCommand only ever recreates
// it on a *successful* switch, so a failed switch attempt (unknown project) must leave the shell
// exactly as it was before the call — not reactivate whatever an earlier, unrelated successful
// switch happened to leave behind.
func TestWrapperDoesNotReactivateStaleStateOnFailedSwitch(t *testing.T) {
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
			stale := filepath.Join(dir, "current-env")
			if err := os.WriteFile(stale, []byte("export BEFORE=stale\n"), 0o600); err != nil {
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
test -z "$BEFORE" || exit 91
`
			args := []string{"-f", "-c", script}
			if tc.name == "bash" {
				args = []string{"--noprofile", "--norc", "-c", script}
			}
			cmd := exec.Command(tc.name, args...)
			cmd.Env = append(os.Environ(), "HOME="+home)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("failed switch reactivated a stale prior environment: %v %s", err, out)
			}
			if _, err := os.Stat(stale); !os.IsNotExist(err) {
				t.Fatalf("stale current-env should have been cleared before invocation, stat err=%v", err)
			}
		})
	}
}

// TestWrapperDoesNotReactivateOnNonSwitchCommands is the wrapper-level half of the "env-switcher
// --help" reactivation bug fix (see TestHelpDoesNotReactivateProject for the full-stack version
// with the real binary): a leftover current-env from an earlier, unrelated successful switch
// must not be re-sourced just because *some* env-switcher command ran, successfully, afterward.
func TestWrapperDoesNotReactivateOnNonSwitchCommands(t *testing.T) {
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
			// Stands in for any non-switch command (--help, list, version, upgrade, ...): it
			// ignores its arguments, prints one line, and exits 0 without touching current-env
			// — exactly like the real dispatcher does for those commands.
			if err := os.WriteFile(fake, []byte("#!/bin/sh\necho ran-nonswitch\nexit 0\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			leftover := filepath.Join(dir, "current-env")
			leftoverPayload := "printf 'env-switcher: activated should-not-run\\n'\nexport REACTIVATED=yes\n"
			if err := os.WriteFile(leftover, []byte(leftoverPayload), 0o600); err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(home, "profile")
			if err := os.WriteFile(profile, []byte(tc.wrapper), 0o600); err != nil {
				t.Fatal(err)
			}
			script := `source "$HOME/profile"
output=$(env-switcher --help)
test "$output" = ran-nonswitch || exit 92
test -z "$REACTIVATED" || exit 93
`
			args := []string{"-f", "-c", script}
			if tc.name == "bash" {
				args = []string{"--noprofile", "--norc", "-c", script}
			}
			cmd := exec.Command(tc.name, args...)
			cmd.Env = append(os.Environ(), "HOME="+home)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("non-switch command reactivated a leftover switch payload: %v %s", err, out)
			}
			if _, err := os.Stat(leftover); !os.IsNotExist(err) {
				t.Fatalf("leftover current-env should have been cleared before invocation, stat err=%v", err)
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
