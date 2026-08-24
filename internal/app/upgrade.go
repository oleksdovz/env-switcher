package app

import (
	"context"
	"fmt"
	"io"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/upgrade"
)

// newUpgrader builds the single Upgrader instance both the "upgrade"/"--upgrade" CLI command and
// the TUI's F6 shortcut drive — see tuiCommand's Services.Upgrade — so the two never duplicate
// the actual upgrade logic (network access, checksum verification, extraction, install).
func newUpgrader() *upgrade.Upgrader {
	return upgrade.NewUpgrader(config.ExecutablePath)
}

// upgradeCommand implements the "upgrade"/"--upgrade" CLI action: find, verify, and install the
// latest compatible stable release, reporting outcome (or failure) as plain stdout/stderr text —
// never anything the shell wrapper could mistake for the switch payload it sources from
// current-env (see switch.go and the installed wrapper templates). u is injected (rather than
// built internally) so tests can point it at a fixture instead of live GitHub.
func upgradeCommand(ctx context.Context, u *upgrade.Upgrader, currentVersion string, stdout, stderr io.Writer) error {
	result, err := u.Upgrade(ctx, currentVersion)
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "upgrade", Message: err.Error()}
	}
	if result.AlreadyCurrent {
		_, _ = fmt.Fprintf(stdout, "env-switcher is already up to date (%s)\n", result.NewVersion)
		return nil
	}
	_, _ = fmt.Fprintf(stdout, "upgraded env-switcher %s -> %s\n", result.OldVersion, result.NewVersion)
	_, _ = fmt.Fprintf(stdout, "installed at %s\n", result.InstalledPath)
	migrateLegacyExecutable(stdout, stderr)
	return nil
}
