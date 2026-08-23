package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSplit(t *testing.T) {
	got, err := split(`code --wait "a b"`)
	if err != nil || len(got) != 3 || got[2] != "a b" {
		t.Fatalf("%#v %v", got, err)
	}
}

func TestResolvePrecedenceAndMissingEditor(t *testing.T) {
	dir := t.TempDir()
	visual := filepath.Join(dir, "visual")
	fallback := filepath.Join(dir, "editor")
	for _, p := range []string{visual, fallback} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("VISUAL", visual+" --wait")
	t.Setenv("EDITOR", fallback)
	got, err := Resolve()
	if err != nil || got[0] != visual || got[1] != "--wait" {
		t.Fatalf("unexpected resolution: %#v %v", got, err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	got, err = Resolve()
	if _, lookErr := exec.LookPath("vim"); lookErr == nil {
		if err != nil || len(got) == 0 || got[0] != "vim" {
			t.Fatalf("expected vim fallback when unset, got %#v %v", got, err)
		}
	} else if err == nil {
		t.Fatal("missing editor accepted with no vim fallback available")
	}
}

func TestOpenReportsFailedEditor(t *testing.T) {
	t.Setenv("VISUAL", "/bin/sh -c 'exit 9'")
	t.Setenv("EDITOR", "")
	if err := Open("/tmp/settings.yaml"); err == nil {
		t.Fatal("failed editor reported success")
	}
}
