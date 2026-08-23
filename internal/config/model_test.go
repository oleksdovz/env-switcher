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
