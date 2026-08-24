package install

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// LegacyExecutablePath is where env-switcher installed its executable before the bin/
// subdirectory convention (config.ExecutablePath) existed.
func LegacyExecutablePath(dataDir string) string {
	return filepath.Join(dataDir, "env-switcher")
}

// MigrateLegacyExecutable removes a leftover executable at the pre-bin/ install location, once
// the canonical bin/env-switcher copy is confirmed to be in place — the managed wrapper block
// only ever points at bin/env-switcher (see internal/install/templates), so once that's present
// the legacy file is unreferenced litter, not a fallback anything still depends on.
//
// It is best-effort and conservative: a legacy path that doesn't exist, isn't a plain regular
// file, or isn't owned by the current user is left alone (returned as "not removed", with an
// error only for the ownership case — everything else is a silent no-op, since "no legacy
// install to migrate" is the common case for anyone who never ran a pre-bin/ version).
func MigrateLegacyExecutable(dataDir string) (removed bool, err error) {
	legacy := LegacyExecutablePath(dataDir)
	canonical := filepath.Join(dataDir, "bin", "env-switcher")

	info, statErr := os.Lstat(legacy)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return false, nil
		}
		return false, statErr
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, nil // not the shape a legacy install would leave; leave it alone
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Uid != uint32(os.Geteuid()) {
		return false, fmt.Errorf("legacy executable %s is not owned by the current user; leaving it in place", legacy)
	}
	if _, err := os.Stat(canonical); err != nil {
		// The canonical copy isn't there yet — never remove the only executable a user has.
		return false, nil
	}
	if err := os.Remove(legacy); err != nil {
		return false, err
	}
	return true, nil
}
