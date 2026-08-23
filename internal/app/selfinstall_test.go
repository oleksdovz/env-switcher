package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestSelfInstallSilentlyUpdatesAlreadyInstalledCopy exercises the "old system" branch: a
// prior install already exists, so running a binary from elsewhere silently refreshes the
// installed executable and the rc block, without any confirmation prompt.
func TestSelfInstallSilentlyUpdatesAlreadyInstalledCopy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "env-switcher"), []byte("old-version"), 0o700); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	selfInstall(bytes.NewBufferString(""), &stdout, &stderr) // no stdin needed: no prompt expected

	if stderr.Len() != 0 {
		t.Fatalf("silent update printed to stderr unexpectedly: %q", stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("updated env-switcher")) {
		t.Fatalf("expected an update confirmation, got %q", stdout.String())
	}
	installed, err := os.ReadFile(filepath.Join(dir, "bin", "env-switcher"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(installed, []byte("old-version")) {
		t.Fatal("installed executable was not refreshed")
	}
	profile := filepath.Join(home, ".bashrc")
	b, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("profile not written: %v", err)
	}
	if !bytes.Contains(b, []byte("env-switcher managed block")) {
		t.Fatalf("profile missing managed block: %q", b)
	}
	// A settings.yaml is not part of this contract: self-update never bootstraps settings on
	// an already-installed system, only the fresh-setup path does.
	if _, err := os.Stat(filepath.Join(dir, "settings.yaml")); !os.IsNotExist(err) {
		t.Fatalf("self-update unexpectedly created settings.yaml: %v", err)
	}
}

// TestSelfInstallNoOpWhenAlreadyRunningInstalledCopy proves the common case — normal usage
// through the installed wrapper — never triggers any self-install activity. A symlink stands
// in for "the running binary is the installed one", since EvalSymlinks resolves both sides to
// the same canonical path.
func TestSelfInstallNoOpWhenAlreadyRunningInstalledCopy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")

	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".env-switcher", "bin")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(current, filepath.Join(dir, "env-switcher")); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	selfInstall(bytes.NewBufferString(""), &stdout, &stderr)

	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected no self-install activity when already running the installed copy, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); !os.IsNotExist(err) {
		t.Fatalf("unexpected profile write: %v", err)
	}
}
