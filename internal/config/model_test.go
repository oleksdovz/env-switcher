package config

import (
	"fmt"
	"testing"
)

func TestValidateRejectsReservedAndCrossKindNames(t *testing.T) {
	body := "echo ok"
	tests := []Settings{
		{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", EnvVars: map[string]string{"__ENV_SWITCHER_X": "x"}}}},
		{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", EnvVars: map[string]string{"X": "x"}, ShellFunctions: map[string]string{"X": body}}}},
	}
	for i := range tests {
		if err := Validate(&tests[i]); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}

// TestUpgradeIsAReservedProjectName guards the "upgrade"/"--upgrade" CLI command: a project
// named "upgrade" would be unreachable through the bare-word switch form (`env-switcher
// upgrade` would run the upgrade command, not switch to a same-named project), so it must be
// rejected the same way "install"/"reload"/etc. already are.
func TestUpgradeIsAReservedProjectName(t *testing.T) {
	if !IsReservedProjectName("upgrade") {
		t.Fatal(`"upgrade" is not in ReservedProjectNames`)
	}
	s := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"upgrade": {Project: "/tmp"}}}
	if err := Validate(s); err == nil {
		t.Fatal("expected validation to reject a project literally named \"upgrade\"")
	}
}

// TestShellFunctionNamesAreDynamicNotAHardcodedAllowlist guards against re-applying an
// approximate, hand-maintained allowlist to function names: bash/zsh function names are command
// names, not variable identifiers, and accept far more than any fixed pattern is likely to
// enumerate — confirmed empirically against real bash/zsh (see internal/config/validate.go's
// functionNameMetachars), not guessed. Every one of these was rejected at some point by an
// earlier, stricter version of this same fix.
func TestShellFunctionNamesAreDynamicNotAHardcodedAllowlist(t *testing.T) {
	body := "echo ok"
	names := []string{
		"k-load", "git.foo", "my:thing", "a-b-c", "plain",
		"k----load",    // repeated separators
		"load-",        // trailing separator
		"lo--ad",       // adjacent separators mid-name
		"a.-.:b",       // mixed separators, adjacent
		"with/slash",   // a real bash/zsh function can be named with a slash
		"k+load",       // and '+'
		"k@load",       // and '@'
		"k%load",       // and '%'
		"k~load",       // and '~'
		"k#load",       // and '#' (mid-word — not a comment start, which requires word-start)
		"k!load",       // and '!'
		"función_ñame", // and any Unicode letter, not just ASCII
		"ünïcode_start",
	}
	for _, name := range names {
		s := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", ShellFunctions: map[string]string{name: body}}}}
		if err := Validate(s); err != nil {
			t.Errorf("function name %q rejected: %v", name, err)
		}
	}
}

// TestShellFunctionNamesStillRejectShellMetacharacters proves the permissive rule stayed a strict
// denylist of actual shell metacharacters, not merely "whatever bash -n accepts": the name is
// spliced unescaped into `name() { body }` in the real (non-`-n`) activation script, and bash -n
// reports "k;load" as syntactically fine too — but only because it parses as two statements (`k`,
// then a function actually named "load"), not because "k;load" is a valid single function name.
func TestShellFunctionNamesStillRejectShellMetacharacters(t *testing.T) {
	body := "echo ok"
	for _, name := range []string{
		"-load", "1load", // must start with a letter/underscore
		"with space", "k;load", "k&load", "k|load", "k(load)", "k<load", "k>load",
		"k$load", "k`load`", "k\"load", "k'load", "k\\load", "k=load", "k{load}", "k[load]", "k*load", "k?load",
	} {
		s := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", ShellFunctions: map[string]string{name: body}}}}
		if err := Validate(s); err == nil {
			t.Errorf("function name %q should have been rejected", name)
		}
	}
}

// TestVariableNamesStillRejectShellSafeSeparators proves the loosened function-name rule did not
// leak into env-var validation: a variable named "k-load" would be an invalid `export` target
// (POSIX variable names cannot contain hyphens), so it must still be rejected.
func TestVariableNamesStillRejectShellSafeSeparators(t *testing.T) {
	s := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", EnvVars: map[string]string{"k-load": "x"}}}}
	if err := Validate(s); err == nil {
		t.Fatal("hyphenated variable name should have been rejected")
	}
}

func TestFunctionDigestIsDeterministic(t *testing.T) {
	body := "echo ok"
	a := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", ShellFunctions: map[string]string{"f": body}}}}
	if FunctionDigest(a) != FunctionDigest(a) {
		t.Fatal("digest is not deterministic")
	}
}

func TestShellCmdValidatedLikeAFunctionAndCoveredByTrust(t *testing.T) {
	empty := ""
	invalid := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", ShellCmd: &empty}}}
	if err := Validate(invalid); err == nil {
		t.Fatal("empty shell-cmd body accepted")
	}

	body := "echo hook"
	without := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp"}}}
	with := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", ShellCmd: &body}}}
	if err := Validate(with); err != nil {
		t.Fatalf("valid shell-cmd rejected: %v", err)
	}
	if HasFunctions(without) {
		t.Fatal("HasFunctions true with no shell-cmd or shell-functions")
	}
	if !HasFunctions(with) {
		t.Fatal("HasFunctions false with a shell-cmd present")
	}
	if FunctionDigest(without) == FunctionDigest(with) {
		t.Fatal("digest unchanged after adding a shell-cmd")
	}

	sharedBody := "echo shared"
	shared := &Settings{Version: 1, Shared: Environment{ShellCmd: &sharedBody}, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp"}}}
	if !HasFunctions(shared) {
		t.Fatal("HasFunctions false with a shared shell-cmd present")
	}
}

func TestDefinitionLimits(t *testing.T) {
	vars := map[string]string{}
	for i := 0; i <= MaxDefinitions; i++ {
		vars[fmt.Sprintf("VAR_%03d", i)] = "x"
	}
	s := &Settings{Version: 1, Envs: map[string]ProjectEnvironment{"dev": {Project: "/tmp", EnvVars: vars}}}
	if err := Validate(s); err == nil {
		t.Fatal("definition limit not enforced")
	}
}
