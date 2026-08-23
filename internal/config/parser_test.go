package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseStrictYAML(t *testing.T) {
	valid := `version: 1
envs:
  dev:
    project: /tmp
    env-vars:
      EMPTY: ""
    shell-functions:
      hi: echo hi
`
	if _, err := Parse(strings.NewReader(valid)); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	cases := []string{
		"version: 1\nversion: 1\nenvs: {}\n",
		"version: 1\nenvs: {}\nunknown: true\n",
		"version: 1\nenvs: &x {}\ncopy: *x\n",
		"version: 1\nenvs: {}\n---\nversion: 1\n",
	}
	for _, input := range cases {
		if parsed, err := Parse(strings.NewReader(input)); err == nil || parsed != nil {
			t.Fatalf("invalid settings accepted: %q", input)
		} else if strings.HasPrefix(input, "version: 1\nversion") && !strings.Contains(err.Error(), "line") {
			t.Fatalf("duplicate-key error lacks source line: %v", err)
		}
	}
}

func TestParseResolvesAnchorsAndMergeKeys(t *testing.T) {
	valid := `version: 1
shared:
  env-vars: &shared_vars
    SHARED_ONLY: "s"
    AWS_REGION: "eu-west-1"
  shell-functions: &shared_funcs
    greet: echo hi
envs:
  dev:
    project: /tmp/dev
    env-vars:
      <<: *shared_vars
      DEV_ONLY: "d"
      AWS_REGION: "eu-central-1"
    shell-functions:
      <<: *shared_funcs
      dev_only: echo dev
`
	settings, err := Parse(strings.NewReader(valid))
	if err != nil {
		t.Fatalf("anchors/merge keys rejected: %v", err)
	}
	dev := settings.Envs["dev"]
	if dev.EnvVars["SHARED_ONLY"] != "s" || dev.EnvVars["DEV_ONLY"] != "d" {
		t.Fatalf("merged env-vars missing entries: %#v", dev.EnvVars)
	}
	if dev.EnvVars["AWS_REGION"] != "eu-central-1" {
		t.Fatalf("explicit key did not override merged value: %q", dev.EnvVars["AWS_REGION"])
	}
	if _, ok := dev.ShellFunctions["greet"]; !ok {
		t.Fatal("merged shell-function missing")
	}
	if _, ok := dev.ShellFunctions["dev_only"]; !ok {
		t.Fatal("project-specific shell-function missing alongside merge")
	}
}

func TestParseRejectsAnchorReferencingAnotherAnchor(t *testing.T) {
	nested := `version: 1
shared:
  env-vars: &inner
    A: "1"
envs:
  dev:
    project: /tmp
    env-vars: &outer
      <<: *inner
      B: "2"
`
	_, err := Parse(strings.NewReader(nested))
	if err == nil {
		t.Fatal("anchor referencing another anchor accepted")
	}
	if !strings.Contains(err.Error(), "cannot itself reference another anchor") {
		t.Fatalf("expected nested-anchor error, got: %v", err)
	}
}

func TestParseRejectsExcessiveAliasExpansion(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: 1\nshared:\n  env-vars: &big\n")
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&b, "    K%04d: %q\n", i, strings.Repeat("x", 400))
	}
	b.WriteString("envs:\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "  p%02d:\n    project: /tmp\n    env-vars:\n      <<: *big\n", i)
	}
	if _, err := Parse(strings.NewReader(b.String())); err == nil {
		t.Fatal("excessive alias expansion accepted")
	} else if !strings.Contains(err.Error(), "expands to more than") {
		t.Fatalf("expected expansion-budget error, got: %v", err)
	}
}

func TestParseRejectsOversizedAndImplicitScalars(t *testing.T) {
	if _, err := Parse(strings.NewReader(strings.Repeat("x", MaxSettingsSize+1))); err == nil {
		t.Fatal("oversized input accepted")
	}
	implicit := "version: 1\nenvs:\n  dev:\n    project: /tmp\n    env-vars:\n      VALUE: 123\n"
	if _, err := Parse(strings.NewReader(implicit)); err == nil {
		t.Fatal("implicit numeric scalar accepted")
	}
	nonScalarBody := "version: 1\nenvs:\n  dev:\n    project: /tmp\n    shell-functions:\n      f:\n        bash: echo no\n"
	if _, err := Parse(strings.NewReader(nonScalarBody)); err == nil {
		t.Fatal("non-scalar function body accepted")
	}
}

func TestLoadRejectsUnsafePermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nenvs:\n  dev:\n    project: /tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unsafe settings permissions accepted")
	}
}
