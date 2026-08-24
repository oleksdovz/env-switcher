package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
)

// TestStarterConfigSwitchesEndToEnd proves the actual shipped starter file — anchors, merge
// keys, single-body shell-functions, and shared+project shell-cmd hooks all included — parses,
// resolves, and activates correctly for a real project, in a real shell.
func TestStarterConfigSwitchesEndToEnd(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go toolchain unavailable")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	root := filepath.Join("..", "..")
	binary := filepath.Join(t.TempDir(), "env-switcher")
	build := exec.Command(goBin, "build", "-o", binary, "./cmd/env-switcher")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v %s", err, out)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	starter, err := os.ReadFile(filepath.Join(root, "testdata", "config", "starter.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(settingsPath, starter, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(settingsPath); err != nil {
		t.Fatalf("starter file does not validate: %v", err)
	}

	cmd := exec.Command(binary, "staging")
	cmd.Env = append(os.Environ(), "HOME="+home, "SHELL=/bin/bash")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("switch failed: %v stdout=%s stderr=%s", err, out.String(), stderr.String())
	}

	currentEnv := filepath.Join(dir, "current-env")
	script := `start=$PWD
source ` + shellQuoteForTest(currentEnv) + `
test "$PWD" = "$start" || exit 40
test "$AWS_REGION" = eu-west-1 || exit 41
test "$LOG_LEVEL" = debug || exit 42
test "$__ENV_SWITCHER_ACTIVE_PROJECT" = staging || exit 43
typeset -f k_load >/dev/null 2>&1 || exit 44
typeset -f k_use_staging >/dev/null 2>&1 || exit 45
`
	verify := exec.Command("bash", "--noprofile", "--norc", "-c", script)
	verify.Env = append(os.Environ(), "HOME="+home)
	if vout, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("sourced current-env did not activate staging correctly: %v %s", err, vout)
	}
}
