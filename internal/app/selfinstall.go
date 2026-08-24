package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/fsatomic"
	installer "github.com/dolf/env-switcher/internal/install"
)

// selfInstall implements "just run the downloaded binary". It only ever applies to the default
// (no-args) invocation, and only when the running process isn't already the installed copy at
// config.ExecutablePath() — i.e. it was started from dist/, ~/Downloads, or anywhere else.
//
//   - Never installed before (no executable at that fixed path yet): ask once, then create
//     ~/.env-switcher plus a starter settings.yaml, install shell integration for the detected
//     shell, and copy this binary into place.
//   - Already installed (a prior run put something at that path): silently keep it current —
//     copy the new binary over the old one and reconcile the managed rc block if its template
//     changed, exactly like re-running `env-switcher install` would, but without asking again.
//
// Any error here is reported to stderr and swallowed: self-install is a convenience, never a
// precondition for running the requested command.
func selfInstall(stdin io.Reader, stdout, stderr io.Writer) {
	current, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(current); err == nil {
		current = resolved
	}
	installedExe, err := config.ExecutablePath()
	if err != nil {
		return
	}
	comparable := installedExe
	if resolved, err := filepath.EvalSymlinks(installedExe); err == nil {
		comparable = resolved
	}
	if filepath.Clean(current) == filepath.Clean(comparable) {
		return // already running the installed copy
	}
	_, statErr := os.Stat(installedExe)
	if statErr == nil {
		selfUpdate(current, installedExe, stdout, stderr)
		return
	}
	selfInstallFresh(current, installedExe, stdin, stdout, stderr)
}

func selfUpdate(current, installedExe string, stdout, stderr io.Writer) {
	shellName := detectShell()
	if shellName == "bash" || shellName == "zsh" {
		target, err := installer.Resolve(shellName, "")
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return
		}
		result, err := installer.Install(target, current)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return
		}
		if result.Changed {
			_, _ = fmt.Fprintf(stdout, "updated env-switcher at %s\n", installedExe)
		}
		migrateLegacyExecutable(stdout, stderr)
		return
	}
	binary, err := os.ReadFile(current)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return
	}
	if err := fsatomic.WriteFile(installedExe, binary, 0o700); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return
	}
	_, _ = fmt.Fprintf(stdout, "updated env-switcher at %s\n", installedExe)
	migrateLegacyExecutable(stdout, stderr)
}

func selfInstallFresh(current, installedExe string, stdin io.Reader, stdout, stderr io.Writer) {
	dataDir, err := config.DataDir()
	if err != nil {
		return
	}
	shellName := detectShell()
	prompt := fmt.Sprintf("First run: create %s with a starter settings.yaml", dataDir)
	if shellName == "bash" || shellName == "zsh" {
		prompt += fmt.Sprintf(" and install %s shell integration", shellName)
	}
	_, _ = fmt.Fprintln(stderr, prompt+"? [y/N]")
	// A 1-byte-buffered reader so this confirmation can never consume bytes meant for the
	// TUI that starts reading the same stdin right after this function returns.
	line, _ := bufio.NewReaderSize(stdin, 1).ReadString('\n')
	if strings.ToLower(strings.TrimSpace(line)) != "y" {
		return
	}
	if _, _, err := config.Bootstrap(); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return
	}
	if shellName != "bash" && shellName != "zsh" {
		binary, err := os.ReadFile(current)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return
		}
		if err := fsatomic.WriteFile(installedExe, binary, 0o700); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return
		}
		_, _ = fmt.Fprintf(stdout, "created %s and copied env-switcher to %s\n", dataDir, installedExe)
		_, _ = fmt.Fprintln(stderr, "shell integration was not installed automatically (unsupported or undetected shell); run `env-switcher install --shell bash|zsh` manually.")
		return
	}
	target, err := installer.Resolve(shellName, "")
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return
	}
	if _, err := installer.Install(target, current); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return
	}
	_, _ = fmt.Fprintf(stdout, "created %s, installed env-switcher at %s, and added shell integration to %s\n", dataDir, installedExe, target.Profile)
	_, _ = fmt.Fprintf(stdout, "restart your shell or run `source %s` to start using env-switcher\n", target.Profile)
}
