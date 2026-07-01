package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestScanJSONLVersion verifies the CLI version is extracted and that the
// last non-empty value wins (a session resumed across a CLI upgrade should
// report the version currently running).
func TestScanJSONLVersion(t *testing.T) {
	dir := t.TempDir()

	t.Run("last non-empty wins", func(t *testing.T) {
		path := filepath.Join(dir, "a.jsonl")
		lines := `{"type":"user","version":"2.1.180","message":{"role":"user","content":"hi"}}
{"type":"assistant","version":"2.1.180","message":{"role":"assistant","model":"claude-opus-4-8","content":[{"type":"text","text":"ok"}]}}
{"type":"user","version":"2.1.197","message":{"role":"user","content":"more"}}
`
		if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
			t.Fatal(err)
		}
		stats, err := ScanJSONL(path)
		if err != nil {
			t.Fatal(err)
		}
		if stats.Version != "2.1.197" {
			t.Errorf("Version = %q, want %q", stats.Version, "2.1.197")
		}
	})

	t.Run("missing version stays empty", func(t *testing.T) {
		path := filepath.Join(dir, "b.jsonl")
		lines := `{"type":"user","message":{"role":"user","content":"hi"}}
`
		if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
			t.Fatal(err)
		}
		stats, err := ScanJSONL(path)
		if err != nil {
			t.Fatal(err)
		}
		if stats.Version != "" {
			t.Errorf("Version = %q, want empty", stats.Version)
		}
	})
}

func TestDecodeProjectDir(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"-Users-ludo-foo", "/Users/ludo/foo"},
		{"-Users-ludo", "/Users/ludo"},
		{"plain", "plain"},
	}
	for _, tt := range tests {
		got := DecodeProjectDir(tt.in)
		if got != tt.want {
			t.Errorf("DecodeProjectDir(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestProjectNameFromDir(t *testing.T) {
	// Note: dashes in the original path are ambiguous (they collide
	// with the "/" → "-" encoding). The scanner prefers the real cwd
	// from the jsonl when available; ProjectNameFromDir is a fallback.
	got := ProjectNameFromDir("-Users-ludo-Documents-foo")
	want := "foo"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestContextWindowFor(t *testing.T) {
	cases := []struct {
		model string
		want  int
	}{
		{"claude-opus-4-5", 1_000_000},
		{"claude-opus-4-7", 1_000_000},
		{"claude-sonnet-4-5", 200_000}, // Sonnet 4.5 stays at 200K
		{"claude-sonnet-4-6", 200_000}, // Sonnet 4.6 stays at 200K (bug fix)
		{"claude-sonnet-4-7", 1_000_000},
		{"claude-haiku-4-5", 200_000},
		{"unknown-model", 200_000},
	}
	for _, c := range cases {
		got := ContextWindowFor(c.model)
		if got != c.want {
			t.Errorf("ContextWindowFor(%q) = %d, want %d", c.model, got, c.want)
		}
	}
}

func TestModelLabel(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"claude-opus-4-7", "Opus 4.7"},
		{"claude-opus-4-8", "Opus 4.8"},
		{"claude-sonnet-4-6", "Sonnet 4.6"},
		{"claude-haiku-4-5-20251001", "Haiku 4.5"}, // date suffix dropped
		{"claude-fable-5", "Fable 5"},
		{"opus", "Opus"}, // short alias, no version
		{"sonnet", "Sonnet"},
		{"haiku", "Haiku"},
		{"<synthetic>", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := ModelLabel(c.id); got != c.want {
			t.Errorf("ModelLabel(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func TestDetermineStatus(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		hasProc  bool
		modAgo   time.Duration
		lastRole string
		want     Status
	}{
		{"no process → ended", false, time.Minute, "user", StatusEnded},
		{"fresh activity → active", true, 2 * time.Second, "assistant", StatusActive},
		{"waiting user input", true, 30 * time.Second, "assistant", StatusWaiting},
		{"long idle", true, 10 * time.Minute, "user", StatusIdle},
	}
	for _, c := range cases {
		got := DetermineStatus(c.hasProc, now.Add(-c.modAgo), c.lastRole, now)
		if got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
