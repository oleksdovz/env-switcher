package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var identifierRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// functionNameMetachars are the characters that would change what a shell actually parses/runs if
// they appeared unquoted in a function name — the name is spliced verbatim into `name() { body }`
// in the real (non-`-n`) activation script, so any of these could turn one function definition
// into something else entirely: a statement separator (making "k;load" define a function named
// "load" and separately try to run bare command "k" — bash -n reports that as syntactically fine,
// since it's parsing two valid statements, not one invalid name), a redirection/pipe, a quote or
// backtick or "$" that starts an expansion, a brace or bracket, or whitespace/control bytes.
//
// Everything else is allowed, including any Unicode letter and shell-safe punctuation that isn't
// in this list (bash and zsh both accept far more in a function name than a hand-maintained
// allowlist could keep up with — confirmed empirically: unicode letters, emoji, and "+@%~#!"
// all define a function under the exact literal name given). shell.ValidateFunction's actual
// bash -n/zsh -n parse against the real target shell remains the final authority beyond this.
const functionNameMetachars = " \t\n\r\v\f;&|()<>*?[]\\\"'`${}="

// validFunctionNameStart reports whether r may start a function name: a letter (any script) or
// underscore — never a digit or punctuation, so a name can never be confused with an option flag
// or a number, matching ordinary naming convention.
func validFunctionNameStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }

func isValidFunctionName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 && !validFunctionNameStart(r) {
			return false
		}
		if r == utf8.RuneError || r < 0x20 || r == 0x7f || strings.ContainsRune(functionNameMetachars, r) {
			return false
		}
	}
	return true
}

// ReservedProjectNames are CLI keywords that double as the bare-word "switch to <env>" form
// (e.g. `env-switcher list`). A project sharing one of these names would be unreachable through
// that form, so configuration rejects the collision up front. internal/app reuses this list so
// dispatch and validation never drift apart.
var ReservedProjectNames = map[string]struct{}{
	"help": {}, "list": {}, "ls": {}, "edit": {}, "get": {}, "version": {},
	"validate": {}, "install": {}, "rollback": {}, "uninstall": {}, "reload": {}, "view": {},
	"upgrade": {},
}

func IsReservedProjectName(name string) bool {
	_, ok := ReservedProjectNames[name]
	return ok
}

// ProjectVarName is the reserved, application-managed environment variable holding an
// environment's resolved (expanded, cleaned, absolute) `project` directory — see
// internal/environment.Resolve. A manually declared value under this name in either shared or
// project env-vars is accepted by validation (so existing configuration written before this
// variable existed keeps loading) but is always overwritten by the computed value at resolve
// time; it is never an error.
const ProjectVarName = "_PROJECT"

func Validate(s *Settings) error {
	if s.Version != SchemaVersion {
		return fmt.Errorf("version: unsupported schema version %d (expected %d)", s.Version, SchemaVersion)
	}
	if len(s.Envs) == 0 || len(s.Envs) > MaxProjects {
		return fmt.Errorf("envs: must contain 1-%d projects", MaxProjects)
	}
	if err := validateDefinitions("shared", s.Shared.EnvVars, s.Shared.ShellFunctions); err != nil {
		return err
	}
	if err := validateShellCmd("shared.shell-cmd", s.Shared.ShellCmd); err != nil {
		return err
	}
	for _, name := range sortedKeys(s.Envs) {
		project := s.Envs[name]
		if len(name) == 0 || len(name) > 64 {
			return fmt.Errorf("envs: project name must contain 1-64 characters")
		}
		if IsReservedProjectName(name) {
			return fmt.Errorf("envs.%s: project name is a reserved CLI keyword", name)
		}
		if strings.TrimSpace(project.Project) == "" {
			return fmt.Errorf("envs.%s.project: path is required", name)
		}
		if strings.ContainsRune(project.Project, 0) {
			return fmt.Errorf("envs.%s.project: NUL is not allowed", name)
		}
		if err := validateDefinitions("envs."+name, project.EnvVars, project.ShellFunctions); err != nil {
			return err
		}
		if err := validateShellCmd("envs."+name+".shell-cmd", project.ShellCmd); err != nil {
			return err
		}
		for _, k := range sortedKeys(project.EnvVars) {
			if _, ok := s.Shared.ShellFunctions[k]; ok {
				return fmt.Errorf("envs.%s.%s: variable conflicts with shared function", name, k)
			}
		}
		for _, k := range sortedKeys(project.ShellFunctions) {
			if _, ok := s.Shared.EnvVars[k]; ok {
				return fmt.Errorf("envs.%s.%s: function conflicts with shared variable", name, k)
			}
		}
	}
	return nil
}

func validateDefinitions(path string, vars map[string]string, funcs map[string]string) error {
	if len(vars) > MaxDefinitions || len(funcs) > MaxDefinitions {
		return fmt.Errorf("%s: definition limit is %d per kind", path, MaxDefinitions)
	}
	for _, name := range sortedKeys(vars) {
		value := vars[name]
		if err := validateVarName(path+".env-vars", name); err != nil {
			return err
		}
		if len(value) > MaxValueSize || strings.ContainsRune(value, 0) || !utf8.ValidString(value) {
			return fmt.Errorf("%s.env-vars.%s: value is invalid or too large", path, name)
		}
	}
	for _, name := range sortedKeys(funcs) {
		if err := validateFunctionName(path+".shell-functions", name); err != nil {
			return err
		}
		if err := validateBody(path+".shell-functions."+name, funcs[name]); err != nil {
			return err
		}
	}
	for _, name := range sortedKeys(vars) {
		if _, ok := funcs[name]; ok {
			return fmt.Errorf("%s.%s: name cannot be both variable and function", path, name)
		}
	}
	return nil
}

// validateShellCmd applies the same body check as a named shell function, since shell-cmd is
// exactly as much trusted executable code — it just has no name and isn't invoked on demand.
func validateShellCmd(path string, cmd *string) error {
	if cmd == nil {
		return nil
	}
	return validateBody(path, *cmd)
}

func validateBody(path, body string) error {
	if body == "" || len(body) > MaxFunctionSize || strings.ContainsRune(body, 0) || !utf8.ValidString(body) {
		return fmt.Errorf("%s: body is invalid or too large", path)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func validateVarName(path, name string) error {
	if !identifierRE.MatchString(name) || strings.HasPrefix(name, "__ENV_SWITCHER_") {
		return fmt.Errorf("%s.%s: invalid or reserved identifier", path, name)
	}
	return nil
}

func validateFunctionName(path, name string) error {
	if !isValidFunctionName(name) || strings.HasPrefix(name, "__ENV_SWITCHER_") {
		return fmt.Errorf("%s.%s: invalid or reserved function name", path, name)
	}
	return nil
}
