package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupDigestAndTargetScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	profile := filepath.Join(home, ".bashrc")
	id, err := backup(profile, []byte("original\n"), 0o640)
	if err != nil || id == "" {
		t.Fatal(err)
	}
	meta, err := latestBackup(profile)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(meta.Data); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup permissions invalid: %v", err)
	}
	if err := os.WriteFile(meta.Data, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restoreLatest(profile); err == nil {
		t.Fatal("tampered backup restored")
	}
	if _, err := latestBackup(filepath.Join(home, ".zshrc")); err == nil {
		t.Fatal("backup accepted for wrong target")
	}
}
