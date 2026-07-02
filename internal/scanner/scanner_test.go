package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ludo/claudewatcher/internal/session"
)

func TestSessionIDsFromCmdline(t *testing.T) {
	const uuid = "feb19cc6-df85-43fc-a894-deca67b44acb"
	const uuid2 = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	cases := []struct {
		name        string
		cmdline     string
		wantSession string
		wantResume  string
	}{
		// --resume names the transcript being resumed; Claude Code forks it
		// into a NEW session file, so it must land in resumeID (hint), never
		// in sessionID (authoritative).
		{"plain resume", "claude --resume " + uuid, "", uuid},
		{"abs path resume", "/Users/ludo/.local/bin/claude --resume " + uuid, "", uuid},
		{"resume equals form", "claude --resume=" + uuid, "", uuid},
		{
			"resume after settings blob",
			`/Users/ludo/.local/bin/claude --settings {"hooks":{"Stop":[]}} --resume ` + uuid,
			"", uuid,
		},
		// --session-id is the transcript's actual id (cmux, /lattice-* scripts).
		{"plain session-id", "claude --session-id " + uuid, uuid, ""},
		{"abs path session-id", "/Users/ludo/.local/bin/claude --session-id " + uuid, uuid, ""},
		{"session-id equals form", "claude --session-id=" + uuid, uuid, ""},
		// Degenerate forms recover nothing.
		{"fresh session, no flags", "claude", "", ""},
		{"resume without value", "claude --resume", "", ""},
		{"resume with non-uuid value", "claude --resume not-a-uuid", "", ""},
		{"session-id without value", "claude --session-id", "", ""},
		{"session-id with non-uuid value", "claude --session-id nope", "", ""},
		// Both present: each goes to its own field.
		{"both flags", "claude --session-id " + uuid + " --resume " + uuid2, uuid, uuid2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotSession, gotResume := sessionIDsFromCmdline(c.cmdline)
			if gotSession != c.wantSession || gotResume != c.wantResume {
				t.Errorf("sessionIDsFromCmdline(%q) = (%q, %q), want (%q, %q)",
					c.cmdline, gotSession, gotResume, c.wantSession, c.wantResume)
			}
		})
	}
}

func TestIsClaudeExe(t *testing.T) {
	cases := []struct {
		cmdline string
		want    bool
	}{
		{"claude --resume abc", true},
		{"/Users/ludo/.local/bin/claude --resume abc", true}, // full-path argv[0] (cmux shim) — pgrep -x missed these
		{"/Users/ludo/.local/bin/claude", true},
		{"node /Users/ludo/claude.js", false}, // argv[0] basename is "node"
		{"claude-extra --flag", false},        // not exactly "claude"
		{"cw", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isClaudeExe(c.cmdline); got != c.want {
			t.Errorf("isClaudeExe(%q) = %v, want %v", c.cmdline, got, c.want)
		}
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

// TestAttributeExactBeatsRecency: a process that names its session by UUID
// (from --resume or --session-id) marks exactly that session, even when a
// different, more-recently-modified transcript sits in the same cwd. This is
// the core guarantee that fixes "ghost" misattribution for identified procs.
func TestAttributeExactBeatsRecency(t *testing.T) {
	now := time.Now()
	const liveID = "11111111-1111-1111-1111-111111111111"
	const deadID = "22222222-2222-2222-2222-222222222222"
	all := []session.Session{
		{ID: liveID, ProjectPath: "/proj1", LastModified: now.Add(-2 * time.Hour)}, // live but quiet
		{ID: deadID, ProjectPath: "/proj1", LastModified: now},                     // dead but most recent
	}
	procs := []claudeProc{{sessionID: liveID, cwd: "/proj1"}}

	attribute(all, procs, now)

	if !all[0].HasProcess {
		t.Error("exact-matched live session should be marked HasProcess")
	}
	if all[1].HasProcess {
		t.Error("more-recent dead transcript must NOT be marked when an exact match exists")
	}
}

// TestAttributeNoUUIDFallsBackToRecency documents the irreducible limit: a
// process with no recoverable UUID can only be placed by recency, so it marks
// the most recently modified free session in its cwd — which may be the wrong
// (dead) one. Real claude launches now carry a UUID, so this is the rare path.
func TestAttributeNoUUIDFallsBackToRecency(t *testing.T) {
	now := time.Now()
	all := []session.Session{
		{ID: "33333333-3333-3333-3333-333333333333", ProjectPath: "/proj2", LastModified: now},                     // most recent (maybe dead)
		{ID: "44444444-4444-4444-4444-444444444444", ProjectPath: "/proj2", LastModified: now.Add(-3 * time.Hour)}, // older (maybe the live one)
	}
	procs := []claudeProc{{sessionID: "", cwd: "/proj2"}} // id-less

	attribute(all, procs, now)

	if !all[0].HasProcess {
		t.Error("id-less fallback should mark the most recent session in the cwd")
	}
	if all[1].HasProcess {
		t.Error("a single id-less proc must mark only one session")
	}
}

// TestAttributeMixedProcs: exact and id-less procs coexist; exact claims its
// session, the id-less proc falls back to the most recent of the REMAINING
// (unclaimed) sessions in its cwd.
func TestAttributeMixedProcs(t *testing.T) {
	now := time.Now()
	const exactID = "55555555-5555-5555-5555-555555555555"
	all := []session.Session{
		{ID: exactID, ProjectPath: "/proj3", LastModified: now.Add(-5 * time.Hour)}, // exact target, oldest
		{ID: "66666666-6666-6666-6666-666666666666", ProjectPath: "/proj3", LastModified: now},
		{ID: "77777777-7777-7777-7777-777777777777", ProjectPath: "/proj3", LastModified: now.Add(-1 * time.Hour)},
	}
	procs := []claudeProc{
		{sessionID: exactID, cwd: "/proj3"}, // exact
		{sessionID: "", cwd: "/proj3"},      // id-less → recency among the unclaimed
	}

	attribute(all, procs, now)

	if !all[0].HasProcess {
		t.Error("exact session must be marked regardless of its (old) mtime")
	}
	if !all[1].HasProcess {
		t.Error("id-less proc should take the most recent unclaimed session")
	}
	if all[2].HasProcess {
		t.Error("third session has no proc and must stay unmarked")
	}
}

func TestFormatDiagnosis(t *testing.T) {
	diags := []ProcDiag{
		{PID: 100, UUID: "11111111-1111-1111-1111-111111111111", Cwd: "/proj1", Status: "matched"},
		{PID: 200, UUID: "", Cwd: "/proj2", Status: "no-uuid (recency)"},
		{PID: 300, UUID: "22222222-2222-2222-2222-222222222222", Cwd: "/proj3", Status: "UNMATCHED (no transcript)"},
	}
	out := FormatDiagnosis(diags)
	// Every pid, uuid (or a dash), cwd and status must appear so the audit is usable.
	for _, want := range []string{"100", "200", "300", "11111111", "/proj1", "/proj2", "/proj3", "matched", "no-uuid", "UNMATCHED", "3 process"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatDiagnosis output missing %q\n--- got ---\n%s", want, out)
		}
	}
	// An id-less proc shows a dash rather than an empty UUID column.
	if !strings.Contains(out, "-") {
		t.Error("id-less proc should render a dash for the UUID")
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

// TestAttributeResumeForkFallsBackToRecency is THE bug that made the fix line
// unusable: `claude --resume <old-uuid>` forks into a new session file, so the
// old transcript is dead (stale mtime) while the live conversation writes to a
// new uuid. The old exact-match logic pinned the process to the dead file
// (ghost) and never fell back, so the live session showed as ended.
func TestAttributeResumeForkFallsBackToRecency(t *testing.T) {
	now := time.Now()
	const oldID = "88888888-8888-8888-8888-888888888888"  // resumed-from, dead
	const forkID = "99999999-9999-9999-9999-999999999999" // live fork target
	all := []session.Session{
		{ID: oldID, ProjectPath: "/proj4", LastModified: now.Add(-2 * time.Hour)},
		{ID: forkID, ProjectPath: "/proj4", LastModified: now.Add(-30 * time.Second)},
	}
	procs := []claudeProc{{resumeID: oldID, cwd: "/proj4"}}

	attribute(all, procs, now)

	if all[0].HasProcess {
		t.Error("stale resumed-from transcript must NOT be marked (it is the fork source, dead)")
	}
	if !all[1].HasProcess {
		t.Error("recency fallback must mark the live forked transcript")
	}
}

// TestAttributeResumeFreshExactWins: while the resumed transcript is still
// fresh (old Claude versions that write in place, or the instant right after
// resume), the exact resume id wins over a more recent decoy in the same cwd.
func TestAttributeResumeFreshExactWins(t *testing.T) {
	now := time.Now()
	const resumedID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const decoyID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	all := []session.Session{
		{ID: resumedID, ProjectPath: "/proj5", LastModified: now.Add(-30 * time.Second)},
		{ID: decoyID, ProjectPath: "/proj5", LastModified: now},
	}
	procs := []claudeProc{{resumeID: resumedID, cwd: "/proj5"}}

	attribute(all, procs, now)

	if !all[0].HasProcess {
		t.Error("fresh resumed transcript must be exact-matched")
	}
	if all[1].HasProcess {
		t.Error("decoy must not be marked when the resume id exact-matches a fresh transcript")
	}
}

// TestAttributeSessionIDNoTranscriptMarksNothing: a --session-id proc whose
// transcript does not exist yet (session just started, first write pending)
// must NOT recency-mark an unrelated file — the transcript appears next tick.
func TestAttributeSessionIDNoTranscriptMarksNothing(t *testing.T) {
	now := time.Now()
	all := []session.Session{
		{ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", ProjectPath: "/proj6", LastModified: now},
	}
	procs := []claudeProc{{sessionID: "dddddddd-dddd-dddd-dddd-dddddddddddd", cwd: "/proj6"}}

	attribute(all, procs, now)

	if all[0].HasProcess {
		t.Error("an unmatched --session-id proc must not mark an unrelated transcript")
	}
}

// TestAttributeResumeUnmatchedFallsBack: a --resume proc whose old transcript
// was deleted still represents a live session; it falls back to recency.
func TestAttributeResumeUnmatchedFallsBack(t *testing.T) {
	now := time.Now()
	all := []session.Session{
		{ID: "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", ProjectPath: "/proj7", LastModified: now},
	}
	procs := []claudeProc{{resumeID: "ffffffff-ffff-ffff-ffff-ffffffffffff", cwd: "/proj7"}}

	attribute(all, procs, now)

	if !all[0].HasProcess {
		t.Error("resume proc with no matching transcript must fall back to recency")
	}
}
