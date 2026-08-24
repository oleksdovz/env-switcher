package app

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/dolf/env-switcher/internal/config"
	installer "github.com/dolf/env-switcher/internal/install"
	tuit "github.com/dolf/env-switcher/internal/tui"
	"github.com/dolf/env-switcher/internal/upgrade"
)

func tuiCommand(ctx context.Context, currentVersion string, stdin io.Reader, stdout, stderr io.Writer) error {
	path, _, err := config.Bootstrap()
	if err != nil {
		return err
	}
	settings, err := config.Load(path)
	if err != nil {
		return err
	}
	// The same Upgrader instance upgradeCommand uses for "upgrade"/"--upgrade": F6 must not
	// duplicate that logic, only trigger it.
	upgrader := newUpgrader()
	services := tuit.Services{Reload: func() (*config.Settings, error) { return reloadSettings(path, settings) }, Install: func() error {
		shellName := detectShell()
		target, err := installer.Resolve(shellName, "")
		if err != nil {
			return err
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		_, err = installer.Install(target, exe)
		return err
	}, Upgrade: func() (upgrade.Result, error) {
		return upgrader.Upgrade(ctx, currentVersion)
	}}
	model := tuit.New(settings, path, services)
	// Nothing captures stdout via command substitution anymore (the wrapper just runs the
	// binary and separately sources current-env), so the TUI renders on stdout like an
	// ordinary terminal program.
	program := tea.NewProgram(model, tea.WithInput(stdin), tea.WithOutput(stdout))
	final, err := program.Run()
	if err != nil {
		return fmt.Errorf("terminal interface: %w", err)
	}
	selected := final.(tuit.Model).Selected
	if selected == "" {
		return nil
	}
	// stdin is nil here deliberately: the TUI's own trust dialog (see internal/tui/model.go) is
	// what gates reaching a selection at all when acknowledgment is outstanding, so this call
	// should never actually need to prompt — passing nil keeps confirmTrustedFunctions from ever
	// trying to block-read the terminal Bubble Tea just released.
	return switchCommand(detectShell(), selected, nil, stdout, stderr)
}
