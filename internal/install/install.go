package install

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/fsatomic"
)

func Install(target Target, sourceExecutable string) (Result, error) {
	dir, _ := config.DataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Result{}, err
	}
	unlock, err := acquireLock(filepath.Join(dir, "install.lock"))
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	original, err := os.ReadFile(target.Profile)
	existed := err == nil
	mode := os.FileMode(0o600)
	if err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	if info, e := os.Stat(target.Profile); e == nil {
		mode = info.Mode().Perm()
	}
	wrapper := zshWrapper
	if target.Shell == "bash" {
		wrapper = bashWrapper
	}
	updated, changed, err := Reconcile(original, Block(wrapper))
	if err != nil {
		return Result{}, err
	}
	binary, err := os.ReadFile(sourceExecutable)
	if err != nil {
		return Result{}, err
	}
	installed, _ := os.ReadFile(target.Executable)
	binaryChanged := !bytes.Equal(binary, installed)
	result := Result{Changed: changed || binaryChanged, Profile: target.Profile}
	if !result.Changed {
		return result, nil
	}
	if existed {
		result.Backup, err = backup(target.Profile, original, mode)
		if err != nil {
			return Result{}, err
		}
	}
	if err := fsatomic.WriteFile(target.Executable, binary, 0o700); err != nil {
		return Result{}, err
	}
	if err := fsatomic.WriteFile(target.Profile, updated, mode); err != nil {
		return Result{}, err
	}
	verifyBinary, err := os.ReadFile(target.Executable)
	if err != nil || !bytes.Equal(verifyBinary, binary) {
		return Result{}, fmt.Errorf("installed executable verification failed")
	}
	verifyProfile, err := os.ReadFile(target.Profile)
	if err != nil || !bytes.Equal(verifyProfile, updated) || bytes.Count(verifyProfile, []byte(Begin)) != 1 {
		return Result{}, fmt.Errorf("installed profile verification failed")
	}
	return result, nil
}
