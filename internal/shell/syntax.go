package shell

import (
	"bytes"
	"fmt"
	"os/exec"
)

func ValidateFunction(shell, name, body string) error {
	if shell != "bash" && shell != "zsh" {
		return fmt.Errorf("unsupported shell")
	}
	cmd := exec.Command(shell, "-n")
	cmd.Stdin = bytes.NewBufferString(name + "() {\n" + body + "\n}\n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("function %s has invalid %s syntax", name, shell)
	}
	return nil
}
