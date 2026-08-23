package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Acknowledgement struct {
	Version int    `json:"version"`
	Digest  string `json:"digest"`
}

func AcknowledgementPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "trusted-functions.json"), nil
}

func IsAcknowledged(digest string) bool {
	path, err := AcknowledgementPath()
	if err != nil {
		return false
	}
	if err := EnsurePrivate(path, false); err != nil {
		return false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var a Acknowledgement
	if json.Unmarshal(b, &a) != nil {
		return false
	}
	return a.Version == 1 && a.Digest == digest
}

func Acknowledge(digest string) error {
	dir, err := DataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	path, err := AcknowledgementPath()
	if err != nil {
		return err
	}
	b, err := json.Marshal(Acknowledgement{Version: 1, Digest: digest})
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(dir, ".trusted-functions-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
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
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save acknowledgement: %w", err)
	}
	return nil
}
