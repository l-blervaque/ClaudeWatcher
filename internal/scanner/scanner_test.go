package scanner

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ludo/claudewatcher/internal/session"
)

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
