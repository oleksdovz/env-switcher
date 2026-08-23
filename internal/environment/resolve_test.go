package environment

import (
	"testing"

	"github.com/dolf/env-switcher/internal/config"
)

func TestResolveOverridesSharedByExactName(t *testing.T) {
	dir := t.TempDir()
	s := &config.Settings{Version: 1, Shared: config.Environment{EnvVars: map[string]string{"A": "shared", "OLD": "old"}}, Envs: map[string]config.ProjectEnvironment{"dev": {Project: dir, EnvVars: map[string]string{"A": "project", "B": "b"}}}}
	e, err := Resolve(s, "dev", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Variables) != 3 || e.Variables[0].Name != "A" || e.Variables[0].Value != "project" {
		t.Fatalf("unexpected variables: %#v", e.Variables)
	}
}

func TestResolveOrdersShellCmdsSharedThenProject(t *testing.T) {
	dir := t.TempDir()
	shared := "echo shared-hook"
	project := "echo project-hook"
	s := &config.Settings{
		Version: 1,
		Shared:  config.Environment{ShellCmd: &shared},
		Envs:    map[string]config.ProjectEnvironment{"dev": {Project: dir, ShellCmd: &project}},
	}
	e, err := Resolve(s, "dev", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if len(e.ShellCmds) != 2 || e.ShellCmds[0] != shared || e.ShellCmds[1] != project {
		t.Fatalf("expected [shared, project] order, got %#v", e.ShellCmds)
	}
}

func TestResolveDoesNotRequireProjectDirectoryToExist(t *testing.T) {
	// `project` in settings.yaml is informational only — Resolve never changes or checks the
	// current directory, so a nonexistent path must not fail the switch.
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: "/no/such/path", EnvVars: map[string]string{"A": "b"}}}}
	e, err := Resolve(s, "dev", "bash")
	if err != nil {
		t.Fatalf("Resolve failed for a nonexistent project directory: %v", err)
	}
	if len(e.Variables) != 1 {
		t.Fatalf("unexpected variables: %#v", e.Variables)
	}
}

func TestResolveOmitsAbsentShellCmds(t *testing.T) {
	dir := t.TempDir()
	s := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{"dev": {Project: dir}}}
	e, err := Resolve(s, "dev", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if len(e.ShellCmds) != 0 {
		t.Fatalf("expected no shell-cmds, got %#v", e.ShellCmds)
	}
}
