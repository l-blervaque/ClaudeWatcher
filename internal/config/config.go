package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	SoundEnabled bool   `json:"sound_enabled"`
	SoundName    string `json:"sound_name"`

	// Column visibility (wide layout). All true by default.
	ShowCache  bool `json:"show_cache"`
	ShowCtx    bool `json:"show_ctx"`
	ShowMsgs   bool `json:"show_msgs"`
	ShowAge    bool `json:"show_age"`
	ShowBadges bool `json:"show_badges"`
}

var defaults = Config{
	SoundEnabled: false,
	SoundName:    "glass",
	ShowCache:    true,
	ShowCtx:      true,
	ShowMsgs:     true,
	ShowAge:      true,
	ShowBadges:   true,
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "claudewatcher", "config.json"), nil
}

func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return defaults, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	// Start from defaults so that new boolean fields default to true
	// even for config files written before those fields existed.
	cfg := defaults
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
