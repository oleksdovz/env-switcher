package environment

import (
	"fmt"
	"sort"

	"github.com/dolf/env-switcher/internal/config"
)

// Resolve does not change or depend on the current directory: `project` in settings.yaml is
// informational (shown by `list`/`get`) but switching never runs `cd`, so a project's directory
// need not exist and this never fails because of it.
//
// It also does not track or remove names a previously active project set: each switch only
// applies the selected project's own variables/functions, and never unsets anything left over
// from a prior selection.
func Resolve(settings *config.Settings, projectName, shell string) (*Effective, error) {
	project, ok := settings.Envs[projectName]
	if !ok {
		return nil, fmt.Errorf("project %q is not configured", projectName)
	}
	if shell != "bash" && shell != "zsh" {
		return nil, fmt.Errorf("unsupported shell %q", shell)
	}
	vars := make(map[string]string, len(settings.Shared.EnvVars)+len(project.EnvVars))
	for k, v := range settings.Shared.EnvVars {
		vars[k] = v
	}
	for k, v := range project.EnvVars {
		vars[k] = v
	}
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

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
