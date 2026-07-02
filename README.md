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

- **Status** — `● active` / `◐ waiting` / `○ idle` / `✓ ended`
- **Project** — the working directory's basename
- **Title** — uses `/rename` value if set, else Claude's auto-generated
  `ai-title`, else the first user prompt
- **Context %** — based on the last assistant message's token usage,
  against a 200K context window (color-coded: green / yellow / red)
- **Cache %** — read cache hit rate for the session
- **Messages** — total user + assistant message count
- **Last activity** — relative time (`30s`, `2m`, `1h`, …)
- **Badges** — `[P]` principal · `[S]` subagent · `[MULTI]` compacted ·
  `[ERR]` API errors · `[Q:N]` pending queue

Subagent sessions are grouped under their parent with a tree indent (`└`).

Press `enter` on a session to see the detail view: full title breakdown
(custom / ai / prompt), exact context tokens, jsonl path, and a preview
of the last assistant message.

## Install

### Prerequisites

- macOS (Linux likely works but is untested — `pgrep -x` and `lsof` flags
  may need tweaking)
- Go 1.22+

### Nerd Font (optional but recommended)

Status icons and badges look best with a Nerd Font. Without one, enable
the plain-text fallback in **Options → Display → Nerd Fonts** (off by
default).

Install via Homebrew:

```bash
# Pick any Nerd Font — JetBrains Mono is a solid default
brew install --cask font-jetbrains-mono-nerd-font

# Other popular choices
brew install --cask font-fira-code-nerd-font
brew install --cask font-meslo-lg-nerd-font
```

Then set the installed font in your terminal emulator, and enable
**Nerd Fonts** in ClaudeWatcher's Options tab.

Alternatively, download any font from [nerdfonts.com](https://www.nerdfonts.com/font-downloads),
unzip, and double-click the `.ttf` / `.otf` files to install via Font Book.

### From source

```bash
git clone https://github.com/l-blervaque/ClaudeWatcher.git
cd ClaudeWatcher
go build -ldflags "-X github.com/ludo/claudewatcher/internal/version.Commit=$(git rev-parse --short HEAD)" -o cw ./cmd/cw
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

### Sessions tab

| Key           | Action                                  |
|---------------|-----------------------------------------|
| `j` / `↓`     | Move cursor down                        |
| `k` / `↑`     | Move cursor up                          |
| `enter`       | Toggle detail view for selected session |
| `esc`         | Leave detail view                       |
| `a`           | Toggle "open only" / "all sessions"     |
| `r`           | Refresh now                             |
| `o`           | Open Options tab                        |
| `tab`         | Next tab (Sessions → Options → Shortcuts) |
| `shift+tab`   | Previous tab                            |
| `ctrl+q`      | Quit                                    |

### Options tab

| Key             | Action                          |
|-----------------|---------------------------------|
| `j` / `↓`       | Move cursor down                |
| `k` / `↑`       | Move cursor up                  |
| `space` / `enter` | Toggle selected option        |
| `tab`           | Go to Shortcuts tab             |
| `esc`           | Back to Sessions                |

The list also auto-refreshes every 2 seconds.

## Options

Open with `o` or the `tab` key. Available toggles:

- **Sounds** — enable / disable notification sounds, choose sound
- **Columns** — show/hide Cache, Ctx, Msgs, Age, Badges columns individually
- **Display → Nerd Fonts** — use Nerd Font glyphs for status icons (requires
  a Nerd Font installed and set in your terminal)

## How it works

1. Lists all `claude` CLI processes with `pgrep -x claude`, then reads each
   one's command line (`ps`) to drop non-sessions — the background `daemon`
   and headless `claude -p` / `--print` runs — which would otherwise inflate
   the per-folder count.
2. For each remaining process, reads its `cwd` with `lsof -d cwd` and tries to
   recover the exact session id it is running: from a `--resume <uuid>`
   argument (terminal / cmux / tmux) or a desktop-app PTY-host socket path.
3. Walks `~/.claude/projects/*/` and reads each `.jsonl` to extract:
   - Real `cwd` (more reliable than decoding the encoded folder name —
     `-` is ambiguous between a path separator and a literal dash)
   - `custom-title` (set via `/rename`) and `ai-title`
   - First user prompt
   - Token usage and model from the last assistant message
   - Last assistant message text (for the detail preview)
   - API error count, queue depth, subagent relationship
4. Marks sessions "open" in two stages: first an **exact match** for any
   process whose session id was recovered (so a just-exited session can't keep
   its badge by borrowing a sibling's process), then a **recency fallback** for
   id-less processes — the N most recently modified main sessions in that cwd
   that weren't already claimed.
5. Subagent sessions are grouped under their parent and inherit the
   parent's "open" status if recently modified

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
