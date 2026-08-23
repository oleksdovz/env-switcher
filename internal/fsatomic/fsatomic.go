// Package fsatomic provides a shared same-directory temp-file-then-rename write used by
// every file this project must never leave half-written: settings backups, the installed
// executable, the managed profile, and the current-env activation file.
package fsatomic

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile writes data to path via a same-directory temporary file, fsyncs it, sets mode,
// and atomically renames it into place. On success it also fsyncs the containing directory
// so the rename itself is durable. The target is left untouched if any step fails.
func WriteFile(path string, data []byte, mode os.FileMode) error {
	return writeFile(path, data, mode, nil)
}

// writeFile is WriteFile with an injectable hook run just before the rename, so tests can
// simulate an interruption between the durable temp-file write and the rename that
// publishes it.
func writeFile(path string, data []byte, mode os.FileMode, beforeRename func() error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".env-switcher-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if beforeRename != nil {
		if err := beforeRename(); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("atomic replace: %w", err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
