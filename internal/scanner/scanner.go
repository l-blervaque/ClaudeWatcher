package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	// pid is the OS process id. Carried for the read-only diagnostic so an
	// attribution audit can name the exact process.
	pid int
	// cwd is the process working directory (used for the recency fallback
	// when the exact session can't be recovered).
	cwd string
	// sessionID is the full session UUID recovered from a `--session-id
	// <uuid>` argument (cmux and skills pre-assign the id). Claude Code uses
	// exactly this id for the transcript, so it is authoritative.
	sessionID string
	// resumeID is the full UUID recovered from a `--resume <uuid>` argument.
	// Claude Code FORKS the resumed conversation into a NEW session file with
	// a new UUID, so this id usually names a dead transcript. It is a hint:
	// only trusted while that transcript is still fresh (see attribute).
	resumeID string
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

// sessionIDsFromCmdline recovers session identity from a `claude` command
// line. `--session-id <uuid>` names the transcript directly (authoritative);
// `--resume <uuid>` names the transcript being resumed, which Claude Code
// forks into a NEW session file — so it is only a hint. Both the space and
// `=` forms are accepted. Pure function over the command line so it is
// unit-testable without spawning procs.
func sessionIDsFromCmdline(cmdline string) (sessionID, resumeID string) {
	fields := strings.Fields(cmdline)
	get := func(flag string, i int) (string, bool) {
		f := fields[i]
		if v, ok := strings.CutPrefix(f, flag+"="); ok && uuidRE.MatchString(v) {
			return v, true
		}
		if f == flag && i+1 < len(fields) && uuidRE.MatchString(fields[i+1]) {
			return fields[i+1], true
		}
		return "", false
	}
	for i := range fields {
		if v, ok := get("--session-id", i); ok {
			sessionID = v
		}
		if v, ok := get("--resume", i); ok {
			resumeID = v
		}
	}
	return sessionID, resumeID
}

// isClaudeExe reports whether a command line's executable (argv[0]) is the
// `claude` CLI, matching by basename so it catches both `claude …` and
// `/path/to/claude …`. `pgrep -x claude` matches the process accounting name
// (comm), which is the bare basename for some launches and the full executable
// path for others — that inconsistency let it silently miss live sessions
// (observed: a session whose comm was the full path), which then showed as
// ghosts. Matching the basename of the real argv[0] is stable across launches.
func isClaudeExe(cmdline string) bool {
	fields := strings.Fields(cmdline)
	if len(fields) == 0 {
		return false
	}
	return filepath.Base(fields[0]) == "claude"
}

// runningClaudeProcs returns the live `claude` processes that represent real
// sessions. We enumerate every process in a single `ps -Aww` read and keep
// those whose executable basename is `claude`. This avoids depending on
// `pgrep -x claude`, whose name (comm) matching is inconsistent across launches
// and missed real sessions. The background daemon and headless `claude -p` runs
// are not sessions and are dropped by isSessionProc. For exact attribution we
// recover the session id from a `--resume`/`--session-id <uuid>` argument, or a
// desktop PTY-host socket path.
func runningClaudeProcs() []claudeProc {
	// One `ps -Aww` lists every process with its full (untruncated) command
	// line, so a session id near the end of a long line isn't cut off. We then
	// keep only the claude session processes.
	psOut, err := exec.Command("ps", "-Aww", "-o", "pid=,command=").Output()
	if err != nil {
		return nil
	}
	cmdByPid := map[string]string{}
	var pids []string
	for _, line := range strings.Split(string(psOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		pid, cmdline := parts[0], strings.TrimSpace(parts[1])
		if !isClaudeExe(cmdline) || !isSessionProc(cmdline) {
			continue
		}
		cmdByPid[pid] = cmdline
		pids = append(pids, pid)
	}
	if len(pids) == 0 {
		return nil
	}

	// cwd per pid: `-Fpn` emits a `p<pid>` line followed by the `n<path>` cwd
	// line for that process, so we can key each cwd to its pid. Batched into one
	// lsof call (heavy per-invocation startup cost). Ignore exit status: it
	// exits non-zero when any listed pid has already exited but still prints the
	// survivors.
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
		cmdline := cmdByPid[pid]
		var prefix string
		if m := sockRE.FindStringSubmatch(cmdline); m != nil {
			prefix = m[1]
		}
		sessionID, resumeID := sessionIDsFromCmdline(cmdline)
		pidNum, _ := strconv.Atoi(pid)
		procs = append(procs, claudeProc{
			pid:           pidNum,
			cwd:           cwdByPid[pid],
			sessionID:     sessionID,
			resumeID:      resumeID,
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

	attribute(all, procs, now)

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

// attribute marks all[i].HasProcess for the MAIN sessions that have a live
// `claude` process, then propagates the flag to recent subagents. It is pure
// over its inputs (no I/O) so attribution can be tested with synthetic data.
//
// Two-stage attribution:
//  1. Exact match — a process that names its own session (a `--resume`/
//     `--session-id <uuid>`, or a desktop PTY-host socket prefix) marks exactly
//     that session, so an unrelated transcript in the same folder (e.g. one
//     just touched by `/exit`, or a more-recent dead transcript) can't steal
//     it. The full UUID is globally unique, so it matches regardless of cwd.
//  2. Recency fallback — processes with no recoverable session id (a freshly
//     started session) are counted per cwd; the top-N most recently modified
//     unclaimed sessions in that cwd are marked. This is best-effort: with no
//     identity it can mark a more-recent dead transcript over a quiet live one.
func attribute(all []session.Session, procs []claudeProc, now time.Time) {
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

	// Subagents inherit HasProcess from their parent main session only if their
	// jsonl was modified recently (same 5-minute threshold as DetermineStatus).
	// Stale/finished subagents must never inherit the flag just because their
	// parent process is still alive.
	for i, s := range all {
		if s.ParentID != "" && mainHasProcess[s.ParentID] && now.Sub(s.LastModified) < 5*time.Minute {
			all[i].HasProcess = true
		}
	}
}

// ProcDiag is one row of the read-only attribution diagnostic.
type ProcDiag struct {
	PID    int
	UUID   string // session id recovered from the cmdline, "" if none
	Cwd    string
	Status string // matched | no-uuid (recency) | transcript-found-not-marked | UNMATCHED (no transcript)
}

// Diagnose reports, for every live claude session process, how the scanner
// attributed it. It runs the same detection + scan as the TUI and is read-only
// (no process is touched), so an attribution regression can be audited directly
// instead of looking like a UI problem.
func Diagnose() ([]ProcDiag, error) {
	procs := runningClaudeProcs()
	ss, err := Scan(ScanOptions{IncludeEnded: true})
	if err != nil {
		return nil, err
	}
	exists := map[string]bool{}
	marked := map[string]bool{}
	for _, s := range ss {
		if s.ParentID != "" {
			continue
		}
		exists[s.ID] = true
		if s.HasProcess {
			marked[s.ID] = true
		}
	}
	out := make([]ProcDiag, 0, len(procs))
	for _, p := range procs {
		d := ProcDiag{PID: p.pid, UUID: p.sessionID, Cwd: p.cwd}
		switch {
		case p.sessionID == "":
			d.Status = "no-uuid (recency)"
		case marked[p.sessionID]:
			d.Status = "matched"
		case exists[p.sessionID]:
			d.Status = "transcript-found-not-marked"
		default:
			d.Status = "UNMATCHED (no transcript)"
		}
		out = append(out, d)
	}
	return out, nil
}

// FormatDiagnosis renders diagnostic rows as an aligned text table. Pure, so it
// is unit-testable independently of live process enumeration.
func FormatDiagnosis(diags []ProcDiag) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-7s %-38s %-28s %s\n", "PID", "UUID", "STATUS", "CWD")
	for _, d := range diags {
		uuid := d.UUID
		if uuid == "" {
			uuid = "-"
		}
		fmt.Fprintf(&b, "%-7d %-38s %-28s %s\n", d.PID, uuid, d.Status, d.Cwd)
	}
	fmt.Fprintf(&b, "\n%d process\n", len(diags))
	return b.String()
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
