package app

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

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
func switchCommand(shellName, project string, stdin io.Reader, stdout, stderr io.Writer) error {
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
	if _, ok := settings.Envs[project]; !ok {
		return unconfiguredProjectError(settings, project)
	}
	if config.HasFunctions(settings) && !config.IsAcknowledged(config.FunctionDigest(settings)) {
		if err := confirmTrustedFunctions(stdin, stderr, settings); err != nil {
			return err
		}
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

// confirmTrustedFunctions is the plain-CLI counterpart of the TUI's own trust dialog. Before this,
// `env-switcher <project>` hard-refused outright the first time (or any time after) a trusted
// shell-functions/shell-cmd body changed, forcing a detour through the TUI just to press `y` once
// — real friction for anyone who only ever uses the CLI form. This prompts right here instead
// (stderr, matching how install/upgrade already confirm inline) and, on "y", persists the
// acknowledgment via config.Acknowledge so the same content never asks again from either the CLI
// or the TUI — exactly one confirmation per actual function change, not one per session.
//
// stdin == nil (the TUI's call into switchCommand, after a form selection) means there is no safe
// way to block on a read here — Bubble Tea owns the terminal in raw mode at that point. The TUI
// instead gates entry with its own dialog before a selection can ever reach this far, so it keeps
// the old hard refusal as a redundant safety net, not a first line of defense.
func confirmTrustedFunctions(stdin io.Reader, stderr io.Writer, settings *config.Settings) error {
	digest := config.FunctionDigest(settings)
	if stdin == nil {
		return &Error{Outcome: OutcomeSecurity, Op: "switch", Message: "trusted shell functions changed; open env-switcher and acknowledge the warning"}
	}
	_, _ = fmt.Fprintln(stderr, "⚠ trusted shell functions or shell-cmd changed since you last confirmed them.")
	_, _ = fmt.Fprintln(stderr, "  Review with `env-switcher get <project>` or `env-switcher edit` first if unsure.")
	_, _ = fmt.Fprint(stderr, "Trust and run them? [y/N] ")
	line, _ := bufio.NewReader(stdin).ReadString('\n')
	if strings.ToLower(strings.TrimSpace(line)) != "y" {
		return &Error{Outcome: OutcomeCancelled, Op: "switch", Message: "cancelled: trusted shell functions were not acknowledged"}
	}
	if err := config.Acknowledge(digest); err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "switch", Message: err.Error()}
	}
	_, _ = fmt.Fprintln(stderr, "✓ trusted function warning acknowledged")
	return nil
}

// unconfiguredProjectError builds an actionable error for a project name that isn't in settings —
// naming the mistake plainly, listing what actually is configured (so the fix is usually just
// picking the right name off the list), and pointing at `list` to see it again outside an error.
func unconfiguredProjectError(settings *config.Settings, project string) *Error {
	var b strings.Builder
	fmt.Fprintf(&b, "⚠ project %q is not configured", project)
	names := make([]string, 0, len(settings.Envs))
	for name := range settings.Envs {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		b.WriteString("\n\nNo projects are configured yet. Add one to settings.yaml (`env-switcher edit`), then run `env-switcher list` to confirm.")
	} else {
		b.WriteString("\n\nConfigured projects:\n")
		for _, name := range names {
			fmt.Fprintf(&b, "  - %s\n", name)
		}
		b.WriteString("\nRun `env-switcher list` (or `ls`) to see this list again.")
	}
	return &Error{Outcome: OutcomeOperation, Op: "switch", Message: b.String()}
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
