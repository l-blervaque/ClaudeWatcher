# ClaudeWatcher

A TUI to monitor live Claude Code sessions — at a glance, see every open
session across your projects with its title, status, context usage, and
last activity. Designed to fit in a narrow terminal column.

## Status

Early but usable. macOS only for now (uses `pgrep` + `lsof` to find
running `claude` CLI processes).

## What it shows

By default, only **currently-open sessions** (i.e. with a live `claude`
CLI process). Press `a` to also show ended ones.

For each session:

- **Title** — uses `/rename` value if set, else Claude's auto-generated
  `ai-title`, else the first user prompt
- **Status** — `● active` / `◐ waiting` / `○ idle` / `✓ ended`
- **Project** — the working directory's basename
- **Context %** — based on the last assistant message's token usage,
  against a 200K context window
- **Messages** — total user + assistant message count
- **Last activity** — relative time (`30s`, `2m`, `1h`, …)

Press `enter` on a session to see the detail view: full title breakdown
(custom / ai / prompt), exact context tokens, jsonl path, and a preview
of the last assistant message.

## Install

### Prerequisites

- macOS (Linux likely works but is untested — `pgrep -x` and `lsof` flags
  may need tweaking)
- Go 1.22+

### From source

```bash
git clone https://github.com/l-blervaque/ClaudeWatcher.git
cd ClaudeWatcher
go build -o cw ./cmd/cw
```

Then either run it from the repo:

```bash
./cw
```

Or install it on your `$PATH`:

```bash
go install ./cmd/cw
# binary lands in $(go env GOPATH)/bin/cw — make sure that's on your PATH
```

## Keys

| Key       | Action                                  |
|-----------|-----------------------------------------|
| `j` / `↓` | Move cursor down                        |
| `k` / `↑` | Move cursor up                          |
| `enter`   | Toggle detail view for selected session |
| `esc`     | Leave detail view                       |
| `a`       | Toggle "open only" / "all sessions"     |
| `r`       | Refresh now                             |
| `q`       | Quit                                    |

The list also auto-refreshes every 2 seconds.

## How it works

1. Lists all `claude` CLI processes with `pgrep -x claude`
2. For each, reads its `cwd` with `lsof -d cwd`
3. Walks `~/.claude/projects/*/` and reads each `.jsonl` to extract:
   - Real `cwd` (more reliable than decoding the encoded folder name —
     `-` is ambiguous between a path separator and a literal dash)
   - `custom-title` (set via `/rename`) and `ai-title`
   - First user prompt
   - Token usage from the last assistant message
   - Last assistant message text (for the detail preview)
4. For each project with N running processes, the N most recently
   modified `.jsonl` files are marked as "open"

## Stack

- Go
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling

## Roadmap

- [ ] Attach / resume a session
- [ ] Filter & search
- [ ] Sticky header when list overflows
- [ ] Linux support
- [ ] Model-aware context window (currently hardcoded to 200K)
