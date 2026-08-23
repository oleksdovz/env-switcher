package install

import (
	"path/filepath"
	"testing"
)

func TestAcquireLockContention(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "lock")
	unlock, err := acquireLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := acquireLock(lock); err == nil {
		t.Fatal("concurrent lock acquired")
	}
}
