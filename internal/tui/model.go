package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ludo/claudewatcher/internal/audio"
	"github.com/ludo/claudewatcher/internal/config"
	"github.com/ludo/claudewatcher/internal/scanner"
	"github.com/ludo/claudewatcher/internal/session"
	"github.com/ludo/claudewatcher/internal/version"
)

const refreshInterval = 2 * time.Second

// Tab indices.
const (
	tabSessions  = 0
	tabOptions   = 1
	tabShortcuts = 2
)

// ---- styles ----

// availableSounds is the ordered list of sound names the user can cycle through.
// It is the single source of truth for both the options panel and the Update handler.
var availableSounds = []string{"glass", "ping", "funk"}

var soundLabels = []string{"Glass", "Ping", "Funk"}

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

	// Cache efficiency color styles.
	cacheGoodStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50"))
	cacheMidStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFC107"))
	cacheBadStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F44336"))

	// Badge styles.
	badgeSubStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
	badgeMultiStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9800"))
	badgeErrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F44336"))
	badgeQueueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2196F3"))

	// Footer box style.
	footerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444")).
			Padding(0, 1)

	// Options section header style.
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#888"))
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
	offset       int // scroll offset for the session list
	width        int
	height       int
	err          error
	detail       bool
	includeEnded bool
	activeTab    int // 0=Sessions 1=Options 2=Raccourcis
	optCursor    int
	cfg          config.Config
	prevStatus   map[string]session.Status
}

func NewModel() Model {
	cfg, _ := config.Load()
	return Model{
		cfg:        cfg,
		prevStatus: make(map[string]session.Status),
	}
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

// optionsMaxCursor is the highest valid optCursor index in the Options tab.
// Sons: 0=toggle, 1=sound selector → 2 items
// Colonnes: 2=ShowCache, 3=ShowCtx, 4=ShowMsgs, 5=ShowAge, 6=ShowBadges → 5 items
// Total indices: 0..6 → 6
const optionsMaxCursor = 6

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.activeTab == tabOptions {
			switch msg.String() {
			case "ctrl+q", "ctrl+c":
				config.Save(m.cfg) //nolint:errcheck
				return m, tea.Quit
			case "j", "down":
				if m.optCursor < optionsMaxCursor {
					m.optCursor++
				}
			case "k", "up":
				if m.optCursor > 0 {
					m.optCursor--
				}
			case " ", "enter":
				switch m.optCursor {
				case 0:
					m.cfg.SoundEnabled = !m.cfg.SoundEnabled
				case 1:
					if m.cfg.SoundEnabled {
						found := false
						for i, s := range availableSounds {
							if s == m.cfg.SoundName {
								m.cfg.SoundName = availableSounds[(i+1)%len(availableSounds)]
								found = true
								break
							}
						}
						if !found {
							m.cfg.SoundName = availableSounds[0]
						}
					}
				case 2:
					m.cfg.ShowCache = !m.cfg.ShowCache
				case 3:
					m.cfg.ShowCtx = !m.cfg.ShowCtx
				case 4:
					m.cfg.ShowMsgs = !m.cfg.ShowMsgs
				case 5:
					m.cfg.ShowAge = !m.cfg.ShowAge
				case 6:
					m.cfg.ShowBadges = !m.cfg.ShowBadges
				}
			case "tab":
				m.activeTab = (m.activeTab + 1) % 3
				config.Save(m.cfg) //nolint:errcheck
			case "shift+tab":
				m.activeTab = (m.activeTab + 2) % 3
				config.Save(m.cfg) //nolint:errcheck
			case "esc":
				m.activeTab = tabSessions
				config.Save(m.cfg) //nolint:errcheck
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % 3
		case "shift+tab":
			m.activeTab = (m.activeTab + 2) % 3
		case "j", "down":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				m.offset = clampOffset(m.cursor, m.offset, m.visibleRows())
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.offset = clampOffset(m.cursor, m.offset, m.visibleRows())
			}
		case "r":
			return m, loadSessions(m.includeEnded)
		case "a":
			m.includeEnded = !m.includeEnded
			m.cursor = 0
			m.offset = 0
			return m, loadSessions(m.includeEnded)
		case "o":
			if !m.detail {
				m.activeTab = tabOptions
			}
		case "enter":
			m.detail = !m.detail
		case "esc":
			m.detail = false
		}
	case tickMsg:
		return m, tea.Batch(loadSessions(m.includeEnded), tick())
	case sessionsMsg:
		m.err = msg.err
		newSessions := sortSessions(msg.sessions)
		if m.cfg.SoundEnabled && detectTransitions(m.prevStatus, newSessions) {
			audio.Play(m.cfg.SoundName)
		}
		m.prevStatus = make(map[string]session.Status, len(newSessions))
		for _, s := range newSessions {
			m.prevStatus[s.ID] = s.Status
		}
		m.sessions = newSessions
		if m.cursor >= len(m.sessions) {
			m.cursor = len(m.sessions) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.offset = clampOffset(m.cursor, m.offset, m.visibleRows())
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
		return fmt.Sprintf("Error: %v\n\nPress ctrl+q to quit.", m.err)
	}
	if m.width == 0 {
		return "Loading..."
	}
	if m.detail && len(m.sessions) > 0 {
		return m.renderDetail()
	}
	switch m.activeTab {
	case tabOptions:
		return m.renderOptions()
	case tabShortcuts:
		return m.renderShortcuts()
	default:
		return m.renderList()
	}
}

// renderTabBar returns the tab navigation bar string.
func (m Model) renderTabBar() string {
	tabs := []string{"Sessions", "Options", "Raccourcis"}
	var parts []string
	for i, t := range tabs {
		if i == m.activeTab {
			parts = append(parts, lipgloss.NewStyle().
				Bold(true).Foreground(lipgloss.Color("#7D56F4")).
				Underline(true).Render(t))
		} else {
			parts = append(parts, dimStyle.Render(t))
		}
	}
	return strings.Join(parts, dimStyle.Render("  │  "))
}

// renderFooter returns a box-bordered footer with contextual shortcuts.
func (m Model) renderFooter() string {
	var content string
	switch {
	case m.detail:
		content = "enter/esc retour · ctrl+q quitter"
	case m.activeTab == tabOptions:
		content = "j/k nav · espace/enter toggle · tab/esc Sessions"
	case m.activeTab == tabShortcuts:
		content = "tab Options · shift+tab Sessions · ctrl+q quitter"
	default: // tabSessions
		content = "j/k nav · enter détail · o options · a all/open · r refresh · ctrl+q quitter"
	}
	w := m.width - 2
	if w < 10 {
		w = 10
	}
	return footerBoxStyle.Width(w).Render(dimStyle.Render(content))
}

func (m Model) renderOptions() string {
	var b strings.Builder
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// --- Sons section ---
	b.WriteString(sectionHeaderStyle.Render("Sons"))
	b.WriteString("\n")

	check := "[ ]"
	if m.cfg.SoundEnabled {
		check = "[x]"
	}

	bars := make([]string, optionsMaxCursor+1)
	for i := range bars {
		bars[i] = unselectedBar
	}
	bars[m.optCursor] = cursorBar

	b.WriteString(bars[0])
	b.WriteString(fmt.Sprintf(" %s Activé\n", check))

	var sl strings.Builder
	for i, name := range availableSounds {
		if name == m.cfg.SoundName {
			sl.WriteString(fmt.Sprintf("[%s]", soundLabels[i]))
		} else {
			sl.WriteString(soundLabels[i])
		}
		if i < len(availableSounds)-1 {
			sl.WriteString("  ")
		}
	}
	line1 := fmt.Sprintf(" Son : %s", sl.String())
	b.WriteString(bars[1])
	if m.cfg.SoundEnabled {
		b.WriteString(line1)
	} else {
		b.WriteString(dimStyle.Render(line1))
	}
	b.WriteString("\n\n")

	// --- Colonnes section ---
	b.WriteString(sectionHeaderStyle.Render("Colonnes"))
	b.WriteString("\n")

	colOptions := []struct {
		label   string
		enabled bool
		idx     int
	}{
		{"Cache", m.cfg.ShowCache, 2},
		{"Ctx", m.cfg.ShowCtx, 3},
		{"Msgs", m.cfg.ShowMsgs, 4},
		{"Âge", m.cfg.ShowAge, 5},
		{"Badges", m.cfg.ShowBadges, 6},
	}
	for _, col := range colOptions {
		chk := "[ ]"
		if col.enabled {
			chk = "[x]"
		}
		b.WriteString(bars[col.idx])
		b.WriteString(fmt.Sprintf(" %s %s\n", chk, col.label))
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

// renderShortcuts shows all keyboard shortcuts.
func (m Model) renderShortcuts() string {
	var b strings.Builder
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render("Raccourcis clavier"))
	b.WriteString("\n\n")

	shortcuts := [][2]string{
		{"j / k", "naviguer haut/bas"},
		{"enter", "ouvrir le détail"},
		{"esc", "fermer le détail"},
		{"a", "afficher tout / sessions ouvertes"},
		{"r", "rafraîchir maintenant"},
		{"o", "ouvrir les Options"},
		{"tab", "onglet suivant"},
		{"shift+tab", "onglet précédent"},
		{"ctrl+q", "quitter"},
	}
	for _, s := range shortcuts {
		key := fmt.Sprintf("%-12s", s[0])
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			lipgloss.NewStyle().Bold(true).Render(key),
			dimStyle.Render(s[1])))
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

func (m Model) renderList() string {
	var b strings.Builder

	// Tab bar.
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Header line: title on the left, version on the right.
	appTitle := titleStyle.Render("ClaudeWatcher")
	ver := dimStyle.Render("v" + version.Version)
	mode := "open"
	if m.includeEnded {
		mode = "all"
	}
	sessionInfo := dimStyle.Render(fmt.Sprintf("%d sessions · %s", len(m.sessions), mode))
	// Build header with version right-aligned.
	titlePart := appTitle + "  " + sessionInfo
	// Rough visible length of titlePart (strip ANSI for width calculation).
	titlePartVisible := stripANSI(appTitle) + "  " + stripANSI(sessionInfo)
	verVisible := stripANSI(ver)
	gap := m.width - len(titlePartVisible) - len(verVisible)
	if gap < 1 {
		gap = 1
	}
	b.WriteString(titlePart)
	b.WriteString(strings.Repeat(" ", gap))
	b.WriteString(ver)
	b.WriteString("\n\n")

	narrow := m.width < 80

	if narrow {
		b.WriteString(m.renderListNarrow())
	} else {
		b.WriteString(m.renderListWide())
	}

	b.WriteString("\n")
	b.WriteString(m.renderFooter())
	return b.String()
}

// sessionBadges returns the inline badges string for a session (e.g. "[S] [MULTI]").
func sessionBadges(s session.Session) string {
	var parts []string
	if s.IsSubagent {
		parts = append(parts, badgeSubStyle.Render("[S]"))
	} else {
		parts = append(parts, badgeSubStyle.Render("[P]"))
	}
	if s.AwaySummaryCount >= 1 {
		parts = append(parts, badgeMultiStyle.Render("[MULTI]"))
	}
	if s.ApiErrorRate > 0.05 {
		parts = append(parts, badgeErrStyle.Render("[ERR]"))
	}
	if s.QueueDepth > 0 {
		parts = append(parts, badgeQueueStyle.Render(fmt.Sprintf("[Q:%d]", s.QueueDepth)))
	}
	return strings.Join(parts, " ")
}

// Column widths for the wide layout — single source of truth used by both
// the header and data rows to prevent alignment drift.
const (
	wideColProj  = 20
	wideColCtx   = 5
	wideColCache = 7
	wideColMsg   = 5
	wideColAgo   = 8
	wideColBadge = 12
	// gap chars between columns: icon(1) + spaces between each col = 7 separators
	wideColGaps = 7
)

// visibleRows returns how many sessions fit on screen given the current layout.
// Narrow layout uses 3 lines per session; wide uses 1 line per session.
// ~8 lines are reserved for tab bar, header, and footer box.
func (m Model) visibleRows() int {
	reserved := 8
	available := m.height - reserved
	if available < 1 {
		return 1
	}
	if m.width < 80 {
		rows := available / 3
		if rows < 1 {
			return 1
		}
		return rows
	}
	return available
}

// clampOffset adjusts the scroll offset so that cursor stays within the
// visible window [offset, offset+visibleRows).
func clampOffset(cursor, offset, visibleRows int) int {
	if cursor < offset {
		return cursor
	}
	if cursor >= offset+visibleRows {
		return cursor - visibleRows + 1
	}
	return offset
}

// renderListNarrow: three lines per session.
//
//	● Session title ················· active
//	  customer-biogen
//	  ctx 46% · 42 msgs · 2m
func (m Model) renderListNarrow() string {
	var b strings.Builder

	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.sessions) {
		end = len(m.sessions)
	}

	for i := m.offset; i < end; i++ {
		s := m.sessions[i]
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

		badges := sessionBadges(s)
		line2 := fmt.Sprintf("  %s  %s", truncate(s.ProjectName, m.width-2), badges)
		line3 := fmt.Sprintf("  ctx %s · %d msgs · %s",
			contextPct(s.ContextTokens, s.Model), s.MessageCount, humanizeAgo(s.LastModified))

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
// columns: status(1) space project(wideColProj) title(flex) [ctx] [cache] [msgs] [ago] [badges]
func (m Model) renderListWide() string {
	var b strings.Builder

	// Compute titleW by subtracting only the visible optional columns.
	optColsW := 0
	if m.cfg.ShowCtx {
		optColsW += wideColCtx + 1
	}
	if m.cfg.ShowCache {
		optColsW += wideColCache + 1
	}
	if m.cfg.ShowMsgs {
		optColsW += wideColMsg + 1
	}
	if m.cfg.ShowAge {
		optColsW += wideColAgo + 1
	}
	if m.cfg.ShowBadges {
		optColsW += wideColBadge + 2
	}

	// Fixed: 2 (cursor+icon) + project + 1 (space)
	titleW := m.width - 2 - wideColProj - optColsW
	if titleW < 10 {
		titleW = 10
	}

	// Build header.
	var hdr strings.Builder
	hdr.WriteString(fmt.Sprintf("  %-*s %-*s",
		wideColProj, "PROJECT",
		titleW, "TITLE"))
	if m.cfg.ShowCtx {
		hdr.WriteString(fmt.Sprintf(" %*s", wideColCtx, "CTX"))
	}
	if m.cfg.ShowCache {
		hdr.WriteString(fmt.Sprintf(" %*s", wideColCache, "CACHE"))
	}
	if m.cfg.ShowMsgs {
		hdr.WriteString(fmt.Sprintf(" %*s", wideColMsg, "MSGS"))
	}
	if m.cfg.ShowAge {
		hdr.WriteString(fmt.Sprintf(" %*s", wideColAgo, "AGE"))
	}
	if m.cfg.ShowBadges {
		hdr.WriteString(fmt.Sprintf("  %-*s", wideColBadge, "FLAGS"))
	}
	b.WriteString(headerStyle.Render(hdr.String()))
	b.WriteString("\n")

	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.sessions) {
		end = len(m.sessions)
	}

	for i := m.offset; i < end; i++ {
		s := m.sessions[i]
		st := statusStyles[s.Status]
		icon := st.Render(s.Status.Icon())
		title := s.Title
		if title == "" {
			title = "—"
		}

		var row strings.Builder
		row.WriteString(fmt.Sprintf("%s %-*s %-*s",
			icon,
			wideColProj, truncate(s.ProjectName, wideColProj),
			titleW, truncate(title, titleW)))

		if m.cfg.ShowCtx {
			row.WriteString(fmt.Sprintf(" %*s", wideColCtx, contextPct(s.ContextTokens, s.Model)))
		}
		if m.cfg.ShowCache {
			cacheStr := cachePct(s.CacheEfficiency)
			row.WriteString(" ")
			row.WriteString(padRight(cacheStr, wideColCache))
		}
		if m.cfg.ShowMsgs {
			row.WriteString(fmt.Sprintf(" %*d", wideColMsg, s.MessageCount))
		}
		if m.cfg.ShowAge {
			row.WriteString(fmt.Sprintf(" %*s", wideColAgo, humanizeAgo(s.LastModified)))
		}
		if m.cfg.ShowBadges {
			// Build badges inline.
			var badgeParts []string
			if s.IsSubagent {
				badgeParts = append(badgeParts, badgeSubStyle.Render("[S]"))
			} else {
				badgeParts = append(badgeParts, badgeSubStyle.Render("[P]"))
			}
			if s.AwaySummaryCount >= 1 {
				badgeParts = append(badgeParts, badgeMultiStyle.Render("[MULTI]"))
			}
			if s.ApiErrorRate > 0.05 {
				badgeParts = append(badgeParts, badgeErrStyle.Render("[ERR]"))
			}
			if s.QueueDepth > 0 {
				badgeParts = append(badgeParts, badgeQueueStyle.Render(fmt.Sprintf("[Q:%d]", s.QueueDepth)))
			}
			badges := strings.Join(badgeParts, " ")
			row.WriteString("  ")
			row.WriteString(badges)
		}

		bar := unselectedBar
		if i == m.cursor {
			bar = cursorBar
		}
		b.WriteString(bar)
		b.WriteString(row.String())
		b.WriteString("\n")
	}
	return b.String()
}

// cachePct returns the cache efficiency as a colored string, or "--".
func cachePct(eff float64) string {
	if eff < 0 {
		return dimStyle.Render("--")
	}
	pct := int(eff * 100)
	s := fmt.Sprintf("%d%%", pct)
	if pct >= 85 {
		return cacheGoodStyle.Render(s)
	}
	if pct >= 70 {
		return cacheMidStyle.Render(s)
	}
	return cacheBadStyle.Render(s)
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
	// Type badge.
	typeLabel := "Principal [P]"
	if s.IsSubagent {
		typeLabel = "Subagent [S]"
	}
	b.WriteString(fmt.Sprintf("Type:     %s\n", badgeSubStyle.Render(typeLabel)))
	if s.AwaySummaryCount >= 1 {
		b.WriteString(fmt.Sprintf("Multi:    %s (%d away summaries)\n",
			badgeMultiStyle.Render("[MULTI]"), s.AwaySummaryCount))
	}

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
		contextPct(s.ContextTokens, s.Model), s.ContextTokens, session.ContextWindowFor(s.Model)))
	b.WriteString(fmt.Sprintf("  Cache:    %s\n", cachePct(s.CacheEfficiency)))
	b.WriteString(fmt.Sprintf("  Messages: %d\n", s.MessageCount))
	b.WriteString(fmt.Sprintf("  Last:     %s (%s)\n",
		humanizeAgo(s.LastModified), s.LastModified.Format("2006-01-02 15:04:05")))
	if s.ApiErrorCount > 0 {
		b.WriteString(fmt.Sprintf("  API Err:  %d erreurs (%.0f%% des turns)\n",
			s.ApiErrorCount, s.ApiErrorRate*100))
	}
	if s.QueueDepth > 0 {
		b.WriteString(fmt.Sprintf("  Queue:    %d tâches en attente\n", s.QueueDepth))
	}

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
	b.WriteString(m.renderFooter())
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
func contextPct(tokens int, model string) string {
	if tokens <= 0 {
		return "—"
	}
	pct := tokens * 100 / session.ContextWindowFor(model)
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
	// padRight pads the visible string (after stripping ANSI) to w runes.
	vis := stripANSI(s)
	if len(vis) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(vis))
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
