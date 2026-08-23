package shell

import (
	"fmt"
	"github.com/dolf/env-switcher/internal/environment"
)

func RenderZsh(e *environment.Effective) (string, error) {
	if e.Shell != "zsh" {
		return "", fmt.Errorf("Zsh renderer received %q environment", e.Shell)
	}
	return Render(e)
}
