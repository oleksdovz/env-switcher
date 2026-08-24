package integration

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/environment"
	"github.com/dolf/env-switcher/internal/shell"
)

// TestCurrentShellActivation proves a switch defines functions, exports variables (values kept
// exactly literal even with quotes/`$`/newlines), and records the active project — and, per the
// simplified contract, leaves anything not managed by this switch (a pre-existing OBSOLETE
// variable/function) untouched rather than removing it.
func TestCurrentShellActivation(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		name := name
		t.Run(name, func(t *testing.T) {
			if _, err := exec.LookPath(name); err != nil {
				t.Skip(name + " unavailable")
			}
			effective := &environment.Effective{Project: "dev", Shell: name, Variables: []environment.Variable{{Name: "QUOTED", Value: "a '$HOME; value\nnext"}}, Functions: []environment.Function{{Name: "hello_env_switcher", Body: "printf '%s' ok"}}}
			payload, err := shell.Render(effective)
			if err != nil {
				t.Fatal(err)
			}
			script := `start=$PWD
payload=$(cat)
apply_payload() { builtin eval -- "$payload"; }
export OBSOLETE=old
obsolete_fn() { :; }
apply_payload || exit $?
test "$OBSOLETE" = old || exit 20
typeset -f obsolete_fn >/dev/null 2>&1 || exit 21
test "$QUOTED" = "a '\$HOME; value
next" || exit 22
test "$PWD" = "$start" || exit 23
test "$__ENV_SWITCHER_ACTIVE_PROJECT" = dev || exit 24
hello_env_switcher
`
			cmd := exec.Command(name)
			if name == "bash" {
				cmd.Args = []string{name, "--noprofile", "--norc", "-c", script}
			} else {
				cmd.Args = []string{name, "-f", "-c", script}
			}
			cmd.Stdin = strings.NewReader(payload)
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("activation failed: %v stderr=%s", err, stderr.String())
			}
			if out.String() != "ok" {
				t.Fatalf("unexpected output %q", out.String())
			}
		})
	}
}

func shellQuoteForTest(s string) string { q, _ := shell.Quote(s); return q }

// TestDynamicFunctionNamesActuallyCallable proves shell-legal-but-unusual function names — not
// just plain identifiers — round-trip through the real activation script: defined by shell.Render
// and then genuinely callable by that exact name in a real shell, not merely accepted by
// config.Validate. Covers what was reported rejected across three rounds of this same fix:
// repeated separators, a slash, and a non-ASCII letter.
func TestDynamicFunctionNamesActuallyCallable(t *testing.T) {
	names := []string{"k----load", "with/slash", "función_ñame"}
	for _, shellName := range []string{"bash", "zsh"} {
		shellName := shellName
		t.Run(shellName, func(t *testing.T) {
			if _, err := exec.LookPath(shellName); err != nil {
				t.Skip(shellName + " unavailable")
			}
			var funcs []environment.Function
			var calls strings.Builder
			for _, name := range names {
				funcs = append(funcs, environment.Function{Name: name, Body: "printf 'called:%s\\n' " + shellQuoteForTest(name)})
				calls.WriteString(shellQuoteForTest(name) + "\n") // invoke by the literal name, quoted the same way any word would be
			}
			effective := &environment.Effective{Project: "dev", Shell: shellName, Functions: funcs}
			payload, err := shell.Render(effective)
			if err != nil {
				t.Fatal(err)
			}
			script := `payload=$(cat)
apply_payload() { builtin eval -- "$payload"; }
apply_payload || exit $?
` + calls.String()
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
				t.Fatalf("activation/call failed: %v stderr=%s", err, stderr.String())
			}
			for _, name := range names {
				if !strings.Contains(out.String(), "called:"+name) {
					t.Errorf("function %q was not called correctly; output=%q", name, out.String())
				}
			}
		})
	}
}

// TestShellCmdsRunSharedThenProjectAfterCommit proves shell-cmd hooks execute in the documented
// order (shared, then project), after variables/functions are already applied, and that a
// failing hook does not prevent the rest of the script (including the other hook) from running.
func TestShellCmdsRunSharedThenProjectAfterCommit(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		name := name
		t.Run(name, func(t *testing.T) {
			if _, err := exec.LookPath(name); err != nil {
				t.Skip(name + " unavailable")
			}
			dir := t.TempDir()
			effective := &environment.Effective{
				Project:   "dev",
				Shell:     name,
				Variables: []environment.Variable{{Name: "HOOK_VAR", Value: "set"}},
				ShellCmds: []string{
					`echo shared >>"$HOOK_LOG"; false`,
					`echo project >>"$HOOK_LOG"; test "$HOOK_VAR" = set && echo saw-var >>"$HOOK_LOG"`,
				},
			}
			payload, err := shell.Render(effective)
			if err != nil {
				t.Fatal(err)
			}
			logPath := dir + "/hooks.log"
			script := `export HOOK_LOG=` + shellQuoteForTest(logPath) + `
payload=$(cat)
apply_payload() { builtin eval -- "$payload"; }
apply_payload
exit_code=$?
test "$exit_code" -eq 0 || exit 30
test "$HOOK_VAR" = set || exit 31
`
			cmd := exec.Command(name)
			if name == "bash" {
				cmd.Args = []string{name, "--noprofile", "--norc", "-c", script}
			} else {
				cmd.Args = []string{name, "-f", "-c", script}
			}
			cmd.Stdin = strings.NewReader(payload)
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("activation failed: %v stderr=%s", err, stderr.String())
			}
			log, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("hook log missing: %v", err)
			}
			if string(log) != "shared\nproject\nsaw-var\n" {
				t.Fatalf("unexpected hook order/content: %q", log)
			}
		})
	}
}

// TestExactlyOneHundredSwitches proves 100 rapid successive switches all apply cleanly. Per the
// simplified contract there is no cleanup between switches, so both projects' variables and
// functions accumulate rather than the earlier one being removed — that's asserted explicitly.
func TestExactlyOneHundredSwitches(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		name := name
		t.Run(name, func(t *testing.T) {
			if _, err := exec.LookPath(name); err != nil {
				t.Skip(name + " unavailable")
			}
			var stream strings.Builder
			for i := 1; i <= 100; i++ {
				e := &environment.Effective{Project: "dev", Shell: name}
				if i%2 == 1 {
					e.Variables = []environment.Variable{{Name: "DEV_ONLY", Value: "dev"}}
					e.Functions = []environment.Function{{Name: "dev_fn", Body: ":"}}
				} else {
					e.Variables = []environment.Variable{{Name: "STAGE_ONLY", Value: "staging"}}
					e.Functions = []environment.Function{{Name: "stage_fn", Body: ":"}}
				}
				payload, err := shell.Render(e)
				if err != nil {
					t.Fatal(err)
				}
				stream.WriteString(payload)
				stream.WriteString("__ENV_SWITCHER_TEST_DELIMITER__\n")
			}
			script := `UNRELATED=keep
payload=
while IFS= read -r line; do
  if [ "$line" = __ENV_SWITCHER_TEST_DELIMITER__ ]; then
    apply_payload() { builtin eval -- "$payload"; }
    apply_payload || exit $?
    payload=
  else
    payload="${payload}${line}
"
  fi
done
test "$DEV_ONLY" = dev || exit 30
test "$STAGE_ONLY" = staging || exit 31
typeset -f dev_fn >/dev/null 2>&1 || exit 32
typeset -f stage_fn >/dev/null 2>&1 || exit 33
test "$UNRELATED" = keep || exit 34
`
			args := []string{"-f", "-c", script}
			if name == "bash" {
				args = []string{"--noprofile", "--norc", "-c", script}
			}
			cmd := exec.Command(name, args...)
			cmd.Stdin = strings.NewReader(stream.String())
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("100-switch test failed: %v %s", err, out)
			}
		})
	}
}

// TestReadonlyConflictIsPartialNotRolledBack documents the current, deliberately simplified
// contract: Render produces a plain sequential script with no snapshot/rollback. A readonly
// conflict fails only that one statement (the shell reports its own error); every other
// statement — including ones alphabetically after the failing name — still applies.
func TestReadonlyConflictIsPartialNotRolledBack(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		name := name
		t.Run(name, func(t *testing.T) {
			if _, err := exec.LookPath(name); err != nil {
				t.Skip(name + " unavailable")
			}
			e := &environment.Effective{Project: "dev", Shell: name, Variables: []environment.Variable{{Name: "A_CHANGED", Value: "new"}, {Name: "Z_LOCKED", Value: "new"}}}
			payload, err := shell.Render(e)
			if err != nil {
				t.Fatal(err)
			}
			script := `A_CHANGED=old
Z_LOCKED=old
readonly Z_LOCKED
payload=$(cat)
apply_payload() { builtin eval -- "$payload"; }
apply_payload >/dev/null 2>&1
test "$A_CHANGED" = new || exit 42
test "$Z_LOCKED" = old || exit 43
`
			args := []string{"-f", "-c", script}
			if name == "bash" {
				args = []string{"--noprofile", "--norc", "-c", script}
			}
			cmd := exec.Command(name, args...)
			cmd.Stdin = strings.NewReader(payload)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("rollback test failed: %v %s", err, out)
			}
		})
	}
}
