package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/editor"
)

func viewCommand(stdin io.Reader, stdout, stderr io.Writer) error {
	path, err := config.SettingsPath()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stderr, "Warning: settings.yaml may contain sensitive values. Display complete unmasked file? [y/N]")
	line, _ := bufio.NewReader(stdin).ReadString('\n')
	if strings.ToLower(strings.TrimSpace(line)) != "y" {
		return &Error{Outcome: OutcomeCancelled, Op: "view", Message: "cancelled"}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	_, err = stdout.Write(b)
	return err
}

func editCommand(args []string, stderr io.Writer) error {
	if len(args) > 1 {
		return &Error{Outcome: OutcomeCompatibility, Op: "edit", Message: "usage: env-switcher edit [project]"}
	}
	path, _, err := config.Bootstrap()
	if err != nil {
		return err
	}
	if len(args) == 1 && args[0] != "" {
		if settings, loadErr := config.Load(path); loadErr == nil {
			if _, ok := settings.Envs[args[0]]; !ok {
				_, _ = fmt.Fprintf(stderr, "note: project %q was not found in settings.yaml\n", args[0])
			}
		}
	}
	return editor.Open(path)
}
