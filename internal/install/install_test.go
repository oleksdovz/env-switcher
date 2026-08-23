package install

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallRollbackAndUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	source := filepath.Join(home, "source")
	if err := os.WriteFile(source, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(home, ".bashrc")
	original := []byte("export KEEP=yes\n")
	if err := os.WriteFile(profile, original, 0o640); err != nil {
		t.Fatal(err)
	}
	target, err := Resolve("bash", profile)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Install(target, source)
	if err != nil || !first.Changed || first.Backup == "" {
		t.Fatalf("first install: %#v %v", first, err)
	}
	installed, _ := os.ReadFile(profile)
	if bytes.Count(installed, []byte(Begin)) != 1 || !bytes.Contains(installed, original) {
		t.Fatal("managed block or unrelated bytes invalid")
	}
	second, err := Install(target, source)
	if err != nil || second.Changed {
		t.Fatalf("second install not idempotent: %#v %v", second, err)
	}
	if err := Rollback(target); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(profile)
	if !bytes.Equal(restored, original) {
		t.Fatalf("rollback mismatch: %q", restored)
	}
	if _, err := Install(target, source); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(target); err != nil {
		t.Fatal(err)
	}
	clean, _ := os.ReadFile(profile)
	if !bytes.Equal(clean, original) {
		t.Fatalf("uninstall changed unrelated bytes: %q", clean)
	}
}

func TestResolveRejectsUnsupportedShellAndSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := Resolve("fish", ""); err == nil {
		t.Fatal("unsupported shell accepted")
	}
	target := filepath.Join(home, "real")
	_ = os.WriteFile(target, nil, 0o600)
	link := filepath.Join(home, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("bash", link); err == nil {
		t.Fatal("symlink accepted")
	}
}
