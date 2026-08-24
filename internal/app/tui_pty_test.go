package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestPTYSelectionMeetsSC001(t *testing.T) {
	for _, shellName := range []string{"bash", "zsh"} {
		shellName := shellName
		t.Run(shellName, func(t *testing.T) { testPTYSelection(t, shellName) })
	}
}

// TestPTYSelectionAfterReload reproduces a reported bug: reloading (F4/r) inside a real
// interactive session left the environment list rendering normally, but arrow keys and Enter
// stopped doing anything afterward — nothing could be selected until the process was restarted.
func TestPTYSelectionAfterReload(t *testing.T) {
	testPTYSelectionAfterReload(t, "bash")
}

func testPTYSelectionAfterReload(t *testing.T, shellName string) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/"+shellName)
	dir := filepath.Join(home, ".env-switcher")
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "env-switcher"), []byte("placeholder"), 0o700); err != nil {
		t.Fatal(err)
	}
	// Matches the shape of the real report: multiple projects, so selection requires actually
	// navigating (not just pressing Enter on whatever's already hovered by default), and a
	// project directory containing a space, as in the reported settings.
	settings := "version: 1\nenvs:\n" +
		"  dev:\n    project: " + project + "\n    env-vars:\n      PROJECT_VALUE: dev\n" +
		"  prod:\n    project: " + project + "\n    env-vars:\n      PROJECT_VALUE: prod\n" +
		"  staging:\n    project: " + project + "\n    env-vars:\n      PROJECT_VALUE: staging\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
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

	screen := make([]byte, 0, 8192)
	buf := make([]byte, 1024)
	readUntil := func(marker string, within time.Duration) {
		t.Helper()
		deadline := time.Now().Add(within)
		for !bytes.Contains(screen, []byte(marker)) && time.Now().Before(deadline) {
			_ = master.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
			n, _ := master.Read(buf)
			screen = append(screen, buf[:n]...)
		}
		if !bytes.Contains(screen, []byte(marker)) {
			t.Fatalf("did not see %q within %s; screen=%q", marker, within, screen)
		}
	}

	readUntil("F10/q Exit", 5*time.Second)
	if _, err := master.Write([]byte{'r'}); err != nil { // F4/r: reload
		t.Fatal(err)
	}
	readUntil("projects reloaded", 5*time.Second)
	time.Sleep(300 * time.Millisecond) // let the reload's own render settle, like a real user pausing to read it
	// Navigate down twice (dev -> prod -> staging) before submitting — Enter alone on the
	// still-default-hovered first option wouldn't catch navigation-specific breakage.
	if _, err := master.Write([]byte{'j', 'j'}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := master.Write([]byte{'\r'}); err != nil { // Enter: select
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("TUI exited with %d after reload+select; screen=%q", code, screen)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("selection after reload timed out — Enter did nothing; screen=%q", screen)
	}
	currentEnv, err := os.ReadFile(filepath.Join(dir, "current-env"))
	if err != nil {
		t.Fatalf("current-env not written after reload+select: %v", err)
	}
	if !strings.Contains(string(currentEnv), "PROJECT_VALUE='staging'") {
		t.Fatalf("navigating after reload did not select staging (still on the default hover?): %q", currentEnv)
	}
}

func testPTYSelection(t *testing.T, shellName string) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/"+shellName)
	dir := filepath.Join(home, ".env-switcher")
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	// Pre-seed the installed-executable marker so this run takes the silent
	// already-installed self-update path instead of prompting for first-time setup — this
	// test measures SC-001 (TUI selection), not the separate self-install confirmation flow.
	if err := os.WriteFile(filepath.Join(dir, "bin", "env-switcher"), []byte("placeholder"), 0o700); err != nil {
		t.Fatal(err)
	}
	settings := "version: 1\nenvs:\n  dev:\n    project: " + project + "\n    env-vars:\n      PROJECT_VALUE: dev\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
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
	started := time.Now()
	// stdin, stdout, and stderr all attach to the pty, matching a real terminal session:
	// nothing captures stdout via command substitution anymore, so the TUI renders there.
	go func() { done <- New(BuildInfo{Version: "test"}).Run(t.Context(), nil, slave, slave, slave) }()
	screen := make([]byte, 0, 4096)
	buf := make([]byte, 1024)
	// F10/q Exit only ever appears in the TUI's own footer, never in the self-install
	// messages that may precede it — a plain "env-switcher" substring match would race
	// against that self-install output and send the keystroke below too early.
	const tuiMarker = "F10/q Exit"
	deadline := time.Now().Add(5 * time.Second)
	for !bytes.Contains(screen, []byte(tuiMarker)) && time.Now().Before(deadline) {
		_ = master.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, _ := master.Read(buf)
		screen = append(screen, buf[:n]...)
	}
	if !bytes.Contains(screen, []byte(tuiMarker)) {
		select {
		case code := <-done:
			t.Fatalf("TUI exited before render with %d; screen=%q", code, screen)
		default:
			t.Fatalf("TUI did not render in PTY; screen=%q", screen)
		}
	}
	if _, err := master.Write([]byte{'\r'}); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("TUI exited with %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TUI selection timed out")
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("selection took %s", elapsed)
	}
	currentEnv, err := os.ReadFile(filepath.Join(dir, "current-env"))
	if err != nil {
		t.Fatalf("current-env not written: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "current-env")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("current-env has unexpected permissions: %v %v", info, err)
	}
	if !strings.Contains(string(currentEnv), "PROJECT_VALUE='dev'") {
		t.Fatal("selection did not write the resolved environment to current-env")
	}
	if !strings.Contains(string(currentEnv), "export _PROJECT='"+project+"'") {
		t.Fatalf("interactive TUI selection did not resolve _PROJECT: %q", currentEnv)
	}
	// The shell payload is written directly by switchCommand, entirely separate from the TUI's
	// own screen output — but the TUI's rendering (lipgloss styles, box-drawing borders, the
	// huh-based form) does write real ANSI escape sequences and control characters to the pty,
	// so assert none of that leaked into the file Bash/Zsh will `source`.
	for _, b := range currentEnv {
		if b == 0x1b || (b < 0x20 && b != '\n' && b != '\t') {
			t.Fatalf("shell payload contains a control/escape byte 0x%02x: %q", b, currentEnv)
		}
	}
}
