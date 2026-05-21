package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ludo/claudewatcher/internal/scanner"
	"github.com/ludo/claudewatcher/internal/session"
)

const refreshInterval = 2 * time.Second

// ---- styles ----

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#888")).
			Underline(true)

	// Subtle left-edge marker instead of full-row highlight (was too
	// loud and made the selected line hard to read).
	cursorBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	cursorBar      = cursorBarStyle.Render("▌")
	unselectedBar  = " "

	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666"))

	statusStyles = map[session.Status]lipgloss.Style{
		session.StatusActive:  lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50")),
		session.StatusWaiting: lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC107")),
		session.StatusIdle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#2196F3")),
		session.StatusEnded:   lipgloss.NewStyle().Foreground(lipgloss.Color("#666")),
	}
)

// ---- messages ----

type tickMsg time.Time
type sessionsMsg struct {
	sessions []session.Session
	err      error
}

// ---- model ----

type Model struct {
	sessions     []session.Session
	cursor       int
	width        int
	height       int
	err          error
	detail       bool
	includeEnded bool // toggle with 'a'
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadSessions(m.includeEnded), tick())
}

func loadSessions(includeEnded bool) tea.Cmd {
	return func() tea.Msg {
		s, err := scanner.Scan(scanner.ScanOptions{IncludeEnded: includeEnded})
		return sessionsMsg{sessions: s, err: err}
	}
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "r":
			return m, loadSessions(m.includeEnded)
		case "a":
			m.includeEnded = !m.includeEnded
			return m, loadSessions(m.includeEnded)
		case "enter":
			m.detail = !m.detail
		case "esc":
			m.detail = false
		}
	case tickMsg:
		return m, tea.Batch(loadSessions(m.includeEnded), tick())
	case sessionsMsg:
		m.err = msg.err
		m.sessions = sortSessions(msg.sessions)
		if m.cursor >= len(m.sessions) {
			m.cursor = len(m.sessions) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
	return m, nil
}

// sortSessions: active > waiting > idle > ended, then by recency
func sortSessions(s []session.Session) []session.Session {
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].Status != s[j].Status {
			return s[i].Status > s[j].Status
		}
		return s[i].LastModified.After(s[j].LastModified)
	})
	return s
}

// ---- render ----

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}
	if m.width == 0 {
		return "Loading..."
	}
	if m.detail && len(m.sessions) > 0 {
		return m.renderDetail()
	}
	return m.renderList()
}

func (m Model) renderList() string {
	var b strings.Builder

	header := titleStyle.Render("ClaudeWatcher")
	b.WriteString(header)
	b.WriteString("  ")
	mode := "open"
	if m.includeEnded {
		mode = "all"
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("%d sessions · %s", len(m.sessions), mode)))
	b.WriteString("\n\n")

	narrow := m.width < 80

	if narrow {
		b.WriteString(m.renderListNarrow())
	} else {
		b.WriteString(m.renderListWide())
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("j/k nav · enter detail · a all/open · r refresh · q quit"))
	return b.String()
}

// renderListNarrow: three lines per session.
//   ● Session title ················· active
//     customer-biogen
//     ctx 46% · 42 msgs · 2m
func (m Model) renderListNarrow() string {
	var b strings.Builder

	for i, s := range m.sessions {
		st := statusStyles[s.Status]
		icon := st.Render(s.Status.Icon())

		title := s.Title
		if title == "" {
			title = "(no title)"
		}
		status := s.Status.Label()

		// line 1: icon + title ··· status, with dotted leader
		// budget: width - 2 (icon+space) - len(status) - 1 (space before status)
		leftBudget := m.width - 2 - len(status) - 1
		if leftBudget < 8 {
			leftBudget = 8
		}
		title = truncate(title, leftBudget-2) // room for at least 2 dots
		fillN := leftBudget - len(title) - 1   // 1 = space after title
		if fillN < 1 {
			fillN = 1
		}
		line1 := fmt.Sprintf("%s %s %s %s",
			icon,
			title,
			dimStyle.Render(strings.Repeat("·", fillN)),
			st.Render(status))

		line2 := fmt.Sprintf("  %s", truncate(s.ProjectName, m.width-2))
		line3 := fmt.Sprintf("  ctx %s · %d msgs · %s",
			contextPct(s.ContextTokens), s.MessageCount, humanizeAgo(s.LastModified))

		bar := unselectedBar
		if i == m.cursor {
			bar = cursorBar
		}
		b.WriteString(bar)
		b.WriteString(line1)
		b.WriteString("\n")
		b.WriteString(bar)
		b.WriteString(line2)
		b.WriteString("\n")
		b.WriteString(bar)
		if i == m.cursor {
			b.WriteString(line3)
		} else {
			b.WriteString(dimStyle.Render(line3))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderListWide: one row per session.
func (m Model) renderListWide() string {
	var b strings.Builder

	// columns: status(2) project(20) title(flex) ctx(5) msgs(5) ago(8)
	projW, ctxW, msgW, agoW := 20, 5, 5, 8
	titleW := m.width - 2 - projW - ctxW - msgW - agoW - 5
	if titleW < 10 {
		titleW = 10
	}

	header := fmt.Sprintf("  %-*s %-*s %*s %*s %*s",
		projW, "PROJECT", titleW, "TITLE", ctxW, "CTX", msgW, "MSGS", agoW, "AGE")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, s := range m.sessions {
		st := statusStyles[s.Status]
		icon := st.Render(s.Status.Icon())
		title := s.Title
		if title == "" {
			title = "—"
		}
		row := fmt.Sprintf("%s %-*s %-*s %*s %*d %*s",
			icon,
			projW, truncate(s.ProjectName, projW),
			titleW, truncate(title, titleW),
			ctxW, contextPct(s.ContextTokens),
			msgW, s.MessageCount,
			agoW, humanizeAgo(s.LastModified))

		bar := unselectedBar
		if i == m.cursor {
			bar = cursorBar
		}
		b.WriteString(bar)
		b.WriteString(row)
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderDetail() string {
	s := m.sessions[m.cursor]
	st := statusStyles[s.Status]

	var b strings.Builder
	b.WriteString(titleStyle.Render("Session detail"))
	b.WriteString("\n\n")

	// Project + status
	b.WriteString(fmt.Sprintf("Project:  %s\n", s.ProjectName))
	b.WriteString(dimStyle.Render(fmt.Sprintf("          %s\n", s.ProjectPath)))
	b.WriteString(fmt.Sprintf("Status:   %s %s\n", st.Render(s.Status.Icon()), st.Render(s.Status.Label())))
	b.WriteString(fmt.Sprintf("Session:  %s\n", s.ID))

	// Titles — show source breakdown so user sees /rename vs ai-title vs prompt
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Titles"))
	b.WriteString("\n")
	if s.CustomTitle != "" {
		b.WriteString(fmt.Sprintf("  /rename:  %s\n", s.CustomTitle))
	}
	if s.AiTitle != "" {
		b.WriteString(fmt.Sprintf("  ai-title: %s\n", s.AiTitle))
	}
	if s.FirstPrompt != "" {
		b.WriteString(fmt.Sprintf("  prompt:   %s\n", truncate(s.FirstPrompt, m.width-12)))
	}

	// Stats
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Stats"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Context:  %s (%d / %d tokens)\n",
		contextPct(s.ContextTokens), s.ContextTokens, session.ContextWindow))
	b.WriteString(fmt.Sprintf("  Messages: %d\n", s.MessageCount))
	b.WriteString(fmt.Sprintf("  Last:     %s (%s)\n",
		humanizeAgo(s.LastModified), s.LastModified.Format("2006-01-02 15:04:05")))

	// Last assistant message preview
	if s.LastAssistant != "" {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Last message"))
		b.WriteString("\n")
		b.WriteString(wrapPreview(s.LastAssistant, m.width-2, 8))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("jsonl: %s\n", s.JSONLPath)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("enter/esc back · q quit"))
	return b.String()
}

// ---- helpers ----

func humanizeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func shortID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

// contextPct returns the context usage as "46%" (or "—" if unknown).
func contextPct(tokens int) string {
	if tokens <= 0 {
		return "—"
	}
	pct := tokens * 100 / session.ContextWindow
	return fmt.Sprintf("%d%%", pct)
}

// wrapPreview wraps text to width, capped at maxLines, with "  " prefix.
func wrapPreview(s string, width, maxLines int) string {
	s = strings.Join(strings.Fields(s), " ")
	if width < 10 {
		width = 10
	}
	var out []string
	for len(s) > 0 && len(out) < maxLines {
		if len(s) <= width-2 {
			out = append(out, "  "+s)
			break
		}
		out = append(out, "  "+s[:width-2])
		s = s[width-2:]
	}
	if len(s) > 0 && len(out) == maxLines {
		out[maxLines-1] = strings.TrimRight(out[maxLines-1], " ") + "…"
	}
	return strings.Join(out, "\n") + "\n"
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	return s[:w-1] + "…"
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

// stripANSI: very rough ANSI escape stripper for length-based padding when
// we want the background highlight to span the whole row.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			in = true
			continue
		}
		if in {
			if c == 'm' {
				in = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
