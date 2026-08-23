package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Resolve picks the editor command: $VISUAL, then $EDITOR, then vim as a last-resort default
// (matching the long-standing convention other CLI tools, e.g. git, already use) so that F3/edit
// works out of the box on a machine where neither is set.
func Resolve() ([]string, error) {
	value := strings.TrimSpace(os.Getenv("VISUAL"))
	if value == "" {
		value = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if value == "" {
		if _, err := exec.LookPath("vim"); err == nil {
			value = "vim"
		}
	}
	if value == "" {
		return nil, fmt.Errorf("no editor configured; set VISUAL or EDITOR")
	}
	args, err := split(value)
	if err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid editor command")
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		return nil, fmt.Errorf("editor executable %q was not found", args[0])
	}
	return args, nil
}

func Open(path string) error {
	cmd, err := Command(path)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}
	return nil
}

func Command(path string) (*exec.Cmd, error) {
	args, err := Resolve()
	if err != nil {
		return nil, err
	}
	return exec.Command(args[0], append(args[1:], path)...), nil
}

func split(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("unclosed quote or escape")
	}
	flush()
	return out, nil
}
