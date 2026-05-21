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

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000")).
			Background(lipgloss.Color("#7D56F4"))

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

// renderListNarrow: three lines per session, fits in slim columns.
//   ● project-name                 2m
//     "first user prompt truncated…"
//     d54dfca0 · 42 msgs · active
func (m Model) renderListNarrow() string {
	var b strings.Builder

	header := dimStyle.Render(fmt.Sprintf("%-*s", m.width, "STATUS  PROJECT / TITLE"))
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, s := range m.sessions {
		st := statusStyles[s.Status]
		icon := st.Render(s.Status.Icon())
		ago := humanizeAgo(s.LastModified)

		// width budget for the project name on the first line
		nameWidth := m.width - 4 /* icon+spaces */ - len(ago) - 1
		if nameWidth < 8 {
			nameWidth = 8
		}
		name := truncate(s.ProjectName, nameWidth)

		title := s.Title
		if title == "" {
			title = "(no prompt yet)"
		}
		title = truncate(title, m.width-4)

		line1 := fmt.Sprintf("%s %-*s %s", icon, nameWidth, name, dimStyle.Render(ago))
		line2 := fmt.Sprintf("  %s", title)
		line3 := fmt.Sprintf("  %s · %d msgs · %s",
			shortID(s.ID), s.MessageCount, st.Render(s.Status.Label()))

		if i == m.cursor {
			b.WriteString(selectedStyle.Render(padRight(stripANSI(line1), m.width)))
			b.WriteString("\n")
			b.WriteString(selectedStyle.Render(padRight(stripANSI(line2), m.width)))
			b.WriteString("\n")
			b.WriteString(selectedStyle.Render(padRight(stripANSI(line3), m.width)))
		} else {
			b.WriteString(line1)
			b.WriteString("\n")
			b.WriteString(dimStyle.Render(line2))
			b.WriteString("\n")
			b.WriteString(dimStyle.Render(line3))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderListWide: one row per session.
func (m Model) renderListWide() string {
	var b strings.Builder

	// columns: status(2) project(20) title(flex) id(10) msgs(5) ago(8)
	projW, idW, msgW, agoW := 20, 10, 5, 8
	titleW := m.width - 2 /* status */ - projW - idW - msgW - agoW - 5 /* spaces */
	if titleW < 10 {
		titleW = 10
	}

	header := fmt.Sprintf("  %-*s %-*s %-*s %*s %*s",
		projW, "PROJECT", titleW, "TITLE", idW, "ID", msgW, "MSGS", agoW, "AGE")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, s := range m.sessions {
		st := statusStyles[s.Status]
		icon := st.Render(s.Status.Icon())
		title := s.Title
		if title == "" {
			title = "—"
		}
		row := fmt.Sprintf("%s %-*s %-*s %-*s %*d %*s",
			icon,
			projW, truncate(s.ProjectName, projW),
			titleW, truncate(title, titleW),
			idW, shortID(s.ID),
			msgW, s.MessageCount,
			agoW, humanizeAgo(s.LastModified))

		if i == m.cursor {
			b.WriteString(selectedStyle.Render(padRight(stripANSI(row), m.width)))
		} else {
			b.WriteString(row)
		}
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
	b.WriteString(fmt.Sprintf("Project:  %s\n", s.ProjectName))
	b.WriteString(dimStyle.Render(fmt.Sprintf("Path:     %s\n", s.ProjectPath)))
	b.WriteString(fmt.Sprintf("Session:  %s\n", s.ID))
	if s.Title != "" {
		b.WriteString(fmt.Sprintf("Title:    %s\n", s.Title))
	}
	b.WriteString(fmt.Sprintf("Status:   %s %s\n", st.Render(s.Status.Icon()), st.Render(s.Status.Label())))
	b.WriteString(fmt.Sprintf("Messages: %d\n", s.MessageCount))
	b.WriteString(fmt.Sprintf("Last:     %s (%s)\n", humanizeAgo(s.LastModified), s.LastModified.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("LastRole: %s\n", s.LastRole))
	b.WriteString(fmt.Sprintf("Process:  %v\n", s.HasProcess))
	b.WriteString(dimStyle.Render(fmt.Sprintf("JSONL:    %s\n", s.JSONLPath)))
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
