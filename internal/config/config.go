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
	ShowModel  bool `json:"show_model"`
	ShowVer    bool `json:"show_ver"`
	ShowBadges bool `json:"show_badges"`

	// NerdFonts enables Nerd Font icons instead of plain Unicode/ASCII symbols.
	// Opt-in: false by default so plain terminals are not affected.
	NerdFonts bool `json:"nerd_fonts"`

	// RefreshSeconds is the TUI auto-refresh interval in seconds. Minimum 1.
	RefreshSeconds int `json:"refresh_seconds"`
}

var defaults = Config{
	SoundEnabled:   false,
	SoundName:      "glass",
	ShowCache:      true,
	ShowCtx:        true,
	ShowMsgs:       true,
	ShowAge:        true,
	ShowModel:      true,
	ShowVer:        true,
	ShowBadges:     true,
	RefreshSeconds: 2,
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
	// Sanitize: config files written before RefreshSeconds existed decode it
	// as 0 (json zero value overwrites the seeded default); floor to default.
	if cfg.RefreshSeconds < 1 {
		cfg.RefreshSeconds = defaults.RefreshSeconds
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
