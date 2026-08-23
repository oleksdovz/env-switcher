package config

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

func FunctionDigest(s *Settings) string {
	h := sha256.New()
	write := func(scope, name, body string) {
		_, _ = h.Write([]byte(scope + "\x00" + name + "\x00" + body))
		_, _ = h.Write([]byte{0})
	}
	// "shell-cmd" can never collide with a real function name: identifierRE forbids hyphens.
	if s.Shared.ShellCmd != nil {
		write("shared", "shell-cmd", *s.Shared.ShellCmd)
	}
	keys := sortedFunctionKeys(s.Shared.ShellFunctions)
	for _, name := range keys {
		write("shared", name, s.Shared.ShellFunctions[name])
	}
	projects := make([]string, 0, len(s.Envs))
	for name := range s.Envs {
		projects = append(projects, name)
	}
	sort.Strings(projects)
	for _, project := range projects {
		env := s.Envs[project]
		if env.ShellCmd != nil {
			write("envs."+project, "shell-cmd", *env.ShellCmd)
		}
		for _, name := range sortedFunctionKeys(env.ShellFunctions) {
			write("envs."+project, name, env.ShellFunctions[name])
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// HasFunctions reports whether selecting a project could execute any user-provided code —
// named shell-functions or the anonymous shell-cmd hook — so the caller knows whether the
// first-run/changed-function trust warning applies.
func HasFunctions(s *Settings) bool {
	if s.Shared.ShellCmd != nil || len(s.Shared.ShellFunctions) > 0 {
		return true
	}
	for _, project := range s.Envs {
		if project.ShellCmd != nil || len(project.ShellFunctions) > 0 {
			return true
		}
	}
	return false
}

func sortedFunctionKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
