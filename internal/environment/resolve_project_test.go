package environment

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
)

func TestResolveExpandsHomeInProjectPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{
		"dollar":       {Project: "$HOME/projects/my"},
		"dollar-brace": {Project: "${HOME}/projects/my"},
		"tilde":        {Project: "~/projects/my"},
	}}
	want := filepath.Join(home, "projects/my")
	for _, name := range []string{"dollar", "dollar-brace", "tilde"} {
		e, err := Resolve(s, name, "bash")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := findVar(t, e, "_PROJECT")
		if got != want {
			t.Errorf("%s: _PROJECT = %q, want %q", name, got, want)
		}
	}
}

func TestResolveProjectPathWithSpacesAndMetacharacters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{
		"dev": {Project: "$HOME/projects/env switcher & friends$(x)/dev"},
	}}
	e, err := Resolve(s, "dev", "bash")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "projects/env switcher & friends$(x)/dev")
	if got := findVar(t, e, "_PROJECT"); got != want {
		t.Fatalf("_PROJECT = %q, want %q", got, want)
	}
}

func TestResolveRejectsEmptyProject(t *testing.T) {
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "   "}}}
	if _, err := Resolve(s, "dev", "bash"); err == nil {
		t.Fatal("expected an error for a blank project path")
	}
}

func TestResolveRejectsUnresolvedRelativeProject(t *testing.T) {
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "projects/my"}}}
	_, err := Resolve(s, "dev", "bash")
	if err == nil {
		t.Fatal("expected an error for a project path that never resolves to an absolute path")
	}
	if !strings.Contains(err.Error(), "envs.dev.project") {
		t.Fatalf("error not scoped to envs.dev.project: %v", err)
	}
}

func TestResolveRejectsProjectThatIsOnlyAnUnsupportedReference(t *testing.T) {
	// $UNSUPPORTED is not $HOME, so it's left completely literal — and a literal "$UNSUPPORTED"
	// is not an absolute path.
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "$UNSUPPORTED/my"}}}
	if _, err := Resolve(s, "dev", "bash"); err == nil {
		t.Fatal("expected an error for an unresolved project reference")
	}
}

func TestResolveSubstitutesProjectVarIntoOtherEnvVars(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := &config.Settings{Version: 1,
		Shared: config.Environment{EnvVars: map[string]string{"CODEX_HOME": "$_PROJECT/.codex"}},
		Envs: map[string]config.ProjectEnvironment{"dev": {
			Project: "$HOME/projects/my",
			EnvVars: map[string]string{"CODEX_SQLITE_HOME": "${_PROJECT}/.codex/sqlite"},
		}},
	}
	e, err := Resolve(s, "dev", "bash")
	if err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(home, "projects/my")
	if got := findVar(t, e, "CODEX_HOME"); got != proj+"/.codex" {
		t.Fatalf("CODEX_HOME = %q", got)
	}
	if got := findVar(t, e, "CODEX_SQLITE_HOME"); got != proj+"/.codex/sqlite" {
		t.Fatalf("CODEX_SQLITE_HOME = %q", got)
	}
}

func TestResolveProjectVarWinsEvenWithoutExplicitDeclaration(t *testing.T) {
	// config.Validate independently rejects a manually declared _PROJECT (see
	// internal/config's TestRejectsManuallyDeclaredProjectVarIn*Scope) — this proves Resolve
	// itself always produces the environment's own value regardless.
	dir := t.TempDir()
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: dir}}}
	e, err := Resolve(s, "dev", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if got := findVar(t, e, "_PROJECT"); got != dir {
		t.Fatalf("_PROJECT = %q, want %q", got, dir)
	}
}

func TestResolveDifferentProjectsProduceDifferentProjectVar(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{
		"a": {Project: dirA},
		"b": {Project: dirB},
	}}
	eA, err := Resolve(s, "a", "bash")
	if err != nil {
		t.Fatal(err)
	}
	eB, err := Resolve(s, "b", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if got := findVar(t, eA, "_PROJECT"); got != dirA {
		t.Fatalf("a: _PROJECT = %q, want %q", got, dirA)
	}
	if got := findVar(t, eB, "_PROJECT"); got != dirB {
		t.Fatalf("b: _PROJECT = %q, want %q", got, dirB)
	}
}

// TestResolveOverwritesManuallyDeclaredProjectVar proves a manually declared _PROJECT — accepted
// by config.Validate for backward compatibility with settings written before this variable
// existed — is always overwritten by the computed value, in both shared and project scope, never
// left as the stale declared value and never an error.
func TestResolveOverwritesManuallyDeclaredProjectVar(t *testing.T) {
	dir := t.TempDir()
	s := &config.Settings{Version: 1,
		Shared: config.Environment{EnvVars: map[string]string{"_PROJECT": "/stale/shared"}},
		Envs: map[string]config.ProjectEnvironment{"dev": {
			Project: dir,
			EnvVars: map[string]string{"_PROJECT": "/stale/project"},
		}},
	}
	e, err := Resolve(s, "dev", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if got := findVar(t, e, "_PROJECT"); got != dir {
		t.Fatalf("_PROJECT = %q, want the computed %q (declared values must be overwritten)", got, dir)
	}
}

func findVar(t *testing.T, e *Effective, name string) string {
	t.Helper()
	for _, v := range e.Variables {
		if v.Name == name {
			return v.Value
		}
	}
	t.Fatalf("variable %s not found in %#v", name, e.Variables)
	return ""
}
