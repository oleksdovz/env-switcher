package upgrade

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// ExtractExecutable extracts exactly one entry — named expectedName — from the zip archive at
// archivePath into destPath (created if absent, truncated if present — the caller is expected to
// have reserved a unique path, e.g. via os.CreateTemp). Anything else in the archive is treated
// as a reason to refuse the whole archive rather than something to skip: an absolute path, a
// "../" component, a symlink, a directory, or any entry whose name doesn't match expectedName.
func ExtractExecutable(archivePath, destPath, expectedName string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer r.Close()

	var target *zip.File
	for _, f := range r.File {
		if err := checkArchiveEntry(f, expectedName); err != nil {
			return err
		}
		target = f
	}
	if target == nil {
		return fmt.Errorf("archive does not contain expected entry %q", expectedName)
	}

	rc, err := target.Open()
	if err != nil {
		return fmt.Errorf("open archive entry %q: %w", target.Name, err)
	}
	defer rc.Close()

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	defer out.Close()

	n, err := io.Copy(out, io.LimitReader(rc, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("extract %q: %w", target.Name, err)
	}
	if n > maxResponseBytes {
		return fmt.Errorf("extracted entry %q exceeded %d byte limit", target.Name, int64(maxResponseBytes))
	}
	return nil
}

// checkArchiveEntry validates a single zip entry against the "only one known file, nothing else"
// contract. f is otherwise unused (not extracted) unless it's the sole match.
func checkArchiveEntry(f *zip.File, expectedName string) error {
	name := strings.ReplaceAll(f.Name, "\\", "/")
	cleaned := path.Clean(name)
	if path.IsAbs(name) || path.IsAbs(cleaned) {
		return fmt.Errorf("archive entry %q has an absolute path", f.Name)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("archive entry %q escapes the archive root", f.Name)
	}
	if f.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("archive entry %q is a symlink, refusing to extract", f.Name)
	}
	if f.FileInfo().IsDir() {
		return fmt.Errorf("unexpected directory entry %q in archive (expected only %q)", f.Name, expectedName)
	}
	if cleaned != expectedName {
		return fmt.Errorf("unexpected archive entry %q (expected only %q)", f.Name, expectedName)
	}
	return nil
}
