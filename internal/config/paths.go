package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".env-switcher"), nil
}

func SettingsPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.yaml"), nil
}

// CurrentEnvPath is the fixed location a successful switch (bare invocation, a project name, or
// --select) writes its shell transaction to. The installed wrapper clears this file before every
// invocation and sources it afterward only if that invocation recreated it, so a non-switch
// command (help, list, version, upgrade, ...) or a failed switch never reactivates a stale one.
func CurrentEnvPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "current-env"), nil
}

// ExecutablePath is the fixed location the installed copy of the CLI lives at, regardless of
// shell. Both `install` and the self-install-on-first-run path resolve it from here.
func ExecutablePath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin", "env-switcher"), nil
}

// ExpandHome expands a leading "~"/"~/" and any "$HOME"/"${HOME}" reference to the current user's
// home directory. This is the application's one controlled, no-shell path-expansion mechanism:
// no other environment variable, no command substitution, no globbing — see ExpandVar, which this
// is built on for the "$HOME"/"${HOME}" half.
func ExpandHome(path string) (string, error) {
	switch {
	case path == "~":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	case strings.HasPrefix(path, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	case strings.HasPrefix(path, "~"):
		return "", fmt.Errorf("only ~/ home expansion is supported")
	}
	if ExpandVar(path, "HOME", "") == path {
		return path, nil // no $HOME/${HOME} reference present; nothing to do
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return ExpandVar(path, "HOME", home), nil
}

// ExpandVar replaces every "$name" or "${name}" reference in s with value, and nothing else — no
// other variable, no command substitution, no globbing, no shell of any kind. A bare "$name" match
// requires name not be immediately followed by another identifier character, so e.g. "$HOME" never
// matches inside "$HOMEBREW", and "$_PROJECT" never matches inside "$_PROJECT2".
func ExpandVar(s, name, value string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '$' {
			rest := s[i+1:]
			if strings.HasPrefix(rest, "{"+name+"}") {
				b.WriteString(value)
				i += 1 + len(name) + 2 // '$' + '{' + name + '}'
				continue
			}
			if strings.HasPrefix(rest, name) {
				after := i + 1 + len(name)
				if after >= len(s) || !isVarIdentByte(s[after]) {
					b.WriteString(value)
					i = after
					continue
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isVarIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// ReferencedVarNames returns, in first-appearance order, the distinct "$name"/"${name}"
// reference names found in s — the same syntax ExpandVar recognizes for one specific name,
// applied generally so a caller can discover which names a value depends on before resolving
// any of them. It is pure syntax: it does not know whether any given name means anything, and
// (like ExpandVar) it never matches "$(", "`", or other shell metacharacters — those pass through
// untouched, exactly as they do today.
func ReferencedVarNames(s string) []string {
	var names []string
	seen := make(map[string]bool)
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			continue
		}
		rest := s[i+1:]
		name := ""
		if strings.HasPrefix(rest, "{") {
			if end := strings.IndexByte(rest, '}'); end > 1 {
				candidate := rest[1:end]
				valid := true
				for j := 0; j < len(candidate); j++ {
					if !isVarIdentByte(candidate[j]) {
						valid = false
						break
					}
				}
				if valid {
					name = candidate
				}
			}
		} else {
			j := 0
			for j < len(rest) && isVarIdentByte(rest[j]) {
				j++
			}
			if j > 0 {
				name = rest[:j]
			}
		}
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

func EnsurePrivate(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if directory != info.IsDir() {
		return fmt.Errorf("unexpected file type for %s", path)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links are not accepted for %s", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("unsafe permissions on %s: require user-only access", path)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("unsafe ownership on %s: current user must own it", path)
	}
	return nil
}
