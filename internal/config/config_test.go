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
	// Save persists the zero value for RefreshSeconds; Load floors it (<1 → default).
	want.RefreshSeconds = 2
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

func TestRefreshSecondsDefaultAndFloor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// No config file → default 2.
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RefreshSeconds != 2 {
		t.Errorf("default RefreshSeconds = %d, want 2", cfg.RefreshSeconds)
	}

	// A pre-RefreshSeconds config file (field absent → 0) must sanitize to 2.
	dir := filepath.Join(home, ".config", "claudewatcher")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"sound_enabled":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RefreshSeconds != 2 {
		t.Errorf("sanitized RefreshSeconds = %d, want 2", cfg.RefreshSeconds)
	}
}
