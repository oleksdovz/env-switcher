package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/environment"
	"github.com/dolf/env-switcher/internal/shell"
)

// TestProjectVarAvailableToSharedAndProjectFunctionsAndShellCmd proves _PROJECT is a real,
// exported shell variable by the time shared/project shell-cmd hooks run and by the time
// shared/project shell functions are later called — going through the actual resolution pipeline
// (config.Load -> environment.Resolve -> shell.Render), not a hand-built Effective.
func TestProjectVarAvailableToSharedAndProjectFunctionsAndShellCmd(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		name := name
		t.Run(name, func(t *testing.T) { testProjectVarAvailable(t, name) })
	}
}

func testProjectVarAvailable(t *testing.T, shellName string) {
	if _, err := exec.LookPath(shellName); err != nil {
		t.Skip(shellName + " unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	settings := "version: 1\n" +
		"shared:\n" +
		"  shell-functions:\n" +
		"    shared_fn: |\n" +
		"      printf 'shared:%s\\n' \"$_PROJECT\"\n" +
		"  shell-cmd: |\n" +
		"    printf 'shared-cmd:%s\\n' \"$_PROJECT\" >>\"$PROBE_LOG\"\n" +
		"envs:\n" +
		"  dev:\n" +
		"    project: $HOME/projects/my\n" +
		"    shell-functions:\n" +
		"      project_fn: |\n" +
		"        printf 'project:%s\\n' \"$_PROJECT\"\n" +
		"    shell-cmd: |\n" +
		"      printf 'project-cmd:%s\\n' \"$_PROJECT\" >>\"$PROBE_LOG\"\n"
	settingsPath := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(settingsPath, []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := config.Load(settingsPath)
	if err != nil {
		t.Fatalf("settings do not validate: %v", err)
	}
	if err := config.Acknowledge(config.FunctionDigest(set)); err != nil {
		t.Fatal(err)
	}
	effective, err := environment.Resolve(set, "dev", shellName)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := shell.Render(effective)
	if err != nil {
		t.Fatal(err)
	}

	wantProject := filepath.Join(home, "projects/my")
	logPath := filepath.Join(home, "probe.log")
	script := `export PROBE_LOG=` + shellQuoteForTest(logPath) + `
payload=$(cat)
apply_payload() { builtin eval -- "$payload"; }
apply_payload || exit $?
shared_out=$(shared_fn)
project_out=$(project_fn)
test "$shared_out" = "shared:` + wantProject + `" || exit 70
test "$project_out" = "project:` + wantProject + `" || exit 71
`
	cmd := exec.Command(shellName)
	if shellName == "bash" {
		cmd.Args = []string{shellName, "--noprofile", "--norc", "-c", script}
	} else {
		cmd.Args = []string{shellName, "-f", "-c", script}
	}
	cmd.Stdin = strings.NewReader(payload)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("activation/function call failed: %v stderr=%s", err, stderr.String())
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("shell-cmd probe log missing: %v", err)
	}
	want := "shared-cmd:" + wantProject + "\nproject-cmd:" + wantProject + "\n"
	if string(log) != want {
		t.Fatalf("shell-cmd hooks did not see _PROJECT correctly: got %q want %q", log, want)
	}
}

// TestProjectVarQuotedSafelyWithSpacesAndMetacharacters proves a project path containing spaces
// and shell metacharacters ends up in the live shell as the exact literal string, never
// word-split, glob-expanded, or executed as a command.
func TestProjectVarQuotedSafelyWithSpacesAndMetacharacters(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		name := name
		t.Run(name, func(t *testing.T) { testProjectVarQuoting(t, name) })
	}
}

func testProjectVarQuoting(t *testing.T, shellName string) {
	if _, err := exec.LookPath(shellName); err != nil {
		t.Skip(shellName + " unavailable")
	}
	home := t.TempDir()
	dangerDir := filepath.Join(home, `projects & 'friends' $(touch `+home+`/pwned) *`)
	settings := &config.Settings{Version: 1, Envs: map[string]config.ProjectEnvironment{
		"dev": {Project: dangerDir},
	}}
	effective, err := environment.Resolve(settings, "dev", shellName)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := shell.Render(effective)
	if err != nil {
		t.Fatal(err)
	}
	script := `payload=$(cat)
apply_payload() { builtin eval -- "$payload"; }
apply_payload || exit $?
printf '%s' "$_PROJECT"
`
	cmd := exec.Command(shellName)
	if shellName == "bash" {
		cmd.Args = []string{shellName, "--noprofile", "--norc", "-c", script}
	} else {
		cmd.Args = []string{shellName, "-f", "-c", script}
	}
	cmd.Stdin = strings.NewReader(payload)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("activation failed: %v stderr=%s", err, stderr.String())
	}
	if out.String() != dangerDir {
		t.Fatalf("_PROJECT = %q, want %q", out.String(), dangerDir)
	}
	if _, err := os.Stat(filepath.Join(home, "pwned")); err == nil {
		t.Fatal("embedded command substitution in the project path executed")
	}
}
