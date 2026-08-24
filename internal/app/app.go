package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dolf/env-switcher/internal/config"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type App struct {
	build BuildInfo
}

func New(build BuildInfo) *App { return &App{build: build} }

// Run dispatches a CLI invocation. Every command below accepts a bare word (`list`) and, where
// noted, an equivalent `--flag` form (`--list`), so the installed shell wrapper can forward
// "$@" verbatim regardless of which style the user typed. Any word that isn't a recognized
// command is treated as a project name to switch to directly (config.Validate rejects project
// names that would collide with a reserved command word, so this is never ambiguous).
//
// The default (no-args) invocation additionally self-installs: see selfInstall.
func (a *App) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		selfInstall(stdin, stdout, stderr)
		if err := tuiCommand(ctx, a.build.Version, stdin, stdout, stderr); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	}
	switch args[0] {
	case "help", "--help", "-h":
		printHelp(stdout)
		return OutcomeSuccess.ExitCode()
	case "list", "--list", "ls":
		if err := listCommand(stdout); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	case "get", "--get":
		if err := getCommand(args[1:], stdout, stderr); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	case "edit", "--edit":
		if err := editCommand(args[1:], stderr); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	case "version", "--version":
		_, _ = fmt.Fprintf(stdout, "env-switcher %s commit=%s built=%s\n", a.build.Version, a.build.Commit, a.build.Date)
		return OutcomeSuccess.ExitCode()
	case "validate", "--validate":
		if err := validateCommand(stdout); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	case "view", "--view":
		if err := viewCommand(stdin, stdout, stderr); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	case "reload", "--reload":
		path, err := config.SettingsPath()
		if err == nil {
			_, err = reloadSettings(path, nil)
		}
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return OutcomeConfiguration.ExitCode()
		}
		_, _ = fmt.Fprintln(stdout, "projects reloaded")
		return OutcomeSuccess.ExitCode()
	case "install", "--install", "rollback", "--rollback", "uninstall", "--uninstall":
		action := strings.TrimPrefix(args[0], "--")
		if err := installCommand(action, args[1:], stdin, stdout, stderr); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	case "upgrade", "--upgrade":
		if err := upgradeCommand(ctx, newUpgrader(), a.build.Version, args[1:], stdin, stdout, stderr); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	case "--select":
		if err := switchDispatch(args[1:], stdout); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	default:
		if err := switchDispatch(args, stdout); err != nil {
			return report(stderr, err)
		}
		return OutcomeSuccess.ExitCode()
	}
}

// switchDispatch parses the plain-CLI switch form's arguments and runs it.
func switchDispatch(args []string, stdout io.Writer) error {
	shellName, project, err := switchArgs(args)
	if err != nil {
		return &Error{Outcome: OutcomeCompatibility, Op: "switch", Message: err.Error()}
	}
	return switchCommand(shellName, project, stdout)
}

func report(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	if appErr, ok := err.(*Error); ok {
		return appErr.Outcome.ExitCode()
	}
	return OutcomeOperation.ExitCode()
}
