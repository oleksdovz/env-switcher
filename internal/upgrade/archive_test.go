package upgrade

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// buildZip writes a zip archive to path whose entries are exactly what add puts into it,
// letting each test construct the archive shape it needs to exercise.
func buildZip(t *testing.T, path string, add func(w *zip.Writer)) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	add(w)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func addFile(t *testing.T, w *zip.Writer, name string, mode os.FileMode, content string) {
	t.Helper()
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
	hdr.SetMode(mode)
	fw, err := w.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
}

func TestExtractExecutableHappyPath(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.zip")
	buildZip(t, archivePath, func(w *zip.Writer) {
		addFile(t, w, "env-switcher", 0o755, "binary-content")
	})
	destPath := filepath.Join(dir, "out")
	if err := ExtractExecutable(archivePath, destPath, "env-switcher"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary-content" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractExecutableRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.zip")
	buildZip(t, archivePath, func(w *zip.Writer) {
		addFile(t, w, "../../etc/passwd", 0o644, "nope")
	})
	if err := ExtractExecutable(archivePath, filepath.Join(dir, "out"), "env-switcher"); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}

func TestExtractExecutableRejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.zip")
	buildZip(t, archivePath, func(w *zip.Writer) {
		addFile(t, w, "/etc/passwd", 0o644, "nope")
	})
	if err := ExtractExecutable(archivePath, filepath.Join(dir, "out"), "env-switcher"); err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
}

func TestExtractExecutableRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.zip")
	buildZip(t, archivePath, func(w *zip.Writer) {
		addFile(t, w, "env-switcher", os.ModeSymlink|0o777, "/etc/passwd")
	})
	if err := ExtractExecutable(archivePath, filepath.Join(dir, "out"), "env-switcher"); err == nil {
		t.Fatal("expected symlink entry to be rejected")
	}
}

func TestExtractExecutableRejectsUnexpectedEntry(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.zip")
	buildZip(t, archivePath, func(w *zip.Writer) {
		addFile(t, w, "env-switcher", 0o755, "binary-content")
		addFile(t, w, "extra-payload.sh", 0o755, "echo surprise")
	})
	if err := ExtractExecutable(archivePath, filepath.Join(dir, "out"), "env-switcher"); err == nil {
		t.Fatal("expected the extra archive entry to be rejected")
	}
}

func TestExtractExecutableRejectsDirectoryEntry(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.zip")
	buildZip(t, archivePath, func(w *zip.Writer) {
		if _, err := w.Create("subdir/"); err != nil {
			t.Fatal(err)
		}
	})
	if err := ExtractExecutable(archivePath, filepath.Join(dir, "out"), "env-switcher"); err == nil {
		t.Fatal("expected a directory entry to be rejected")
	}
}

func TestExtractExecutableRejectsMissingEntry(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.zip")
	buildZip(t, archivePath, func(w *zip.Writer) {
		addFile(t, w, "something-else", 0o755, "x")
	})
	if err := ExtractExecutable(archivePath, filepath.Join(dir, "out"), "env-switcher"); err == nil {
		t.Fatal("expected missing entry to be rejected")
	}
}
