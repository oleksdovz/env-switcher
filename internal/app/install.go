package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dolf/env-switcher/internal/config"
	installer "github.com/dolf/env-switcher/internal/install"
)

// migrateLegacyExecutable removes an executable left over at the pre-bin/ install location
// (~/.env-switcher/env-switcher, before the bin/ subdirectory convention) once the canonical
// bin/env-switcher copy is confirmed in place. Best-effort and non-fatal, matching the rest of
// this file's self-install conveniences: a stale leftover file is litter, never a reason to fail
// an otherwise-successful install.
func migrateLegacyExecutable(stdout, stderr io.Writer) {
	dataDir, err := config.DataDir()
	if err != nil {
		return
	}
	removed, err := installer.MigrateLegacyExecutable(dataDir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "note:", err)
		return
	}
	if removed {
		canonical, err := config.ExecutablePath()
		if err != nil {
			canonical = "~/.env-switcher/bin/env-switcher"
		}
		_, _ = fmt.Fprintf(stdout, "removed superseded executable at %s (now installed at %s)\n",
			installer.LegacyExecutablePath(dataDir), canonical)
	}
}

// actionVerb reads naturally as the lead line of each command's detail block ("install will:",
// "rollback will:", "uninstall will:").
var actionVerb = map[string]string{"install": "install", "rollback": "restore", "uninstall": "remove"}

func installCommand(action string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	shellName, profile, yes, err := installArgs(args)
	if err != nil {
		return err
	}
	target, err := installer.Resolve(shellName, profile)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stderr, "→ %s will:\n", actionVerb[action])
	_, _ = fmt.Fprintf(stderr, "  shell             %s\n", target.Shell)
	_, _ = fmt.Fprintf(stderr, "  profile           %s\n", target.Profile)
	if action == "install" {
		_, _ = fmt.Fprintf(stderr, "  executable        %s\n", target.Executable)
	}
	if !yes {
		_, _ = fmt.Fprintf(stderr, "\n%s shell integration in %s? [y/N] ", action, target.Profile)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			return &Error{Outcome: OutcomeCancelled, Op: action, Message: "cancelled"}
		}
	}
	_, _ = fmt.Fprintln(stderr)
	switch action {
	case "install":
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		result, err := installer.Install(target, exe)
		if err != nil {
			return err
		}
		if result.Changed {
			_, _ = fmt.Fprintf(stdout, "✓ installed integration in %s", result.Profile)
			if result.Backup != "" {
				_, _ = fmt.Fprintf(stdout, " (backup %s)", result.Backup)
			}
			_, _ = fmt.Fprintln(stdout)
		} else {
			_, _ = fmt.Fprintln(stdout, "✓ integration is already up to date")
		}
		migrateLegacyExecutable(stdout, stderr)
	case "rollback":
		if err := installer.Rollback(target); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout, "✓ profile restored from verified backup")
	case "uninstall":
		if err := installer.Uninstall(target); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout, "✓ managed integration removed")
	}
	return nil
}

func installArgs(args []string) (shellName, profile string, yes bool, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--shell":
			if i+1 >= len(args) {
				err = fmt.Errorf("--shell requires a value")
				return
			}
			i++
			shellName = args[i]
		case "--profile":
			if i+1 >= len(args) {
				err = fmt.Errorf("--profile requires a value")
				return
			}
			i++
			profile = args[i]
		case "--yes":
			yes = true
		default:
			err = fmt.Errorf("unexpected argument %q", args[i])
			return
		}
	}
	if shellName == "" {
		shellName = detectShell()
	}
	return
}

func detectShell() string {
	if target := os.Getenv("__ENV_SWITCHER_TARGET_SHELL"); target == "bash" || target == "zsh" {
		return target
	}
	value := os.Getenv("SHELL")
	if strings.HasSuffix(value, "/bash") {
		return "bash"
	}
	if strings.HasSuffix(value, "/zsh") {
		return "zsh"
	}
	return ""
}
