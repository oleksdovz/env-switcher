package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestViewRequiresWarningConfirmation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".env-switcher")
	_ = os.Mkdir(dir, 0o700)
	content := []byte("SECRET_CANARY: visible-only-after-confirmation\n")
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, warn bytes.Buffer
	if err := viewCommand(bytes.NewBufferString("n\n"), &out, &warn); err == nil || out.Len() != 0 {
		t.Fatal("cancelled view disclosed content")
	}
	out.Reset()
	warn.Reset()
	if err := viewCommand(bytes.NewBufferString("y\n"), &out, &warn); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), content) || warn.Len() == 0 {
		t.Fatal("confirmed view did not return full content and warning")
	}
}
