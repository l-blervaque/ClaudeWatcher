# ClaudeWatcher

A TUI to monitor active Claude Code sessions, their linked project, and current status.

## Status

Early PoC — work in progress.

## Features (MVP)

- List all Claude Code sessions discovered from `~/.claude/projects/`
- Show: project name, short session ID, status, last activity, message count
- Status detection:
  - `● active` — `claude` process alive + jsonl modified < 5s ago
  - `◐ waiting` — process alive, last message from assistant, idle briefly
  - `○ idle` — process alive but inactive > 5min
  - `✓ ended` — no associated process
- Auto-refresh every 2s
- Keyboard navigation (`j/k`, `enter`, `r`, `q`)
- **Narrow-layout friendly** — designed to work in a slim terminal column

## Stack

- Go
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling

## Build

```bash
go build -o cw ./cmd/cw
./cw
```

## Roadmap

- [ ] Session detail view (recent messages, todos)
- [ ] Attach/resume session
- [ ] Filter & search
- [ ] Stats summary
