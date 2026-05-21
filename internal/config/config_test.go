// internal/config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ludo/claudewatcher/internal/config"
)

func TestLoadReturnsDefaultsWhenFileAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SoundEnabled != false {
		t.Errorf("SoundEnabled = %v, want false", cfg.SoundEnabled)
	}
	if cfg.SoundName != "glass" {
		t.Errorf("SoundName = %q, want \"glass\"", cfg.SoundName)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := config.Config{SoundEnabled: true, SoundName: "ping"}
	if err := config.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() after Save error = %v", err)
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := config.Save(config.Config{SoundEnabled: false, SoundName: "funk"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	path := filepath.Join(tmp, ".config", "claudewatcher", "config.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("config file not created at %s", path)
	}
}
