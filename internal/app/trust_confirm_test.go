package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/testutil"
)

// writeTrustFixture lays out settings with a shared shell-function, so switching to "dev"
// requires acknowledgment.
func writeTrustFixture(t *testing.T) (home, dir string) {
	t.Helper()
	home = testutil.IsolatedHome(t)
	dir = filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	settings := "version: 1\n" +
		"shared:\n" +
		"  shell-functions:\n" +
		"    greet: |\n" +
		"      echo hi\n" +
		"envs:\n" +
		"  dev:\n" +
		"    project: /tmp\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	return home, dir
}

// TestDirectSwitchConfirmsTrustedFunctionsInline proves `env-switcher <project>` no longer hard
// refuses when trusted functions haven't been acknowledged yet: it prompts right there (stderr),
// and "y" both completes the switch and persists the acknowledgment, so a second switch with the
// same settings never prompts again.
func TestDirectSwitchConfirmsTrustedFunctionsInline(t *testing.T) {
	home, dir := writeTrustFixture(t)
	settings, err := config.Load(filepath.Join(dir, "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	digest := config.FunctionDigest(settings)
	if config.IsAcknowledged(digest) {
		t.Fatal("fixture should start unacknowledged")
	}

	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("y\n")
	if err := switchCommand("bash", "dev", stdin, &stdout, &stderr); err != nil {
		t.Fatalf("expected the switch to succeed after confirming inline: %v", err)
	}
	if !strings.Contains(stderr.String(), "Trust and run them?") {
		t.Fatalf("expected an inline confirmation prompt on stderr, got: %q", stderr.String())
	}
	if !config.IsAcknowledged(digest) {
		t.Fatal("confirming inline should persist the acknowledgment, same as the TUI dialog does")
	}
	if _, err := os.Stat(filepath.Join(home, ".env-switcher", "current-env")); err != nil {
		t.Fatalf("expected a switch payload to be written: %v", err)
	}

	// A second switch with the same (unchanged) settings must not prompt again.
	var stdout2, stderr2 bytes.Buffer
	if err := switchCommand("bash", "dev", strings.NewReader(""), &stdout2, &stderr2); err != nil {
		t.Fatalf("second switch should succeed without re-prompting: %v", err)
	}
	if strings.Contains(stderr2.String(), "Trust and run them?") {
		t.Fatalf("second switch re-prompted even though the digest was already acknowledged: %q", stderr2.String())
	}
}

// TestDirectSwitchDeclinesTrustedFunctions proves answering anything but "y" cancels the switch
// (outcome class 2, matching install/upgrade decline behavior) without writing a payload and
// without persisting an acknowledgment.
func TestDirectSwitchDeclinesTrustedFunctions(t *testing.T) {
	home, dir := writeTrustFixture(t)
	settings, err := config.Load(filepath.Join(dir, "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	digest := config.FunctionDigest(settings)

	var stdout, stderr bytes.Buffer
	err = switchCommand("bash", "dev", strings.NewReader("n\n"), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected declining the trust prompt to fail the switch")
	}
	appErr, ok := err.(*Error)
	if !ok || appErr.Outcome != OutcomeCancelled {
		t.Fatalf("expected a cancelled outcome, got: %v", err)
	}
	if config.IsAcknowledged(digest) {
		t.Fatal("declining must not persist an acknowledgment")
	}
	if _, err := os.Stat(filepath.Join(home, ".env-switcher", "current-env")); !os.IsNotExist(err) {
		t.Fatalf("declining must not write a switch payload, stat err=%v", err)
	}
}

// TestTUISwitchNeverBlocksOnStdinForTrustedFunctions proves switchCommand's TUI call path (stdin
// == nil) keeps the old hard refusal instead of attempting to read anything — reading here would
// block forever against a Bubble Tea program that has already released the terminal.
func TestTUISwitchNeverBlocksOnStdinForTrustedFunctions(t *testing.T) {
	_, dir := writeTrustFixture(t)
	_ = dir
	var stdout, stderr bytes.Buffer
	err := switchCommand("bash", "dev", nil, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected the switch to fail when stdin is nil and trust is unacknowledged")
	}
	appErr, ok := err.(*Error)
	if !ok || appErr.Outcome != OutcomeSecurity {
		t.Fatalf("expected a security outcome, got: %v", err)
	}
	if !strings.Contains(appErr.Message, "open env-switcher") {
		t.Fatalf("expected the message to point at the TUI, got: %q", appErr.Message)
	}
}
