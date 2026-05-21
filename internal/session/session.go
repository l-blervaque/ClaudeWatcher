package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Status int

const (
	StatusEnded Status = iota
	StatusIdle
	StatusWaiting
	StatusActive
)

func (s Status) Icon() string {
	switch s {
	case StatusActive:
		return "●"
	case StatusWaiting:
		return "◐"
	case StatusIdle:
		return "○"
	default:
		return "✓"
	}
}

func (s Status) Label() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusWaiting:
		return "waiting"
	case StatusIdle:
		return "idle"
	default:
		return "ended"
	}
}

type Session struct {
	ID           string
	ProjectDir   string // encoded directory name
	ProjectPath  string // decoded absolute path
	ProjectName  string // last segment, human-readable
	Title        string // first user prompt, used as session name
	JSONLPath    string
	LastModified time.Time
	MessageCount int
	LastRole     string // "user" or "assistant"
	Status       Status
	HasProcess   bool
}

// DecodeProjectDir converts "-Users-ludo-foo-bar" into "/Users/ludo/foo/bar".
// Claude Code encodes the cwd by replacing "/" with "-".
func DecodeProjectDir(name string) string {
	if !strings.HasPrefix(name, "-") {
		return name
	}
	return "/" + strings.ReplaceAll(strings.TrimPrefix(name, "-"), "-", "/")
}

func ProjectNameFromDir(name string) string {
	decoded := DecodeProjectDir(name)
	return filepath.Base(decoded)
}

// jsonlLine is a minimal struct to extract role, cwd, and content.
// Content can be a string (real user prompt) or a list (tool results,
// images, etc.) — we use json.RawMessage to inspect both shapes.
type jsonlLine struct {
	Type    string `json:"type"`
	Cwd     string `json:"cwd"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// JSONLStats holds the extracted metadata from a session jsonl file.
type JSONLStats struct {
	MessageCount int
	LastRole     string
	Cwd          string // real working directory, when present
	FirstPrompt  string // first user message with string content
}

// ScanJSONL reads a session jsonl file and extracts stats.
func ScanJSONL(path string) (JSONLStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return JSONLStats{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var stats JSONLStats
	for scanner.Scan() {
		var l jsonlLine
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			continue
		}
		if l.Cwd != "" && stats.Cwd == "" {
			stats.Cwd = l.Cwd
		}
		if l.Type == "user" || l.Type == "assistant" {
			stats.MessageCount++
			if l.Message.Role != "" {
				stats.LastRole = l.Message.Role
			} else {
				stats.LastRole = l.Type
			}
			// First real user prompt = first user message whose content
			// is a JSON string (lists are tool_result / multimodal).
			if l.Type == "user" && stats.FirstPrompt == "" && len(l.Message.Content) > 0 && l.Message.Content[0] == '"' {
				var s string
				if json.Unmarshal(l.Message.Content, &s) == nil {
					stats.FirstPrompt = s
				}
			}
		}
	}
	return stats, scanner.Err()
}

// DetermineStatus computes a session's status given:
//   - hasProcess: is there a `claude` process associated (heuristic)
//   - lastMod:    jsonl mtime
//   - lastRole:   last message role
//   - now:        current time
func DetermineStatus(hasProcess bool, lastMod time.Time, lastRole string, now time.Time) Status {
	age := now.Sub(lastMod)
	if !hasProcess {
		return StatusEnded
	}
	if age < 5*time.Second {
		return StatusActive
	}
	if age > 5*time.Minute {
		return StatusIdle
	}
	if lastRole == "assistant" {
		return StatusWaiting
	}
	return StatusActive
}
