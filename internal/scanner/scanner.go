package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
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

// runningClaudeCwds returns the cwd of each live `claude` CLI process,
// as a slice (one entry per process — duplicates kept so callers can
// count how many sessions are open per project).
func runningClaudeCwds() []string {
	// `pgrep -x claude` matches the exact executable name, avoiding
	// the Claude.app desktop helper processes.
	out, err := exec.Command("pgrep", "-x", "claude").Output()
	if err != nil {
		return nil
	}
	pids := strings.Fields(string(out))

	var cwds []string
	for _, pid := range pids {
		lo, err := exec.Command("lsof", "-a", "-p", pid, "-d", "cwd", "-Fn").Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(lo), "\n") {
			if strings.HasPrefix(line, "n/") {
				cwds = append(cwds, strings.TrimPrefix(line, "n"))
			}
		}
	}
	return cwds
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

	procCount := map[string]int{}
	for _, cwd := range runningClaudeCwds() {
		procCount[cwd]++
	}
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
						AwaySummaryCount:     stats.AwaySummaryCount,
						ApiErrorCount:        stats.ApiErrorCount,
						TurnCount:            stats.TurnCount,
						ApiErrorRate:         stats.ApiErrorRate,
						QueueDepth:           stats.QueueDepth,
						CompactBoundaryCount: stats.CompactBoundaryCount,
						ActiveSkill:          stats.ActiveSkill,
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
				AwaySummaryCount:     stats.AwaySummaryCount,
				ApiErrorCount:        stats.ApiErrorCount,
				TurnCount:            stats.TurnCount,
				ApiErrorRate:         stats.ApiErrorRate,
				QueueDepth:           stats.QueueDepth,
				CompactBoundaryCount: stats.CompactBoundaryCount,
				ActiveSkill:          stats.ActiveSkill,
			})
		}
	}

	// Group by project, mark the top-N most-recent MAIN session jsonl files as
	// having a process, where N = number of running claude processes in that cwd.
	// Subagent sessions are excluded from this competition: they inherit
	// HasProcess from their parent main session in the second pass below.
	byPath := map[string][]int{}
	for i, s := range all {
		if s.ParentID != "" {
			continue // subagents handled separately
		}
		byPath[s.ProjectPath] = append(byPath[s.ProjectPath], i)
	}
	// Build a set of main session IDs that have a process, for the subagent pass.
	mainHasProcess := map[string]bool{}
	for path, idxs := range byPath {
		n := procCount[path]
		if n == 0 {
			continue
		}
		sort.Slice(idxs, func(a, b int) bool {
			return all[idxs[a]].LastModified.After(all[idxs[b]].LastModified)
		})
		if n > len(idxs) {
			n = len(idxs)
		}
		for _, i := range idxs[:n] {
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
