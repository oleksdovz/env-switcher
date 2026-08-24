package environment

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dolf/env-switcher/internal/config"
)

// Resolve does not change or depend on the current directory: `project` in settings.yaml is
// informational (shown by `list`/`get`) but switching never runs `cd`, so a project's directory
// need not exist and this never fails because of that. It does, however, need to turn into a
// clean absolute path: that's what the reserved config.ProjectVarName ("_PROJECT") variable
// holds, always, for every resolve — see resolveProjectPath. An empty or unresolved project path
// fails Resolve for that reason, even though a nonexistent one does not.
//
// It also does not track or remove names a previously active project set: each switch only
// applies the selected project's own variables/functions, and never unsets anything left over
// from a prior selection. _PROJECT is no exception to that — like every other resolved variable,
// it's simply included fresh in every switch's payload, so it's always current after a successful
// switch; there is no separate removal step for it or anything else.
//
// Resolution order (see also shell.Render, which the resulting Effective feeds): 1) resolve
// env.project, 2) define _PROJECT, 3) resolve shared and project env-vars — substituting
// $_PROJECT/${_PROJECT} references into their values — 4) collect shell functions, 5) collect
// shell-cmd hooks. Functions/shell-cmd bodies are not text-substituted: they see _PROJECT as an
// ordinary exported shell variable, since the rendered script always exports variables (_PROJECT
// included) before running shell-cmd, and a function body only ever runs later, when called.
func Resolve(settings *config.Settings, projectName, shell string) (*Effective, error) {
	project, ok := settings.Envs[projectName]
	if !ok {
		return nil, fmt.Errorf("project %q is not configured", projectName)
	}
	if shell != "bash" && shell != "zsh" {
		return nil, fmt.Errorf("unsupported shell %q", shell)
	}
	projectDir, err := resolveProjectPath(project.Project)
	if err != nil {
		return nil, fmt.Errorf("envs.%s.project: %w", projectName, err)
	}

	vars := make(map[string]string, len(settings.Shared.EnvVars)+len(project.EnvVars)+1)
	for k, v := range settings.Shared.EnvVars {
		vars[k] = config.ExpandVar(v, config.ProjectVarName, projectDir)
	}
	for k, v := range project.EnvVars {
		vars[k] = config.ExpandVar(v, config.ProjectVarName, projectDir)
	}
	vars[config.ProjectVarName] = projectDir

	funcs := make(map[string]string, len(settings.Shared.ShellFunctions)+len(project.ShellFunctions))
	for k, v := range settings.Shared.ShellFunctions {
		funcs[k] = v
	}
	for k, v := range project.ShellFunctions {
		funcs[k] = v
	}
	out := &Effective{Project: projectName, Shell: shell}
	if settings.Shared.ShellCmd != nil {
		out.ShellCmds = append(out.ShellCmds, *settings.Shared.ShellCmd)
	}
	if project.ShellCmd != nil {
		out.ShellCmds = append(out.ShellCmds, *project.ShellCmd)
	}
	for _, k := range sortedStringKeys(vars) {
		out.Variables = append(out.Variables, Variable{Name: k, Value: vars[k]})
	}
	for _, k := range sortedStringKeys(funcs) {
		out.Functions = append(out.Functions, Function{Name: k, Body: funcs[k]})
	}
	return out, nil
}

// resolveProjectPath turns a configured `project` path into the value config.ProjectVarName
// ("_PROJECT") gets: expand a leading "~"/"~/" or a "$HOME"/"${HOME}" reference via
// config.ExpandHome (the application's one controlled, no-shell path-expansion mechanism — no
// other variable, no command substitution, no globbing), then clean it into an absolute path.
// Any other content passes through unexpanded, so a path that doesn't resolve to something
// absolute (empty, relative, or otherwise unresolved) is rejected rather than silently used.
func resolveProjectPath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("project path is empty")
	}
	expanded, err := config.ExpandHome(raw)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(expanded) == "" {
		return "", fmt.Errorf("project path resolves to an empty value")
	}
	cleaned := filepath.Clean(expanded)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("project path %q does not resolve to an absolute path (expected it to start with / after ~ or $HOME expansion)", raw)
	}
	return cleaned, nil
}

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
