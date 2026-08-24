package fsatomic

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileReplacesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(b) != "new" || info.Mode().Perm() != 0o640 {
		t.Fatal("atomic write content or mode mismatch")
	}
}

func TestInterruptedWriteLeavesPriorFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile")
	if err := os.WriteFile(path, []byte("prior"), 0o600); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("simulated interruption")
	if err := writeFile(path, []byte("partial"), 0o600, func() error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("unexpected error %v", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "prior" {
		t.Fatalf("interrupted write changed prior file: %q", b)
	}
}

func TestPublishReplacesContentAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env-switcher")
	if err := os.WriteFile(path, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(dir, "env-switcher.tmp")
	if err := os.WriteFile(tmp, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Publish(tmp, path, 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new" || info.Mode().Perm() != 0o700 {
		t.Fatalf("publish content/mode mismatch: %q %v", b, info.Mode().Perm())
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("temp file was not consumed: stat err=%v", err)
	}
}

func TestPublishRejectsDifferentDirectories(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	tmp := filepath.Join(other, "env-switcher.tmp")
	if err := os.WriteFile(tmp, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "env-switcher")
	if err := Publish(tmp, path, 0o700); err == nil {
		t.Fatal("expected an error for a cross-directory publish")
	}
}

func TestReadOnlyParentFailsWithoutChangingFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "profile")
	if err := os.WriteFile(path, []byte("prior"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	if err := WriteFile(path, []byte("new"), 0o600); err == nil {
		t.Fatal("write in read-only parent succeeded")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "prior" {
		t.Fatal("read-only failure changed file")
	}
}
