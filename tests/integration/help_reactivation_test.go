package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
	installer "github.com/dolf/env-switcher/internal/install"
)

// TestHelpDoesNotReactivateProject is the full-stack regression test for the reported bug: after
// a real successful switch, running a non-switch command like `env-switcher --help` through the
// real installed wrapper and the real binary must not reactivate that project again — no
// "activated" hook re-run, no shell-cmd side effects, no reload, just the plain help text. See
// TestWrapperDoesNotReactivateOnNonSwitchCommands for the wrapper-only (fake-binary) version of
// this same contract.
func TestHelpDoesNotReactivateProject(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		name := name
		t.Run(name, func(t *testing.T) { testHelpDoesNotReactivateProject(t, name) })
	}
}

func testHelpDoesNotReactivateProject(t *testing.T, shellName string) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go toolchain unavailable")
	}
	if _, err := exec.LookPath(shellName); err != nil {
		t.Skip(shellName + " unavailable")
	}

	// Built before HOME is overridden below: `go build` uses $HOME for its module/build caches,
	// and pointing that at a temp dir this test later deletes causes cleanup failures against
	// the (deliberately read-only) module cache it would create there.
	root := filepath.Join("..", "..")
	builtBinary := filepath.Join(t.TempDir(), "env-switcher")
	build := exec.Command(goBin, "build", "-o", builtBinary, "./cmd/env-switcher")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v %s", err, out)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".env-switcher")
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(binDir, "env-switcher")
	builtBytes, err := os.ReadFile(builtBinary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, builtBytes, 0o700); err != nil {
		t.Fatal(err)
	}

	settings := "version: 1\nenvs:\n  dev:\n    project: /tmp\n    env-vars:\n      MARKER: activated\n" +
		"    shell-cmd: |\n      echo hook-ran >>\"$HOOK_LOG\"\n"
	settingsPath := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(settingsPath, []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(settingsPath)
	if err != nil {
		t.Fatalf("test settings do not validate: %v", err)
	}
	if err := config.Acknowledge(config.FunctionDigest(loaded)); err != nil {
		t.Fatal(err)
	}

	wrapper, ok := installer.Wrapper(shellName)
	if !ok {
		t.Fatalf("no wrapper template for %s", shellName)
	}
	profile := filepath.Join(home, "profile")
	if err := os.WriteFile(profile, []byte(wrapper), 0o600); err != nil {
		t.Fatal(err)
	}
	hookLog := filepath.Join(home, "hook.log")

	script := `export HOOK_LOG=` + shellQuoteForTest(hookLog) + `
source ` + shellQuoteForTest(profile) + `
env-switcher dev >/dev/null
test "$MARKER" = activated || exit 60
test "$(wc -l <` + shellQuoteForTest(hookLog) + `)" -eq 1 || exit 61

help_output=$(env-switcher --help)
help_status=$?
test "$help_status" -eq 0 || exit 62
case "$help_output" in
  *"env-switcher manages"*) ;;
  *) exit 63 ;;
esac
case "$help_output" in
  *"activated"*) exit 64 ;;
esac
test "$(wc -l <` + shellQuoteForTest(hookLog) + `)" -eq 1 || exit 65
`
	args := []string{"-f", "-c", script}
	if shellName == "bash" {
		args = []string{"--noprofile", "--norc", "-c", script}
	}
	cmd := exec.Command(shellName, args...)
	cmd.Env = append(os.Environ(), "HOME="+home, "SHELL=/bin/"+shellName)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("env-switcher --help reactivated the project: %v\n%s", err, out)
	}
}
