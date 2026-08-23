package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFunctionBodyUsesStdinNotProcessArguments(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	stdinPath := filepath.Join(dir, "stdin")
	fake := filepath.Join(dir, "bash")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + argsPath + "\ncat > " + stdinPath + "\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	canary := "SECRET_FUNCTION_CANARY"
	if err := ValidateFunction("bash", "example", "echo "+canary); err != nil {
		t.Fatal(err)
	}
	args, _ := os.ReadFile(argsPath)
	stdin, _ := os.ReadFile(stdinPath)
	if strings.Contains(string(args), canary) {
		t.Fatal("function body leaked into process arguments")
	}
	if !strings.Contains(string(stdin), canary) {
		t.Fatal("syntax checker did not receive function body on stdin")
	}
}
