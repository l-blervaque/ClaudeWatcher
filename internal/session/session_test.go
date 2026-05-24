package session

import (
	"os"
	"testing"
	"time"
)

func writeTempJSONL(t *testing.T, lines []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.jsonl")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	for _, l := range lines {
		f.WriteString(l + "\n") //nolint:errcheck
	}
	return f.Name()
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
		{"claude-sonnet-4-5", 200_000},  // Sonnet 4.5 stays at 200K
		{"claude-sonnet-4-6", 200_000},  // Sonnet 4.6 stays at 200K (bug fix)
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

func TestScanJSONL_CompactBoundary(t *testing.T) {
	lines := []string{
		`{"type":"system","subtype":"init","cwd":"/tmp/proj"}`,
		`{"type":"system","subtype":"compact_boundary","durationMs":5000}`,
		`{"type":"system","subtype":"compact_boundary","durationMs":2000}`,
		`{"type":"system","subtype":"compact_boundary","durationMs":500}`, // brisée, pas comptée
	}
	stats, err := ScanJSONL(writeTempJSONL(t, lines))
	if err != nil {
		t.Fatalf("ScanJSONL error: %v", err)
	}
	if stats.CompactBoundaryCount != 2 {
		t.Errorf("CompactBoundaryCount = %d, want 2", stats.CompactBoundaryCount)
	}
}

func TestScanJSONL_CompactBoundaryZero(t *testing.T) {
	lines := []string{
		`{"type":"system","subtype":"init","cwd":"/tmp/proj"}`,
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
	}
	stats, err := ScanJSONL(writeTempJSONL(t, lines))
	if err != nil {
		t.Fatalf("ScanJSONL error: %v", err)
	}
	if stats.CompactBoundaryCount != 0 {
		t.Errorf("CompactBoundaryCount = %d, want 0", stats.CompactBoundaryCount)
	}
}

func TestScanJSONL_ActiveSkill(t *testing.T) {
	lines := []string{
		`{"type":"system","subtype":"init","cwd":"/tmp/proj"}`,
		`{"type":"assistant","attributionSkill":"superpowers:brainstorming","message":{"role":"assistant","model":"claude-sonnet-4-6","content":"hello","usage":{"input_tokens":50}}}`,
		`{"type":"assistant","attributionSkill":"superpowers:test-driven-development","message":{"role":"assistant","model":"claude-sonnet-4-6","content":"world","usage":{"input_tokens":80}}}`,
	}
	stats, err := ScanJSONL(writeTempJSONL(t, lines))
	if err != nil {
		t.Fatalf("ScanJSONL error: %v", err)
	}
	if stats.ActiveSkill != "superpowers:test-driven-development" {
		t.Errorf("ActiveSkill = %q, want %q", stats.ActiveSkill, "superpowers:test-driven-development")
	}
}

func TestScanJSONL_ActiveSkillEmpty(t *testing.T) {
	lines := []string{
		`{"type":"system","subtype":"init","cwd":"/tmp/proj"}`,
		`{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-6","content":"no skill","usage":{"input_tokens":50}}}`,
	}
	stats, err := ScanJSONL(writeTempJSONL(t, lines))
	if err != nil {
		t.Fatalf("ScanJSONL error: %v", err)
	}
	if stats.ActiveSkill != "" {
		t.Errorf("ActiveSkill = %q, want empty", stats.ActiveSkill)
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
