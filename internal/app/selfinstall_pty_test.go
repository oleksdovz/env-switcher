package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestSelfInstallFreshConfirmedSetsUpEverything runs the bare (no-args) entry point against a
// completely fresh HOME — no ~/.env-switcher at all — with a real PTY (bubbletea needs one),
// confirms the self-install prompt, and verifies the directory, starter settings, installed
// executable, and managed rc block all exist afterward. It exits via F10 without selecting a
// project, since selection itself is already covered by TestPTYSelectionMeetsSC001.
func TestSelfInstallFreshConfirmedSetsUpEverything(t *testing.T) {
	for _, shellName := range []string{"bash", "zsh"} {
		shellName := shellName
		t.Run(shellName, func(t *testing.T) { testSelfInstallFreshConfirmed(t, shellName) })
	}
}

func testSelfInstallFreshConfirmed(t *testing.T, shellName string) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/"+shellName)

	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	if err := pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Fatal(err)
	}

	done := make(chan int, 1)
	go func() { done <- New(BuildInfo{Version: "test"}).Run(t.Context(), nil, slave, slave, slave) }()

	screen := make([]byte, 0, 4096)
	buf := make([]byte, 1024)
	deadline := time.Now().Add(5 * time.Second)
	for !bytes.Contains(screen, []byte("[y/N]")) && time.Now().Before(deadline) {
		_ = master.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, _ := master.Read(buf)
		screen = append(screen, buf[:n]...)
	}
	if !bytes.Contains(screen, []byte("[y/N]")) {
		t.Fatalf("self-install prompt did not appear; screen=%q", screen)
	}
	if _, err := master.Write([]byte("y\n")); err != nil {
		t.Fatal(err)
	}

	// The starter settings.yaml Bootstrap just created includes example shell functions, so
	// the TUI's first-run trusted-code warning appears before the normal project list does.
	// Answer it too, then wait for the list view's footer.
	const tuiMarker = "F10/q Exit"
	const trustPrompt = "trusted executable code"
	trustAnswered := false
	deadline = time.Now().Add(5 * time.Second)
	for !bytes.Contains(screen, []byte(tuiMarker)) && time.Now().Before(deadline) {
		_ = master.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, _ := master.Read(buf)
		screen = append(screen, buf[:n]...)
		if !trustAnswered && bytes.Contains(screen, []byte(trustPrompt)) {
			if _, err := master.Write([]byte("y\n")); err != nil {
				t.Fatal(err)
			}
			trustAnswered = true
		}
	}
	if !bytes.Contains(screen, []byte(tuiMarker)) {
		t.Fatalf("TUI did not render after confirmed self-install; screen=%q", screen)
	}
	if _, err := master.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exited with %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("quit timed out")
	}

	dir := filepath.Join(home, ".env-switcher")
	if _, err := os.Stat(filepath.Join(dir, "settings.yaml")); err != nil {
		t.Fatalf("starter settings.yaml not created: %v", err)
	}
	installed := filepath.Join(dir, "bin", "env-switcher")
	if info, err := os.Stat(installed); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("executable not installed at %s: %v %v", installed, info, err)
	}
	profile := filepath.Join(home, "."+shellName+"rc")
	b, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("profile not written: %v", err)
	}
	if !bytes.Contains(b, []byte("env-switcher managed block")) || !bytes.Contains(b, []byte("bin/env-switcher")) {
		t.Fatalf("profile missing managed block: %q", b)
	}
}
