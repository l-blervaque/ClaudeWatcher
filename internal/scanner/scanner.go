package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ludo/claudewatcher/internal/session"
)

// parseJSONL parses a session file into stats. Indirected through a variable
// so tests can count how often a file is actually parsed.
var parseJSONL = session.ScanJSONL

// statsCacheEntry remembers the parse result for a file, keyed by the file
// identity (mtime + size). A session jsonl's parsed stats are a pure function
// of its contents, so if neither mtime nor size changed since the last scan we
// can reuse the cached stats instead of re-reading and re-parsing the file.
type statsCacheEntry struct {
	modTime time.Time
	size    int64
	stats   session.JSONLStats
}

var (
	statsCacheMu sync.Mutex
	statsCache   = map[string]statsCacheEntry{}
)

// scanJSONLCached returns parsed stats for path, reusing a cached result when
// the file's mtime and size are unchanged. This keeps idle scans cheap: only
// the handful of files that changed since the previous tick are re-parsed,
// rather than every historical session file on every tick.
func scanJSONLCached(path string, info os.FileInfo) session.JSONLStats {
	modTime, size := info.ModTime(), info.Size()

	statsCacheMu.Lock()
	entry, ok := statsCache[path]
	statsCacheMu.Unlock()
	if ok && entry.size == size && entry.modTime.Equal(modTime) {
		return entry.stats
	}

	stats, _ := parseJSONL(path)

	statsCacheMu.Lock()
	statsCache[path] = statsCacheEntry{modTime: modTime, size: size, stats: stats}
	statsCacheMu.Unlock()
	return stats
}

// projectsRoot returns ~/.claude/projects
func projectsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// claudeProc describes a live `claude` process that represents a real session.
type claudeProc struct {
	// cwd is the process working directory (used for the recency fallback
	// when the exact session can't be recovered).
	cwd string
	// sessionID is the full session UUID recovered from a `--resume <uuid>`
	// argument. This is the strongest identity signal — exact and globally
	// unique — and is present for every resumed terminal / cmux / tmux session.
	// Empty for a freshly started session (no --resume on its command line).
	sessionID string
	// sessionPrefix is the leading hex of the session UUID, recovered from a
	// desktop-app PTY-host socket path (…/pty/<prefix>.sock). Used as a
	// secondary signal when no --resume id is present (desktop app). Empty when
	// the process carries no recoverable identity (plain terminal, VS Code).
	sessionPrefix string
}

// sockRE pulls the session-id prefix out of a PTY-host socket arg such as
// `/tmp/cc-daemon-501/e0d9d869/pty/1fe22414.sock`.
var sockRE = regexp.MustCompile(`/pty/([0-9a-fA-F]+)\.sock`)

// uuidRE matches a canonical session UUID (the value passed to `--resume`).
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// sessionIDFromCmdline recovers the full session UUID from a `claude` command
// line that names its session, via either `--resume <uuid>` (terminal / cmux
// resume) or `--session-id <uuid>` (skills/scripts that pre-assign an id, e.g.
// /lattice-*). Both the space and `=` forms are accepted. Returns "" when no id
// is present (a freshly started session). Pure function over the command line
// so it is unit-testable without spawning procs.
func sessionIDFromCmdline(cmdline string) string {
	fields := strings.Fields(cmdline)
	for i, f := range fields {
		for _, flag := range []string{"--resume", "--session-id"} {
			if v, ok := strings.CutPrefix(f, flag+"="); ok {
				if uuidRE.MatchString(v) {
					return v
				}
				continue
			}
			if f == flag && i+1 < len(fields) && uuidRE.MatchString(fields[i+1]) {
				return fields[i+1]
			}
		}
	}
	return ""
}

// runningClaudeProcs returns the live `claude` processes that represent real
// sessions. `pgrep -x claude` matches the exact executable name, but that name
// is shared by the background daemon and headless `claude -p` runs, which are
// not sessions and would inflate the per-cwd count — those are dropped here.
// For exact attribution we recover the session id two ways: from a
// `--resume <uuid>` argument (the common case — terminal / cmux / tmux) and,
// failing that, from a desktop-app PTY-host socket path (…/pty/<prefix>.sock).
func runningClaudeProcs() []claudeProc {
	out, err := exec.Command("pgrep", "-x", "claude").Output()
	if err != nil {
		return nil
	}
	pids := strings.Fields(string(out))
	if len(pids) == 0 {
		return nil
	}

	// Batch the per-pid lookups into one ps and one lsof call. Both tools have
	// heavy per-invocation startup cost, so spawning them once per pid (34
	// processes per tick at 17 sessions) was the dominant cost of a scan.
	// Ignore exit status on both: each exits non-zero when any pid in the list
	// has already exited (the gap between pgrep and here), but still prints the
	// surviving pids on stdout — parsing what we got preserves the per-pid
	// resilience of the old one-call-per-pid loops.

	// cmdline per pid: `ps -o pid=,command=` prints "<pid> <command…>". -ww
	// disables column truncation, so a session id near the end of a long command
	// line isn't cut off (which would fail the uuid match and drop the process to
	// the fragile recency fallback).
	cmdByPid := map[string]string{}
	psOut, _ := exec.Command("ps", "-ww", "-o", "pid=,command=", "-p", strings.Join(pids, ",")).Output()
	for _, line := range strings.Split(string(psOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		cmdByPid[parts[0]] = strings.TrimSpace(parts[1])
	}

	// cwd per pid: `-Fpn` emits a `p<pid>` line followed by the `n<path>` cwd
	// line for that process, so we can key each cwd to its pid.
	cwdByPid := map[string]string{}
	lo, _ := exec.Command("lsof", "-a", "-p", strings.Join(pids, ","), "-d", "cwd", "-Fpn", "-w").Output()
	var curPid string
	for _, line := range strings.Split(string(lo), "\n") {
		switch {
		case strings.HasPrefix(line, "p"):
			curPid = line[1:]
		case strings.HasPrefix(line, "n/") && curPid != "":
			cwdByPid[curPid] = strings.TrimPrefix(line, "n")
		}
	}

	var procs []claudeProc
	for _, pid := range pids {
		cmdline, ok := cmdByPid[pid]
		if !ok || !isSessionProc(cmdline) {
			continue
		}
		var prefix string
		if m := sockRE.FindStringSubmatch(cmdline); m != nil {
			prefix = m[1]
		}
		procs = append(procs, claudeProc{
			cwd:           cwdByPid[pid],
			sessionID:     sessionIDFromCmdline(cmdline),
			sessionPrefix: prefix,
		})
	}
	return procs
}

// isSessionProc reports whether a `claude` command line represents a real
// session. Only the genuine non-sessions are rejected: the background daemon
// and headless `claude -p` / `--print` runs. Desktop PTY-host helpers, the
// VS Code extension, and plain terminal sessions all pass.
func isSessionProc(cmdline string) bool {
	fields := strings.Fields(cmdline)
	// The daemon runs as `claude daemon …`, so "daemon" is the first argument
	// after the executable. Match it positionally rather than as a substring,
	// so a session whose prompt or path merely contains "daemon" is not dropped.
	if len(fields) >= 2 && fields[1] == "daemon" {
		return false
	}
	// Headless print mode. Match whole arguments so a path containing "-p"
	// doesn't trip the check.
	for _, field := range fields {
		if field == "-p" || field == "--print" {
			return false
		}
	}
	return true
}

// ScanOptions controls how Scan filters sessions.
type ScanOptions struct {
	// IncludeEnded: when true, return sessions with no associated running
	// claude process. Default false — we only want sessions actually open.
	IncludeEnded bool
}

// Scan walks ~/.claude/projects and returns sessions.
//
// By default only sessions associated with a running `claude` process
// are returned. For each project with N running processes, the N most
// recently modified jsonl files are matched to those processes.
func Scan(opts ScanOptions) ([]session.Session, error) {
	root, err := projectsRoot()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	procs := runningClaudeProcs()
	now := time.Now()

	var all []session.Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectDir := entry.Name()
		projectPath := session.DecodeProjectDir(projectDir)
		projectName := session.ProjectNameFromDir(projectDir)

		fullDir := filepath.Join(root, projectDir)
		files, err := os.ReadDir(fullDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				// Check for <session-uuid>/subagents/ directory structure.
				subagentsDir := filepath.Join(fullDir, f.Name(), "subagents")
				subFiles, err := os.ReadDir(subagentsDir)
				if err != nil {
					continue
				}
				for _, sf := range subFiles {
					if sf.IsDir() || !strings.HasSuffix(sf.Name(), ".jsonl") {
						continue
					}
					jsonlPath := filepath.Join(subagentsDir, sf.Name())
					info, err := sf.Info()
					if err != nil {
						continue
					}
					stats := scanJSONLCached(jsonlPath, info)
					id := strings.TrimSuffix(sf.Name(), ".jsonl")

					path := projectPath
					name := projectName
					if stats.Cwd != "" {
						path = stats.Cwd
						name = filepath.Base(stats.Cwd)
					}

					title, source := resolveTitle(stats)
					all = append(all, session.Session{
						ID:               id,
						ProjectDir:       projectDir,
						ProjectPath:      path,
						ProjectName:      name,
						Title:            title,
						TitleSource:      source,
						CustomTitle:      stats.CustomTitle,
						AiTitle:          stats.AiTitle,
						FirstPrompt:      cleanTitle(stats.FirstPrompt),
						JSONLPath:        jsonlPath,
						LastModified:     info.ModTime(),
						MessageCount:     stats.MessageCount,
						LastRole:         stats.LastRole,
						ContextTokens:    stats.ContextTokens,
						Model:            stats.Model,
						Version:          stats.Version,
						LastAssistant:    stats.LastAssistant,
						IsSubagent:       stats.IsSubagent,
						ParentID:         f.Name(), // directory name = parent session UUID
						CacheEfficiency:  stats.CacheEfficiency,
						AwaySummaryCount: stats.AwaySummaryCount,
						ApiErrorCount:    stats.ApiErrorCount,
						TurnCount:        stats.TurnCount,
						ApiErrorRate:     stats.ApiErrorRate,
						QueueDepth:       stats.QueueDepth,
					})
				}
				continue
			}
			if !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			jsonlPath := filepath.Join(fullDir, f.Name())
			info, err := f.Info()
			if err != nil {
				continue
			}
			stats := scanJSONLCached(jsonlPath, info)
			id := strings.TrimSuffix(f.Name(), ".jsonl")

			// Prefer the real cwd from the jsonl — folder-name decoding
			// is ambiguous on "-" vs "/" boundaries.
			path := projectPath
			name := projectName
			if stats.Cwd != "" {
				path = stats.Cwd
				name = filepath.Base(stats.Cwd)
			}

			title, source := resolveTitle(stats)
			all = append(all, session.Session{
				ID:               id,
				ProjectDir:       projectDir,
				ProjectPath:      path,
				ProjectName:      name,
				Title:            title,
				TitleSource:      source,
				CustomTitle:      stats.CustomTitle,
				AiTitle:          stats.AiTitle,
				FirstPrompt:      cleanTitle(stats.FirstPrompt),
				JSONLPath:        jsonlPath,
				LastModified:     info.ModTime(),
				MessageCount:     stats.MessageCount,
				LastRole:         stats.LastRole,
				ContextTokens:    stats.ContextTokens,
				Model:            stats.Model,
				Version:          stats.Version,
				LastAssistant:    stats.LastAssistant,
				IsSubagent:       stats.IsSubagent,
				CacheEfficiency:  stats.CacheEfficiency,
				AwaySummaryCount: stats.AwaySummaryCount,
				ApiErrorCount:    stats.ApiErrorCount,
				TurnCount:        stats.TurnCount,
				ApiErrorRate:     stats.ApiErrorRate,
				QueueDepth:       stats.QueueDepth,
			})
		}
	}

	// Attribute live processes to MAIN sessions. Subagents are excluded from
	// this competition: they inherit HasProcess from their parent main session
	// in the second pass below.
	//
	// Two-stage attribution:
	//  1. Exact match — when a process names its own session, mark exactly that
	//     session so an unrelated transcript in the same folder (e.g. one just
	//     touched by `/exit`) can't steal it. The identity comes from either a
	//     `--resume <uuid>` argument (full id, matched globally) or, failing
	//     that, a desktop PTY-host socket prefix (matched within the cwd to
	//     bound the tiny prefix-collision risk).
	//  2. Recency fallback — processes with no recoverable session id (a
	//     freshly started terminal session) are counted per cwd; the top-N most
	//     recently modified main sessions in that cwd that weren't already
	//     claimed in stage 1 are marked open.
	byPath := map[string][]int{}
	mainByID := map[string]int{} // full session ID -> index in all (main sessions only)
	for i, s := range all {
		if s.ParentID != "" {
			continue // subagents handled separately
		}
		byPath[s.ProjectPath] = append(byPath[s.ProjectPath], i)
		mainByID[s.ID] = i
	}

	// Stage 1: exact session-id attribution.
	mainHasProcess := map[string]bool{} // session ID -> has process (for subagent pass)
	fallbackCount := map[string]int{}   // cwd -> count of id-less processes
	for _, p := range procs {
		switch {
		case p.sessionID != "":
			// Strongest signal: a full UUID is globally unique, so match it
			// across all main sessions regardless of cwd (covers the case where
			// lsof failed to recover the process cwd).
			if i, ok := mainByID[p.sessionID]; ok {
				all[i].HasProcess = true
				mainHasProcess[all[i].ID] = true
			}
		case p.sessionPrefix != "":
			for _, i := range byPath[p.cwd] {
				if strings.HasPrefix(all[i].ID, p.sessionPrefix) {
					all[i].HasProcess = true
					mainHasProcess[all[i].ID] = true
				}
			}
		default:
			fallbackCount[p.cwd]++
		}
	}

	// Stage 2: recency fallback for id-less processes.
	for path, idxs := range byPath {
		n := fallbackCount[path]
		if n == 0 {
			continue
		}
		// Only sessions not already claimed in stage 1 compete.
		free := make([]int, 0, len(idxs))
		for _, i := range idxs {
			if !all[i].HasProcess {
				free = append(free, i)
			}
		}
		sort.Slice(free, func(a, b int) bool {
			return all[free[a]].LastModified.After(all[free[b]].LastModified)
		})
		if n > len(free) {
			n = len(free)
		}
		for _, i := range free[:n] {
			all[i].HasProcess = true
			mainHasProcess[all[i].ID] = true
		}
	}
	// Subagents inherit HasProcess from their parent main session only if
	// their jsonl file was modified recently (same 5-minute threshold used by
	// DetermineStatus). Stale/finished subagents must never inherit the flag
	// just because their parent process is still alive.
	for i, s := range all {
		if s.ParentID != "" && mainHasProcess[s.ParentID] && now.Sub(s.LastModified) < 5*time.Minute {
			all[i].HasProcess = true
		}
	}

	out := make([]session.Session, 0, len(all))
	for _, s := range all {
		s.Status = session.DetermineStatus(s.HasProcess, s.LastModified, s.LastRole, now)
		if !opts.IncludeEnded && s.Status == session.StatusEnded {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// cleanTitle collapses whitespace and trims.
func cleanTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// resolveTitle picks the best display title and reports its source.
// Priority: /rename > AI-generated > first user prompt (or last assistant for subagents).
func resolveTitle(s session.JSONLStats) (string, string) {
	if t := cleanTitle(s.CustomTitle); t != "" {
		return t, "custom"
	}
	if t := cleanTitle(s.AiTitle); t != "" {
		return t, "ai"
	}
	if s.IsSubagent {
		if t := cleanTitle(truncate(s.LastAssistant, 120)); t != "" {
			return t, "last_assistant"
		}
	}
	if t := cleanTitle(s.FirstPrompt); t != "" {
		return t, "prompt"
	}
	return "", ""
}

// truncate returns the first n runes of s, trimming at a word boundary when possible.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	cut := string(runes[:n])
	// Walk back to last space for a cleaner break.
	if idx := strings.LastIndex(cut, " "); idx > n/2 {
		cut = cut[:idx]
	}
	return cut
}
