package shell

import (
	"fmt"
	"github.com/dolf/env-switcher/internal/environment"
)

func RenderBash(e *environment.Effective) (string, error) {
	if e.Shell != "bash" {
		return "", fmt.Errorf("Bash renderer received %q environment", e.Shell)
	}
	return Render(e)
}
