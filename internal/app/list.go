package app

import (
	"fmt"
	"io"
	"sort"

	"github.com/dolf/env-switcher/internal/config"
)

// listCommand prints every configured project and its directory. Paths are not treated as
// secrets, so this never warns and is safe to script against.
func listCommand(stdout io.Writer) error {
	path, _, err := config.Bootstrap()
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "list", Message: err.Error()}
	}
	settings, err := config.Load(path)
	if err != nil {
		return &Error{Outcome: OutcomeConfiguration, Op: "list", Message: err.Error()}
	}
	names := make([]string, 0, len(settings.Envs))
	for name := range settings.Envs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", name, settings.Envs[name].Project)
	}
	return nil
}
