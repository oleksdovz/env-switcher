package app

import (
	"fmt"
	"io"
)

const helpText = `env-switcher manages per-project directories, environment variables, and trusted shell
functions for the current Bash or Zsh session.

Usage:
  env-switcher                  open the interactive project picker
  env-switcher <project>        switch directly to <project>
  env-switcher list | ls        list configured projects
  env-switcher get <project>    show one project's resolved configuration
  env-switcher edit [project]   open settings.yaml in $VISUAL/$EDITOR
  env-switcher validate         validate settings.yaml
  env-switcher reload           re-validate settings.yaml non-interactively
  env-switcher view             show the complete settings.yaml after confirmation
  env-switcher install          install or update shell integration
  env-switcher rollback         restore the previous shell profile backup
  env-switcher uninstall        remove the managed shell integration
  env-switcher upgrade          install the latest compatible release
  env-switcher version          print build metadata
  env-switcher help             show this help

Every command above also accepts an equivalent "--flag" form (--list, --get, --edit, --validate,
--reload, --view, --install, --rollback, --uninstall, --upgrade, --version, --select) for the
same action. TUI keys: Enter select, F2/v view, F3/e edit, F4/r reload, F5/i install, F6 upgrade,
F10/q exit.
`

func printHelp(stdout io.Writer) { _, _ = fmt.Fprint(stdout, helpText) }
