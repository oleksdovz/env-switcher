package config

import "testing"

// TestManuallyDeclaredProjectVarIsAcceptedByValidation proves settings written before _PROJECT
// existed (or that simply still declare it, e.g. as a leftover manual workaround) keep loading:
// declaring it is accepted, not an error — internal/environment.Resolve is what actually always
// overwrites it with the computed value, never validation.
func TestManuallyDeclaredProjectVarIsAcceptedByValidation(t *testing.T) {
	shared := &Settings{Version: 1,
		Shared: Environment{EnvVars: map[string]string{"_PROJECT": "/tmp/stale"}},
		Envs:   map[string]ProjectEnvironment{"dev": {Project: "/tmp"}},
	}
	if err := Validate(shared); err != nil {
		t.Fatalf("shared-scope _PROJECT should be accepted, got: %v", err)
	}

	perEnv := &Settings{Version: 1,
		Envs: map[string]ProjectEnvironment{"example": {Project: "/tmp", EnvVars: map[string]string{"_PROJECT": "/tmp/stale"}}},
	}
	if err := Validate(perEnv); err != nil {
		t.Fatalf("env-scoped _PROJECT should be accepted, got: %v", err)
	}
}

func TestAcceptsSettingsWithoutManuallyDeclaredProjectVar(t *testing.T) {
	s := &Settings{Version: 1,
		Shared: Environment{EnvVars: map[string]string{"LOG_LEVEL": "info"}},
		Envs:   map[string]ProjectEnvironment{"dev": {Project: "/tmp", EnvVars: map[string]string{"CODEX_HOME": "$_PROJECT/.codex"}}},
	}
	if err := Validate(s); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
