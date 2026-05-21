package session

import (
	"testing"
	"time"
)

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

func TestDetermineStatus(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name       string
		hasProc    bool
		modAgo     time.Duration
		lastRole   string
		want       Status
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
