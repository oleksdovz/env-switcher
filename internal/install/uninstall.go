package install

import (
	"os"

	"github.com/dolf/env-switcher/internal/fsatomic"
)

func Uninstall(target Target) error {
	original, err := os.ReadFile(target.Profile)
	if err != nil {
		return err
	}
	updated, changed, err := RemoveBlock(original)
	if err != nil {
		return err
	}
	if changed {
		info, _ := os.Stat(target.Profile)
		if _, err := backup(target.Profile, original, info.Mode()); err != nil {
			return err
		}
		if err := fsatomic.WriteFile(target.Profile, updated, info.Mode()); err != nil {
			return err
		}
	}
	if err := os.Remove(target.Executable); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
