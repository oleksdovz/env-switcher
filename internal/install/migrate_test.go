package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyExecutableRemovesOnceCanonicalExists(t *testing.T) {
	dir := t.TempDir()
	legacy := LegacyExecutablePath(dir)
	if err := os.WriteFile(legacy, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "env-switcher"), []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}
	removed, err := MigrateLegacyExecutable(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("expected the legacy executable to be removed")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy executable still present: %v", err)
	}
}

func TestMigrateLegacyExecutableNoopWhenNothingLegacyExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "env-switcher"), []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}
	removed, err := MigrateLegacyExecutable(dir)
	if err != nil || removed {
		t.Fatalf("expected a no-op, got removed=%v err=%v", removed, err)
	}
}

func TestMigrateLegacyExecutableNeverRemovesWithoutCanonicalCopy(t *testing.T) {
	dir := t.TempDir()
	legacy := LegacyExecutablePath(dir)
	if err := os.WriteFile(legacy, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	// No bin/env-switcher yet: must not strand the user with no executable at all.
	removed, err := MigrateLegacyExecutable(dir)
	if err != nil || removed {
		t.Fatalf("expected a no-op without a canonical copy, got removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy executable was removed prematurely: %v", err)
	}
}

func TestMigrateLegacyExecutableLeavesSymlinkAlone(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere")
	if err := os.WriteFile(target, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := LegacyExecutablePath(dir)
	if err := os.Symlink(target, legacy); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "env-switcher"), []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}
	removed, err := MigrateLegacyExecutable(dir)
	if err != nil || removed {
		t.Fatalf("expected a symlink to be left alone, got removed=%v err=%v", removed, err)
	}
	if _, err := os.Lstat(legacy); err != nil {
		t.Fatalf("symlink was removed: %v", err)
	}
}
