package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
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

// runningProjectDirs returns the set of project paths (decoded) currently
// associated with a live `claude` process. We use `ps` and grep for cwd-like
// args; this is a coarse heuristic.
//
// Strategy: list `claude` processes and for each, read its cwd via lsof.
// Falls back to an empty set if anything fails.
func runningProjectDirs() map[string]bool {
	out := map[string]bool{}

	psOut, err := exec.Command("pgrep", "-f", "claude").Output()
	if err != nil {
		return out
	}
	pids := strings.Fields(string(psOut))
	for _, pid := range pids {
		// lsof -a -p PID -d cwd -Fn  → outputs lines like "n/path/to/cwd"
		lo, err := exec.Command("lsof", "-a", "-p", pid, "-d", "cwd", "-Fn").Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(lo), "\n") {
			if strings.HasPrefix(line, "n/") {
				out[strings.TrimPrefix(line, "n")] = true
			}
		}
	}
	return out
}

// Scan walks ~/.claude/projects and returns one Session per .jsonl file.
func Scan() ([]session.Session, error) {
	root, err := projectsRoot()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	activeCwds := runningProjectDirs()
	now := time.Now()

	var sessions []session.Session
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
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			jsonlPath := filepath.Join(fullDir, f.Name())
			info, err := f.Info()
			if err != nil {
				continue
			}
			stats, _ := session.ScanJSONL(jsonlPath)
			id := strings.TrimSuffix(f.Name(), ".jsonl")

			// Prefer the cwd recorded in the jsonl — it's authoritative.
			// Decoding the folder name is ambiguous because "-" in the
			// original path is indistinguishable from a "/" separator.
			path := projectPath
			name := projectName
			if stats.Cwd != "" {
				path = stats.Cwd
				name = filepath.Base(stats.Cwd)
			}
			hasProc := activeCwds[path]
			status := session.DetermineStatus(hasProc, info.ModTime(), stats.LastRole, now)

			sessions = append(sessions, session.Session{
				ID:           id,
				ProjectDir:   projectDir,
				ProjectPath:  path,
				ProjectName:  name,
				JSONLPath:    jsonlPath,
				LastModified: info.ModTime(),
				MessageCount: stats.MessageCount,
				LastRole:     stats.LastRole,
				Status:       status,
				HasProcess:   hasProc,
			})
		}
	}

	return sessions, nil
}
