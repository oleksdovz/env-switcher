package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dolf/env-switcher/internal/config"
	"github.com/dolf/env-switcher/internal/fsatomic"
)

type backupMeta struct {
	Source, Data, Digest string
	Mode                 uint32
	Created              string
}

func backup(path string, data []byte, mode os.FileMode) (string, error) {
	dir, _ := config.DataDir()
	dir = filepath.Join(dir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	id := time.Now().UTC().Format("20060102T150405.000000000Z")
	dataPath := filepath.Join(dir, id+".profile")
	sum := sha256.Sum256(data)
	if err := fsatomic.WriteFile(dataPath, data, 0o600); err != nil {
		return "", err
	}
	meta := backupMeta{Source: path, Data: dataPath, Digest: hex.EncodeToString(sum[:]), Mode: uint32(mode.Perm()), Created: time.Now().UTC().Format(time.RFC3339Nano)}
	b, _ := json.Marshal(meta)
	if err := fsatomic.WriteFile(filepath.Join(dir, id+".json"), append(b, '\n'), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

func latestBackup(path string) (backupMeta, error) {
	dir, _ := config.DataDir()
	files, _ := filepath.Glob(filepath.Join(dir, "backups", "*.json"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var m backupMeta
		if json.Unmarshal(b, &m) == nil && m.Source == path {
			return m, nil
		}
	}
	return backupMeta{}, fmt.Errorf("no backup found for profile")
}

func restoreLatest(path string) error {
	m, err := latestBackup(path)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(m.Data)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != m.Digest {
		return fmt.Errorf("backup digest mismatch")
	}
	return fsatomic.WriteFile(path, b, os.FileMode(m.Mode))
}
