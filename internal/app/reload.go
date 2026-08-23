package app

import "github.com/dolf/env-switcher/internal/config"

func reloadSettings(path string, current *config.Settings) (*config.Settings, error) {
	candidate, err := config.Load(path)
	if err != nil {
		return current, err
	}
	return candidate, nil
}
