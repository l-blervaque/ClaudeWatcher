package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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
	ID            string
	ProjectDir    string
	ProjectPath   string
	ProjectName   string
	Title         string // resolved display name
	TitleSource   string // "custom" | "ai" | "prompt" | ""
	CustomTitle   string
	AiTitle       string
	FirstPrompt   string
	JSONLPath     string
	LastModified  time.Time
	MessageCount  int
	LastRole      string
	ContextTokens int
	Model         string // last assistant message model id (e.g. "claude-opus-4-7")
	LastAssistant string
	Status        Status
	HasProcess    bool
}

// Default context window when the model is unknown.
const ContextWindow = 200_000

// ContextWindowFor returns the context window size in tokens for a given
// Claude model id. Opus 4.5+ and Sonnet 4.5+ support 1M tokens; older models
// and Haiku stay at 200K.
func ContextWindowFor(model string) int {
	family, major, minor, ok := parseClaudeVersion(model)
	if !ok {
		return ContextWindow
	}
	switch family {
	case "opus", "sonnet":
		if major > 4 || (major == 4 && minor >= 5) {
			return 1_000_000
		}
	}
	return ContextWindow
}

// parseClaudeVersion extracts family + version from ids like
// "claude-opus-4-7" or "claude-sonnet-4-6-20251001".
func parseClaudeVersion(id string) (family string, major, minor int, ok bool) {
	if !strings.HasPrefix(id, "claude-") {
		return "", 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(id, "claude-"), "-")
	if len(parts) < 3 {
		return "", 0, 0, false
	}
	maj, err1 := strconv.Atoi(parts[1])
	min, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil {
		return "", 0, 0, false
	}
	return parts[0], maj, min, true
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

// jsonlLine is a minimal struct to extract everything we care about.
type jsonlLine struct {
	Type        string `json:"type"`
	Cwd         string `json:"cwd"`
	CustomTitle string `json:"customTitle"`
	AiTitle     string `json:"aiTitle"`
	Message     struct {
		Role    string          `json:"role"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   struct {
			InputTokens              int `json:"input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			OutputTokens             int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// JSONLStats holds the extracted metadata from a session jsonl file.
type JSONLStats struct {
	MessageCount    int
	LastRole        string
	Cwd             string // real working directory
	FirstPrompt     string // first user message with string content
	CustomTitle     string // last /rename value
	AiTitle         string // last auto-generated title
	ContextTokens   int    // last known total tokens in context (input + cache_read + cache_creation)
	Model           string // last assistant message model id
	LastAssistant   string // text of the last assistant message
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
		// Titles: last one wins (user can /rename multiple times).
		if l.Type == "custom-title" && l.CustomTitle != "" {
			stats.CustomTitle = l.CustomTitle
		}
		if l.Type == "ai-title" && l.AiTitle != "" {
			stats.AiTitle = l.AiTitle
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
			// Track last assistant context size + text.
			if l.Type == "assistant" {
				u := l.Message.Usage
				if total := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens; total > 0 {
					stats.ContextTokens = total
				}
				if l.Message.Model != "" {
					stats.Model = l.Message.Model
				}
				if text := extractAssistantText(l.Message.Content); text != "" {
					stats.LastAssistant = text
				}
			}
		}
	}
	return stats, scanner.Err()
}

// extractAssistantText pulls the first text block from an assistant
// message content array. Returns "" if content is not a list or has no text.
func extractAssistantText(raw json.RawMessage) string {
	if len(raw) == 0 || raw[0] != '[' {
		return ""
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			return b.Text
		}
	}
	return ""
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
