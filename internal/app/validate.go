package app

import (
	"fmt"
	"io"

	"github.com/dolf/env-switcher/internal/config"
)

func validateCommand(stdout io.Writer) error {
	path, _, err := config.Bootstrap()
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "bootstrap", Message: err.Error()}
	}
	if err := config.EnsurePrivate(path, false); err != nil {
		return &Error{Outcome: OutcomeSecurity, Op: "validate", Message: err.Error()}
	}
	if _, err := config.Load(path); err != nil {
		return &Error{Outcome: OutcomeConfiguration, Op: "validate", Message: err.Error()}
	}
	_, _ = fmt.Fprintln(stdout, "settings are valid")
	return nil
}
