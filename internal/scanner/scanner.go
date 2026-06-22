package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ludo/claudewatcher/internal/session"
)

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
	// sessionPrefix is the leading hex of the session UUID, recovered from a
	// desktop-app PTY-host socket path (…/pty/<prefix>.sock). Empty when the
	// process carries no session identity (plain terminal, VS Code extension).
	sessionPrefix string
}

// sockRE pulls the session-id prefix out of a PTY-host socket arg such as
// `/tmp/cc-daemon-501/e0d9d869/pty/1fe22414.sock`.
var sockRE = regexp.MustCompile(`/pty/([0-9a-fA-F]+)\.sock`)

// runningClaudeProcs returns the live `claude` processes that represent real
// sessions. `pgrep -x claude` matches the exact executable name, but that name
// is shared by the background daemon and headless `claude -p` runs, which are
// not sessions and would inflate the per-cwd count — those are dropped here.
// Desktop-app PTY-host processes ARE sessions and additionally carry the
// session id in their socket path, so we recover it for exact attribution.
func runningClaudeProcs() []claudeProc {
	out, err := exec.Command("pgrep", "-x", "claude").Output()
	if err != nil {
		return nil
	}
	pids := strings.Fields(string(out))

	var procs []claudeProc
	for _, pid := range pids {
		cmdOut, err := exec.Command("ps", "-o", "command=", "-p", pid).Output()
		if err != nil {
			continue
		}
		cmdline := strings.TrimSpace(string(cmdOut))
		if !isSessionProc(cmdline) {
			continue
		}
		var cwd string
		if lo, err := exec.Command("lsof", "-a", "-p", pid, "-d", "cwd", "-Fn").Output(); err == nil {
			for _, line := range strings.Split(string(lo), "\n") {
				if strings.HasPrefix(line, "n/") {
					cwd = strings.TrimPrefix(line, "n")
					break
				}
			}
		}
		var prefix string
		if m := sockRE.FindStringSubmatch(cmdline); m != nil {
			prefix = m[1]
		}
		procs = append(procs, claudeProc{cwd: cwd, sessionPrefix: prefix})
	}
	return procs
}

// isSessionProc reports whether a `claude` command line represents a real
// session. Only the genuine non-sessions are rejected: the background daemon
// and headless `claude -p` / `--print` runs. Desktop PTY-host helpers, the
// VS Code extension, and plain terminal sessions all pass.
func isSessionProc(cmdline string) bool {
	if strings.Contains(cmdline, "daemon") { // `claude daemon run …`
		return false
	}
	// Headless print mode. Match whole arguments so a path containing "-p"
	// doesn't trip the check.
	for _, field := range strings.Fields(cmdline) {
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
					stats, _ := session.ScanJSONL(jsonlPath)
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
			stats, _ := session.ScanJSONL(jsonlPath)
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

	// Attribute live processes to MAIN sessions. Subagents are excluded from
	// this competition: they inherit HasProcess from their parent main session
	// in the second pass below.
	//
	// Two-stage attribution:
	//  1. Exact match — a desktop PTY-host process names its own session in the
	//     socket path. Mark exactly that session, so an unrelated transcript in
	//     the same folder (e.g. one just touched by `/exit`) can't steal it.
	//  2. Recency fallback — processes with no recoverable session id (plain
	//     terminal, VS Code extension) are counted per cwd; the top-N most
	//     recently modified main sessions in that cwd that weren't already
	//     claimed in stage 1 are marked open.
	byPath := map[string][]int{}
	for i, s := range all {
		if s.ParentID != "" {
			continue // subagents handled separately
		}
		byPath[s.ProjectPath] = append(byPath[s.ProjectPath], i)
	}

	// Stage 1: exact session-id attribution.
	mainHasProcess := map[string]bool{} // session ID -> has process (for subagent pass)
	fallbackCount := map[string]int{}   // cwd -> count of id-less processes
	for _, p := range procs {
		if p.sessionPrefix == "" {
			fallbackCount[p.cwd]++
			continue
		}
		for _, i := range byPath[p.cwd] {
			if strings.HasPrefix(all[i].ID, p.sessionPrefix) {
				all[i].HasProcess = true
				mainHasProcess[all[i].ID] = true
			}
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
