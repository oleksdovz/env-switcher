package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dolf/env-switcher/internal/config"
)

func Resolve(shellName, override string) (Target, error) {
	if shellName != "bash" && shellName != "zsh" {
		return Target{}, fmt.Errorf("supported shells are bash and zsh")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Target{}, err
	}
	profile := override
	if profile == "" {
		if shellName == "bash" {
			profile = filepath.Join(home, ".bashrc")
		} else {
			profile = filepath.Join(home, ".zshrc")
		}
	}
	if !filepath.IsAbs(profile) {
		return Target{}, fmt.Errorf("profile path must be absolute")
	}
	if info, err := os.Lstat(profile); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return Target{}, fmt.Errorf("profile must be a regular non-symlink file")
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Uid != uint32(os.Geteuid()) {
			return Target{}, fmt.Errorf("profile must be owned by the current user")
		}
	} else if !os.IsNotExist(err) {
		return Target{}, err
	}
	dir, err := config.DataDir()
	if err != nil {
		return Target{}, err
	}
	if rel, err := filepath.Rel(dir, profile); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return Target{}, fmt.Errorf("profile override must be outside env-switcher data directory")
	}
	executable, err := config.ExecutablePath()
	if err != nil {
		return Target{}, err
	}
	return Target{Shell: shellName, Profile: profile, Executable: executable}, nil
}
