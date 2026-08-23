package shell

import (
	"fmt"
	"strings"

	"github.com/dolf/env-switcher/internal/environment"
)

// Render generates the plain shell statements that apply e in the current shell: define
// functions, export variables, record the active project, then run any shell-cmd hooks. It never
// changes directory — `project` in settings.yaml is informational only.
//
// This is a straight-line sequence, not a transaction: there is no readonly pre-check and no
// snapshot/rollback. If a name happens to be readonly in the current shell, that one export or
// function definition fails (the shell prints its own error) and the rest of the file still runs
// — a partial application is possible. It also does not track or remove names a previously
// active project set: each switch only ever applies the selected project's own
// variables/functions, on top of whatever the shell already has. The caller is responsible for
// delivering this text to the shell (writing it to the current-env file for `source`, or
// otherwise); Render itself only produces the statements.
func Render(e *environment.Effective) (string, error) {
	var body strings.Builder
	qproject, _ := Quote(e.Project)
	for _, fn := range e.Functions {
		body.WriteString(fn.Name + "() {\n" + fn.Body + "\n}\n")
	}
	for _, v := range e.Variables {
		q, err := Quote(v.Value)
		if err != nil {
			return "", fmt.Errorf("variable %s cannot be quoted", v.Name)
		}
		body.WriteString("export " + v.Name + "=" + q + "\n")
	}
	body.WriteString("export __ENV_SWITCHER_ACTIVE_PROJECT=" + qproject + "\n")
	for _, cmd := range e.ShellCmds {
		body.WriteString(cmd)
		if !strings.HasSuffix(cmd, "\n") {
			body.WriteString("\n")
		}
	}
	return body.String(), nil
}
