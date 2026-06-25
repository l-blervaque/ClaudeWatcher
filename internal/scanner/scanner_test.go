package scanner

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ludo/claudewatcher/internal/session"
)

func TestSessionIDFromCmdline(t *testing.T) {
	const uuid = "feb19cc6-df85-43fc-a894-deca67b44acb"
	cases := []struct {
		name    string
		cmdline string
		want    string
	}{
		{"plain resume", "claude --resume " + uuid, uuid},
		{"abs path resume", "/Users/ludo/.local/bin/claude --resume " + uuid, uuid},
		{"equals form", "claude --resume=" + uuid, uuid},
		{
			// cmux wraps a long --settings JSON blob before --resume; the uuid is
			// still recovered from the trailing argument.
			"resume after settings blob",
			`/Users/ludo/.local/bin/claude --settings {"hooks":{"Stop":[]}} --resume ` + uuid,
			uuid,
		},
		{"fresh session, no resume", "claude", ""},
		{"resume without value", "claude --resume", ""},
		{"resume with non-uuid value", "claude --resume not-a-uuid", ""},
		// `--session-id <uuid>` is the other exact-identity form: skills and
		// scripts (e.g. /lattice-*) launch claude with a pre-assigned id rather
		// than --resume. It names the transcript directly, so treat it like resume.
		{"plain session-id", "claude --session-id " + uuid, uuid},
		{"abs path session-id", "/Users/ludo/.local/bin/claude --session-id " + uuid, uuid},
		{"session-id equals form", "claude --session-id=" + uuid, uuid},
		{"session-id without value", "claude --session-id", ""},
		{"session-id with non-uuid value", "claude --session-id nope", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sessionIDFromCmdline(c.cmdline); got != c.want {
				t.Errorf("sessionIDFromCmdline(%q) = %q, want %q", c.cmdline, got, c.want)
			}
		})
	}
}

func TestIsSessionProc(t *testing.T) {
	const uuid = "feb19cc6-df85-43fc-a894-deca67b44acb"
	cases := []struct {
		name    string
		cmdline string
		want    bool
	}{
		{"resumed session", "claude --resume " + uuid, true},
		{"fresh session", "claude", true},
		{"abs path session", "/Users/ludo/.local/bin/claude --resume " + uuid, true},
		{"daemon", "claude daemon run", false},
		{"abs path daemon", "/Users/ludo/.local/bin/claude daemon", false},
		{"headless -p", "claude -p \"summarize\"", false},
		{"headless --print", "claude --print \"summarize\"", false},
		{"prompt containing daemon word is not the daemon", "claude --resume " + uuid + " restart the daemon", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isSessionProc(c.cmdline); got != c.want {
				t.Errorf("isSessionProc(%q) = %v, want %v", c.cmdline, got, c.want)
			}
		})
	}
}

func TestSockRE(t *testing.T) {
	const arg = "/tmp/cc-daemon-501/e0d9d869/pty/1fe22414.sock"
	m := sockRE.FindStringSubmatch(arg)
	if m == nil {
		t.Fatalf("sockRE did not match %q", arg)
	}
	if m[1] != "1fe22414" {
		t.Errorf("sockRE prefix = %q, want %q", m[1], "1fe22414")
	}
}

// TestScanReusesCacheForUnchangedFiles verifies that Scan parses each session
// file once and then reuses the cached result on subsequent scans, as long as
// the file's mtime and size are unchanged. Without the cache, every tick
// re-parsed every historical file, which pinned the CPU.
func TestScanReusesCacheForUnchangedFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projDir := filepath.Join(home, ".claude", "projects", "-Users-test-proj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonl := filepath.Join(projDir, "11111111-1111-1111-1111-111111111111.jsonl")
	content := `{"type":"user","cwd":"/Users/test/proj","message":{"role":"user","content":"hi"}}` + "\n"
	if err := os.WriteFile(jsonl, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reset cache and install a counting parser.
	statsCacheMu.Lock()
	statsCache = map[string]statsCacheEntry{}
	statsCacheMu.Unlock()
	var parses int64
	orig := parseJSONL
	parseJSONL = func(path string) (session.JSONLStats, error) {
		atomic.AddInt64(&parses, 1)
		return orig(path)
	}
	t.Cleanup(func() { parseJSONL = orig })

	opts := ScanOptions{IncludeEnded: true}

	if _, err := Scan(opts); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&parses); got != 1 {
		t.Fatalf("first scan: expected 1 parse, got %d", got)
	}

	// Second scan with no file change must hit the cache — no new parse.
	if _, err := Scan(opts); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&parses); got != 1 {
		t.Fatalf("second scan (unchanged): expected still 1 parse, got %d", got)
	}

	// Touching the file (new mtime + larger size) must invalidate the cache.
	if err := os.WriteFile(jsonl, []byte(content+content), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(jsonl, future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(opts); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&parses); got != 2 {
		t.Fatalf("third scan (changed): expected 2 parses, got %d", got)
	}
}
