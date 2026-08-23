package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func IsolatedHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	realHome, err := os.UserHomeDir()
	if err == nil && filepath.Clean(dir) == filepath.Clean(realHome) {
		t.Fatal("temporary test home resolved to the real user home")
	}
	t.Setenv("HOME", dir)
	return dir
}
