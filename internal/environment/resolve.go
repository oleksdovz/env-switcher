package environment

import (
	"fmt"
	"os"
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
// env.project, 2) define _PROJECT, 3) merge shared and project env-vars (project overrides
// shared, same key wins) and resolve $NAME/${NAME} references within and across them — see
// resolveVarRefs — 4) collect shell functions, 5) collect shell-cmd hooks. Functions/shell-cmd
// bodies are not text-substituted: they see _PROJECT (and every other resolved env-var) as an
// ordinary exported shell variable, since the rendered script always exports variables before
// running shell-cmd, and a function body only ever runs later, when called.
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
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home: %w", err)
	}

	raw := make(map[string]string, len(settings.Shared.EnvVars)+len(project.EnvVars))
	for k, v := range settings.Shared.EnvVars {
		raw[k] = v
	}
	for k, v := range project.EnvVars {
		raw[k] = v
	}
	vars, err := resolveVarRefs(raw, projectDir, home)
	if err != nil {
		return nil, fmt.Errorf("envs.%s.env-vars: %w", projectName, err)
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

// resolveVarRefs resolves "$name"/"${name}" references inside raw's values — via
// config.ExpandVar, so still no command substitution, no globbing, no shell of any kind — where
// name may be: "HOME" (the current user's home directory), "_PROJECT" or "PROJECT" (both alias
// the same resolved project directory projectDir; "PROJECT" exists only so a value can reference
// it without the leading underscore), or the name of another key in raw itself. A reference to
// any other name isn't a mistake — it just isn't something this function knows how to resolve, so
// it passes through unexpanded, same as config.ExpandHome already documents for its callers.
//
// Resolution follows each value's actual dependencies, not map iteration or declaration order:
// a variable that references another still-unresolved variable is resolved only after that
// dependency is, however many keys apart they are or whichever was declared first. A reference
// cycle (direct or transitive) is reported as an error — with the chain that closed it — rather
// than looped on or silently left unresolved.
func resolveVarRefs(raw map[string]string, projectDir, home string) (map[string]string, error) {
	const (
		unvisited = iota
		inProgress
		done
	)
	resolved := make(map[string]string, len(raw))
	state := make(map[string]int, len(raw))

	var resolve func(name string, chain []string) (string, error)
	resolve = func(name string, chain []string) (string, error) {
		if v, ok := resolved[name]; ok {
			return v, nil
		}
		if state[name] == inProgress {
			return "", fmt.Errorf("circular reference: %s", strings.Join(append(chain, name), " -> "))
		}
		state[name] = inProgress
		val := raw[name]
		for _, ref := range config.ReferencedVarNames(val) {
			switch ref {
			case "HOME":
				val = config.ExpandVar(val, "HOME", home)
			case "_PROJECT", "PROJECT":
				val = config.ExpandVar(val, ref, projectDir)
			default:
				if _, ok := raw[ref]; ok {
					depVal, err := resolve(ref, append(chain, name))
					if err != nil {
						return "", err
					}
					val = config.ExpandVar(val, ref, depVal)
				}
			}
		}
		state[name] = done
		resolved[name] = val
		return val, nil
	}

	for name := range raw {
		if _, err := resolve(name, nil); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
