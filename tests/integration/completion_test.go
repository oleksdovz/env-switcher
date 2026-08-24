package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/dolf/env-switcher/internal/config"
	installer "github.com/dolf/env-switcher/internal/install"
)

// builtEnvSwitcherOnce builds the real env-switcher binary exactly once for every test in this
// file that needs one, instead of once per (sub)test — `go build` was the single biggest cost in
// this file, run up to 7 times across what's otherwise a handful of fast, isolated-home tests.
var (
	builtEnvSwitcherOnce sync.Once
	builtEnvSwitcherPath string
	builtEnvSwitcherErr  error
)

func builtEnvSwitcher(t *testing.T) []byte {
	t.Helper()
	builtEnvSwitcherOnce.Do(func() {
		goBin, err := exec.LookPath("go")
		if err != nil {
			builtEnvSwitcherErr = err
			return
		}
		// Built into a directory outside t.TempDir() (which would be removed at the end of
		// whichever single test happened to trigger this Once) and before any test below
		// overrides $HOME — `go build` uses $HOME for its module/build caches, and pointing that
		// at a temp dir a test later deletes causes cleanup failures against the (deliberately
		// read-only) module cache it would create there.
		dir, err := os.MkdirTemp("", "env-switcher-completion-build-*")
		if err != nil {
			builtEnvSwitcherErr = err
			return
		}
		out := filepath.Join(dir, "env-switcher")
		build := exec.Command(goBin, "build", "-o", out, "./cmd/env-switcher")
		build.Dir = filepath.Join("..", "..")
		if combined, err := build.CombinedOutput(); err != nil {
			builtEnvSwitcherErr = fmt.Errorf("build failed: %w: %s", err, combined)
			return
		}
		builtEnvSwitcherPath = out
	})
	if builtEnvSwitcherErr != nil {
		t.Skipf("could not build env-switcher: %v", builtEnvSwitcherErr)
	}
	data, err := os.ReadFile(builtEnvSwitcherPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// buildCompletionFixture lays out an isolated home with the (once-built, shared) env-switcher
// binary installed plus a small settings.yaml.
func buildCompletionFixture(t *testing.T) (home string) {
	t.Helper()
	binary := builtEnvSwitcher(t)

	home = t.TempDir()
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "env-switcher"), binary, 0o700); err != nil {
		t.Fatal(err)
	}
	settings := "version: 1\nenvs:\n  dev:\n    project: /tmp\n  staging:\n    project: /tmp\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// pathWithoutYq returns the current $PATH with every directory that actually contains a `yq`
// executable removed, so a "yq is not installed" scenario stays true regardless of where a given
// machine happens to install it. A hardcoded "/usr/bin:/bin" isn't good enough here — some CI
// images (unlike a typical dev machine, where yq usually lives under Homebrew) ship yq
// preinstalled directly in /usr/bin, so that literal PATH value doesn't actually hide it.
func pathWithoutYq(t *testing.T) string {
	t.Helper()
	dirs := filepath.SplitList(os.Getenv("PATH"))
	kept := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(d, "yq")); err == nil {
			continue
		}
		kept = append(kept, d)
	}
	return strings.Join(kept, string(filepath.ListSeparator))
}

// writeRC installs the real, current wrapper template (installer.Wrapper — never a hand-copied
// string, so this test stays honest to what's actually shipped) into the given shell's startup
// file, plus zsh's compinit bootstrap where relevant.
func writeRC(t *testing.T, home, shellName string) {
	t.Helper()
	wrapper, ok := installer.Wrapper(shellName)
	if !ok {
		t.Fatalf("no wrapper template for %s", shellName)
	}
	var rc string
	switch shellName {
	case "zsh":
		rc = "autoload -Uz compinit && compinit -u -d " + shellQuoteForTest(filepath.Join(home, ".zcompdump")) + "\n" + wrapper
	case "bash":
		rc = wrapper
	}
	name := ".bashrc"
	if shellName == "zsh" {
		name = ".zshrc"
	}
	if err := os.WriteFile(filepath.Join(home, name), []byte(rc), 0o600); err != nil {
		t.Fatal(err)
	}
}

// completeInPTY starts an interactive shell in a real PTY, types keys, and returns everything
// written to the terminal within the read window — enough to see what a real TAB-completion
// press actually offered, not just what a function returns when called directly.
func completeInPTY(t *testing.T, shellName, home string, extraEnv []string, keys string, readFor time.Duration) string {
	t.Helper()
	var cmd *exec.Cmd
	switch shellName {
	case "bash":
		cmd = exec.Command("bash", "--rcfile", filepath.Join(home, ".bashrc"), "-i")
	case "zsh":
		// -d (--no-globalrcs) skips /etc/zsh/* entirely, so this test only ever runs the rc file
		// it wrote into the isolated $HOME below. Without it, whatever a given machine's system-wide
		// /etc/zsh/zshrc happens to do (some CI images configure completion, or call compinit,
		// system-wide for every user) runs first and is outside this test's control — on at least
		// one real CI runner that produced its own "insecure directories" prompt before this test's
		// own compinit call ever got a chance to run, which then ate the test's typed keys as its
		// y/n answer instead of a TAB press.
		cmd = exec.Command("zsh", "-d", "-i")
	default:
		t.Fatalf("unsupported shell %q", shellName)
	}
	cmd.Env = append([]string{"HOME=" + home, "TERM=xterm", "PS1=$ "}, extraEnv...)

	master, err := pty.StartWithAttrs(cmd, &pty.Winsize{Rows: 24, Cols: 120}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer func() { _ = cmd.Process.Kill() }()

	// SetReadDeadline on a pty.Start-spawned master doesn't reliably interrupt a blocked Read on
	// every platform, so bound reads by draining continuously in the background instead of racing
	// individual reads against a deadline — the test just snapshots the buffer after sleeping for
	// as long as it wants to give the shell to respond.
	var mu sync.Mutex
	var screen bytes.Buffer
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				mu.Lock()
				screen.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	snapshot := func() string {
		mu.Lock()
		defer mu.Unlock()
		return screen.String()
	}

	time.Sleep(1500 * time.Millisecond) // let the shell (and compinit, for zsh) start up
	if _, err := master.Write([]byte(keys)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(readFor)
	out := snapshot()
	_, _ = master.Write([]byte("\x03")) // Ctrl-C: abandon the completion attempt, don't run anything
	return out
}

func TestZshCompletionWorksOnFirstInvocation(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh unavailable")
	}
	if _, err := exec.LookPath("yq"); err != nil {
		t.Skip("yq unavailable")
	}
	home := buildCompletionFixture(t)
	writeRC(t, home, "zsh")
	// No prior env-switcher invocation in this session at all — proving the completion function
	// (top-level, not nested inside env-switcher()) is available from the very first TAB press.
	screen := completeInPTY(t, "zsh", home, nil, "env-switcher \t\t", 1500*time.Millisecond)
	if !strings.Contains(screen, "dev") || !strings.Contains(screen, "staging") {
		t.Fatalf("completion did not offer configured projects on first use: %q", screen)
	}
	assertNoActivation(t, home, screen)
}

func TestZshCompletionFallsBackToListWithoutYq(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh unavailable")
	}
	home := buildCompletionFixture(t)
	writeRC(t, home, "zsh")
	screen := completeInPTY(t, "zsh", home, []string{"PATH=" + pathWithoutYq(t)}, "env-switcher \t\t", 1500*time.Millisecond)
	if !strings.Contains(screen, "dev") || !strings.Contains(screen, "staging") {
		t.Fatalf("completion did not fall back to `env-switcher list` without yq: %q", screen)
	}
	assertNoActivation(t, home, screen)
}

func TestBashCompletionWorksOnFirstInvocation(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	if _, err := exec.LookPath("yq"); err != nil {
		t.Skip("yq unavailable")
	}
	home := buildCompletionFixture(t)
	writeRC(t, home, "bash")
	screen := completeInPTY(t, "bash", home, nil, "env-switcher \t\t", 1500*time.Millisecond)
	if !strings.Contains(screen, "dev") || !strings.Contains(screen, "staging") {
		t.Fatalf("completion did not offer configured projects on first use: %q", screen)
	}
	assertNoActivation(t, home, screen)
}

func TestBashCompletionFallsBackToListWithoutYq(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	home := buildCompletionFixture(t)
	writeRC(t, home, "bash")
	screen := completeInPTY(t, "bash", home, []string{"PATH=" + pathWithoutYq(t)}, "env-switcher \t\t", 1500*time.Millisecond)
	if !strings.Contains(screen, "dev") || !strings.Contains(screen, "staging") {
		t.Fatalf("completion did not fall back to `env-switcher list` without yq: %q", screen)
	}
	assertNoActivation(t, home, screen)
}

// TestZshCompletionSurvivesShellCmdReloadingCompinit reproduces the second reported bug: a
// configured shell-cmd that calls `compinit` again (a common way to (re)load some other tool's
// completions right after a switch) wipes zsh's *entire* completion registry, not just entries
// added since the last compinit — silently losing this binding along with everything else, so
// completion broke after exactly one successful switch. env-switcher() re-asserts the binding
// itself on every invocation specifically to self-heal from this.
func TestZshCompletionSurvivesShellCmdReloadingCompinit(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh unavailable")
	}
	if _, err := exec.LookPath("yq"); err != nil {
		t.Skip("yq unavailable")
	}
	home := buildCompletionFixture(t)
	// Overwrite the fixture's settings with one whose shell-cmd calls compinit again, exactly
	// the pattern reported: a real-world zshrc reloading some tool's completion on every switch.
	settings := "version: 1\n" +
		"shared:\n" +
		"  shell-cmd: |\n" +
		"    autoload -Uz compinit\n" +
		"    compinit -u\n" +
		"envs:\n" +
		"  dev:\n" +
		"    project: /tmp\n" +
		"  staging:\n" +
		"    project: /tmp\n"
	settingsPath := filepath.Join(home, ".env-switcher", "settings.yaml")
	if err := os.WriteFile(settingsPath, []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	// Acknowledge the shell-cmd's trust warning so the upcoming switch actually applies it,
	// exactly as a real first-time user would via the TUI/CLI. config.Acknowledge resolves paths
	// from $HOME, so this test process's own environment needs to match the PTY subprocess's.
	t.Setenv("HOME", home)
	set, err := config.Load(settingsPath)
	if err != nil {
		t.Fatalf("fixture settings do not validate: %v", err)
	}
	if err := config.Acknowledge(config.FunctionDigest(set)); err != nil {
		t.Fatal(err)
	}

	writeRC(t, home, "zsh")
	// Switch once (running the compinit-calling shell-cmd), then attempt completion — on the
	// *second* command in this session, exactly the reported "stops working after being called a
	// second time".
	screen := completeInPTY(t, "zsh", home, nil, "env-switcher dev\r"+"env-switcher \t\t", 2000*time.Millisecond)
	if !strings.Contains(screen, "dev") || !strings.Contains(screen, "staging") {
		t.Fatalf("completion did not survive a shell-cmd that reloads compinit: %q", screen)
	}
}

// assertNoActivation proves completion never switched, activated, or wrote a payload: exactly
// what "must not execute the env-switcher wrapper, source current-env, activate a project, or run
// any configured shell command" means observably.
func assertNoActivation(t *testing.T, home, screen string) {
	t.Helper()
	if strings.Contains(screen, "activated") {
		t.Fatalf("completion appears to have activated a project: %q", screen)
	}
	if _, err := os.Stat(filepath.Join(home, ".env-switcher", "current-env")); !os.IsNotExist(err) {
		t.Fatalf("completion wrote a switch payload, stat err=%v", err)
	}
}

// The remaining edge cases (missing yq and missing binary, empty .envs, invalid YAML) are
// exercised by calling the completion function directly after sourcing the real template — a
// real terminal isn't needed to prove "no candidates, no error", and this is faster/more
// deterministic than driving a PTY for every combination.

func TestZshCompletionDegradesGracefully(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh unavailable")
	}
	testCompletionDegradesGracefully(t, "zsh")
}

func TestBashCompletionDegradesGracefully(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	testCompletionDegradesGracefully(t, "bash")
}

func testCompletionDegradesGracefully(t *testing.T, shellName string) {
	cases := []struct {
		name     string
		settings string // "" means no settings.yaml at all
		noYq     bool
		noBinary bool
	}{
		{name: "no_yq_no_binary_falls_back_to_nothing", settings: "version: 1\nenvs:\n  dev:\n    project: /tmp\n", noYq: true, noBinary: true},
		{name: "missing_config", settings: ""},
		{name: "empty_envs", settings: "version: 1\nenvs: {}\n"},
		{name: "invalid_yaml", settings: "this: [is not\n  valid yaml"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, ".env-switcher")
			if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
				t.Fatal(err)
			}
			if c.settings != "" {
				if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(c.settings), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if !c.noBinary {
				if err := os.WriteFile(filepath.Join(dir, "bin", "env-switcher"), builtEnvSwitcher(t), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			wrapper, ok := installer.Wrapper(shellName)
			if !ok {
				t.Fatalf("no wrapper template for %s", shellName)
			}
			path := os.Getenv("PATH")
			if c.noYq {
				path = pathWithoutYq(t)
			}
			var script string
			var cmd *exec.Cmd
			switch shellName {
			case "zsh":
				script = wrapper + "\n_env_switcher_completion; echo \"COMPLETION_EXIT=$?\"\n"
				cmd = exec.Command("zsh", "-f", "-c", script)
			case "bash":
				script = wrapper + "\nCOMP_WORDS=(env-switcher \"\"); COMP_CWORD=1\n_env_switcher_completion; echo \"COMPLETION_EXIT=$? REPLY=${COMPREPLY[*]}\"\n"
				cmd = exec.Command("bash", "--noprofile", "--norc", "-c", script)
			}
			cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+path)
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("completion function errored: %v stderr=%s", err, stderr.String())
			}
			if !strings.Contains(out.String(), "COMPLETION_EXIT=0") {
				t.Fatalf("completion did not exit cleanly: stdout=%q stderr=%q", out.String(), stderr.String())
			}
			if strings.Contains(out.String(), "dev") {
				t.Fatalf("completion produced a candidate it shouldn't have: %q", out.String())
			}
			if stderr.String() != "" {
				t.Fatalf("completion printed a terminal error: %q", stderr.String())
			}
		})
	}
}
