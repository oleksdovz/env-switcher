package app

import (
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/dolf/env-switcher/internal/config"
	installer "github.com/dolf/env-switcher/internal/install"
	tuit "github.com/dolf/env-switcher/internal/tui"
)

func tuiCommand(stdin io.Reader, stdout, stderr io.Writer) error {
	path, _, err := config.Bootstrap()
	if err != nil {
		return err
	}
	settings, err := config.Load(path)
	if err != nil {
		return err
	}
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
	return switchCommand(detectShell(), selected, stdout)
}
