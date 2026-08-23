package app

import (
	"bufio"
	"fmt"
	"io"
	"sort"

	"github.com/dolf/env-switcher/internal/config"
)

// getCommand prints one project's resolved (shared + project-specific) configuration,
// including unmasked variable values and function bodies. It stays script-friendly — no
// blocking confirmation, since piping `env-switcher get prod | grep REGION` must not hang on
// stdin — but still advertises the disclosure with a stderr advisory, consistent with the
// project's "never silently disclose secrets" posture (F2/F3 in the TUI).
func getCommand(args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 || args[0] == "" {
		return &Error{Outcome: OutcomeCompatibility, Op: "get", Message: "usage: env-switcher get <project>"}
	}
	name := args[0]
	path, _, err := config.Bootstrap()
	if err != nil {
		return &Error{Outcome: OutcomeOperation, Op: "get", Message: err.Error()}
	}
	settings, err := config.Load(path)
	if err != nil {
		return &Error{Outcome: OutcomeConfiguration, Op: "get", Message: err.Error()}
	}
	project, ok := settings.Envs[name]
	if !ok {
		return &Error{Outcome: OutcomeOperation, Op: "get", Message: fmt.Sprintf("project %q is not configured", name)}
	}
	_, _ = fmt.Fprintln(stderr, "Warning: settings.yaml may contain sensitive values; the values below are unmasked.")

	w := bufio.NewWriter(stdout)
	defer w.Flush()
	_, _ = fmt.Fprintf(w, "project: %s\ndirectory: %s\n", name, project.Project)

	vars := make(map[string]string, len(settings.Shared.EnvVars)+len(project.EnvVars))
	for k, v := range settings.Shared.EnvVars {
		vars[k] = v
	}
	for k, v := range project.EnvVars {
		vars[k] = v
	}
	if len(vars) > 0 {
		_, _ = fmt.Fprintln(w, "env-vars:")
		for _, k := range sortedKeys(vars) {
			_, _ = fmt.Fprintf(w, "  %s=%s\n", k, vars[k])
		}
	}

	funcs := make(map[string]string, len(settings.Shared.ShellFunctions)+len(project.ShellFunctions))
	for k, v := range settings.Shared.ShellFunctions {
		funcs[k] = v
	}
	for k, v := range project.ShellFunctions {
		funcs[k] = v
	}
	if len(funcs) > 0 {
		_, _ = fmt.Fprintln(w, "shell-functions:")
		for _, k := range sortedKeys(funcs) {
			_, _ = fmt.Fprintf(w, "  %s:\n", k)
			printBody(w, funcs[k])
		}
	}

	if settings.Shared.ShellCmd != nil || project.ShellCmd != nil {
		_, _ = fmt.Fprintln(w, "shell-cmd:")
		if settings.Shared.ShellCmd != nil {
			_, _ = fmt.Fprintln(w, "  shared:")
			printBody(w, *settings.Shared.ShellCmd)
		}
		if project.ShellCmd != nil {
			_, _ = fmt.Fprintf(w, "  %s:\n", name)
			printBody(w, *project.ShellCmd)
		}
	}
	return nil
}

func printBody(w io.Writer, body string) {
	for _, line := range splitLines(body) {
		_, _ = fmt.Fprintf(w, "    %s\n", line)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
