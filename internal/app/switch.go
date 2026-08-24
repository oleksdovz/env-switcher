package app

import (
	"fmt"
	"io"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/environment"
	"github.com/dolf/env-switcher/internal/fsatomic"
	"github.com/dolf/env-switcher/internal/shell"
)

// switchCommand resolves project for shellName and writes the resulting shell transaction to the
// fixed current-env file. Used both by the bare-CLI switch form and by the TUI after a project is
// picked — the only two paths (along with `--select`) that are ever expected to produce this
// payload; see the installed wrapper templates, which clear current-env before every invocation
// and source it afterward only if this call recreated it, so every other command (help, list,
// version, upgrade, ...) leaves the running shell untouched.
//
// On any failure it leaves current-env exactly as it found it — which, since the wrapper always
// clears it first, means it stays absent. "No change on failure" here means the wrapper has
// nothing to source, not "re-apply the last successful switch": once a switch has failed, the
// values already live in the current shell (from whenever, if ever, it last switched
// successfully) are left alone rather than redundantly reapplied.
func switchCommand(shellName, project string, stdout io.Writer) error {
	if shellName != "bash" && shellName != "zsh" {
		return &Error{Outcome: OutcomeCompatibility, Op: "switch", Message: "supported shells are bash and zsh"}
	}
	if project == "" {
		return &Error{Outcome: OutcomeCompatibility, Op: "switch", Message: "project is required"}
	}
	path, _, err := config.Bootstrap()
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "switch", Message: err.Error()}
	}
	settings, err := config.Load(path)
	if err != nil {
		return &Error{Outcome: OutcomeConfiguration, Op: "switch", Message: err.Error()}
	}
	if config.HasFunctions(settings) && !config.IsAcknowledged(config.FunctionDigest(settings)) {
		return &Error{Outcome: OutcomeSecurity, Op: "switch", Message: "trusted shell functions changed; open env-switcher and acknowledge the warning"}
	}
	effective, err := environment.Resolve(settings, project, shellName)
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "switch", Message: err.Error()}
	}
	for _, fn := range effective.Functions {
		if err := shell.ValidateFunction(shellName, fn.Name, fn.Body); err != nil {
			return &Error{Outcome: OutcomeConfiguration, Op: "switch", Message: err.Error()}
		}
	}
	for i, cmd := range effective.ShellCmds {
		// shell-cmd is anonymous; the name here only labels the syntax probe, it's discarded.
		if err := shell.ValidateFunction(shellName, fmt.Sprintf("__shell_cmd_%d__", i), cmd); err != nil {
			return &Error{Outcome: OutcomeConfiguration, Op: "switch", Message: "shell-cmd has invalid syntax: " + err.Error()}
		}
	}
	script, err := shell.Render(effective)
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "switch", Message: "could not render activation script"}
	}
	currentEnv, err := config.CurrentEnvPath()
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "switch", Message: err.Error()}
	}
	if err := fsatomic.WriteFile(currentEnv, []byte(script), 0o600); err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "switch", Message: err.Error()}
	}
	_, _ = fmt.Fprintf(stdout, "switched to %s\n", project)
	return nil
}

// switchArgs parses `[--shell bash|zsh] <project>` used by the bare-CLI switch form.
func switchArgs(args []string) (shellName, project string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--shell":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--shell requires a value")
			}
			i++
			shellName = args[i]
		default:
			if project != "" {
				return "", "", fmt.Errorf("unexpected argument %q", args[i])
			}
			project = args[i]
		}
	}
	return resolveShellArg(shellName), project, nil
}

func resolveShellArg(explicit string) string {
	if explicit == "bash" || explicit == "zsh" {
		return explicit
	}
	return detectShell()
}
