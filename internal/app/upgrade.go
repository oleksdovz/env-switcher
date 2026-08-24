package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/upgrade"
)

// newUpgrader builds the single Upgrader instance both the "upgrade"/"--upgrade" CLI command and
// the TUI's F6 shortcut drive — see tuiCommand's Services.Upgrade — so the two never duplicate
// the actual upgrade logic (network access, checksum verification, extraction, install).
func newUpgrader() *upgrade.Upgrader {
	return upgrade.NewUpgrader(config.ExecutablePath)
}

// upgradeCommand implements the "upgrade"/"--upgrade" CLI action: check what's available, show
// it, ask before changing anything (unless --yes), then find, verify, and install the latest
// compatible stable release — narrating each step. All of that goes to stderr (it's diagnostic/
// interactive, not a stable machine-parseable output); the final one-line outcome stays on
// stdout, matching every other command. Neither ever writes anything the shell wrapper could
// mistake for the switch payload it sources from current-env (see switch.go and the installed
// wrapper templates) — internal/upgrade never touches that file at all. u is injected (rather
// than built internally) so tests can point it at a fixture instead of live GitHub.
func upgradeCommand(ctx context.Context, u *upgrade.Upgrader, currentVersion string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	yes, err := upgradeArgs(args)
	if err != nil {
		return &Error{Outcome: OutcomeCompatibility, Op: "upgrade", Message: err.Error()}
	}

	_, _ = fmt.Fprintf(stderr, "→ checking %s for the latest release\n", upgrade.SourceURL)
	check, err := u.Check(ctx, currentVersion)
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "upgrade", Message: err.Error()}
	}
	_, _ = fmt.Fprintf(stderr, "  current version   %s\n", check.CurrentLabel)
	_, _ = fmt.Fprintf(stderr, "  latest release    %s\n", check.NewVersion)
	_, _ = fmt.Fprintf(stderr, "  platform          %s\n", u.Platform)

	if !check.UpgradeAvailable {
		_, _ = fmt.Fprintf(stdout, "✓ already up to date (%s)\n", check.NewVersion)
		return nil
	}

	if !yes {
		_, _ = fmt.Fprintf(stderr, "\nUpgrade %s -> %s? [y/N] ", check.CurrentLabel, check.NewVersion)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			return &Error{Outcome: OutcomeCancelled, Op: "upgrade", Message: "skipped (" + check.CurrentLabel + " -> " + check.NewVersion + " available)"}
		}
	}

	_, _ = fmt.Fprintln(stderr)
	u.Progress = func(stage string) { _, _ = fmt.Fprintf(stderr, "→ %s\n", stage) }
	result, err := u.Apply(ctx, check.Release, check.CurrentLabel)
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "upgrade", Message: err.Error()}
	}
	_, _ = fmt.Fprintf(stdout, "✓ upgraded %s -> %s\n", result.OldVersion, result.NewVersion)
	_, _ = fmt.Fprintf(stdout, "  installed at %s\n", result.InstalledPath)
	migrateLegacyExecutable(stdout, stderr)
	return nil
}

func upgradeArgs(args []string) (yes bool, err error) {
	for _, a := range args {
		switch a {
		case "--yes", "-y":
			yes = true
		default:
			return false, fmt.Errorf("unexpected argument %q", a)
		}
	}
	return yes, nil
}
