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

	// Hover marker: a pale version of the selection bar, shown on the row the
	// mouse is over to signal it's clickable.
	hoverBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4A3D6B"))
	hoverBar      = hoverBarStyle.Render("▌")

	// Tab label shown under the mouse (brighter than the dim resting state).
	tabHoverStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#999"))

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
	mouseX       int // last known mouse column (-1 = off-screen)
	mouseY       int // last known mouse row (-1 = off-screen)
}

func NewModel() Model {
	cfg, _ := config.Load()
	return Model{
		cfg:        cfg,
		prevStatus: make(map[string]session.Status),
		mouseX:     -1,
		mouseY:     -1,
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
// Colonnes: 2=ShowCache, 3=ShowCtx, 4=ShowMsgs, 5=ShowAge, 6=ShowModel, 7=ShowVersion, 8=ShowBadges → 7 items
// Display: 9=NerdFonts → 1 item
// Total indices: 0..9 → 9
const optionsMaxCursor = 9

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
					m.cfg.ShowModel = !m.cfg.ShowModel
				case 7:
					m.cfg.ShowVersion = !m.cfg.ShowVersion
				case 8:
					m.cfg.ShowBadges = !m.cfg.ShowBadges
				case 9:
					m.cfg.NerdFonts = !m.cfg.NerdFonts
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
		if m.activeTab == tabShortcuts {
			switch msg.String() {
			case "ctrl+q", "ctrl+c":
				return m, tea.Quit
			case "tab":
				m.activeTab = (m.activeTab + 1) % 3
			case "shift+tab":
				m.activeTab = (m.activeTab + 2) % 3
			case "esc":
				m.activeTab = tabSessions
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
	case tea.MouseMsg:
		// Track the pointer so the renderer can highlight the hovered target.
		m.mouseX, m.mouseY = msg.X, msg.Y
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		// Wheel scroll over the Sessions list moves the cursor.
		if !m.detail && m.activeTab == tabSessions {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				if m.cursor > 0 {
					m.cursor--
					m.offset = clampOffset(m.cursor, m.offset, m.visibleRows())
				}
				return m, nil
			case tea.MouseButtonWheelDown:
				if m.cursor < len(m.sessions)-1 {
					m.cursor++
					m.offset = clampOffset(m.cursor, m.offset, m.visibleRows())
				}
				return m, nil
			}
		}
		if msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		// Tab bar sits on row 0 of every non-detail view.
		if !m.detail && msg.Y == 0 {
			if tab, ok := tabAtX(msg.X); ok {
				m.activeTab = tab
				config.Save(m.cfg) //nolint:errcheck
			}
			return m, nil
		}
		// A click on a session row selects it; clicking the selected row opens detail.
		if !m.detail && m.activeTab == tabSessions {
			if idx, ok := m.sessionAtY(msg.Y); ok {
				if idx == m.cursor {
					m.detail = true
				} else {
					m.cursor = idx
					m.offset = clampOffset(m.cursor, m.offset, m.visibleRows())
				}
			}
			return m, nil
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

// sortSessions: active > waiting > idle > ended, then by recency.
// Main sessions (ParentID == "") are sorted first, then each main session is
// immediately followed by its active subagents sorted by recency.
func sortSessions(s []session.Session) []session.Session {
	// Separate main sessions from subagents.
	var mains, subs []session.Session
	for _, sess := range s {
		if sess.ParentID == "" {
			mains = append(mains, sess)
		} else {
			subs = append(subs, sess)
		}
	}

	// Sort main sessions: status desc, then recency desc.
	sort.SliceStable(mains, func(i, j int) bool {
		if mains[i].Status != mains[j].Status {
			return mains[i].Status > mains[j].Status
		}
		return mains[i].LastModified.After(mains[j].LastModified)
	})

	// Sort subagents: recency desc (status is always active at this point due to
	// scanner filtering, but sort consistently anyway).
	sort.SliceStable(subs, func(i, j int) bool {
		if subs[i].Status != subs[j].Status {
			return subs[i].Status > subs[j].Status
		}
		return subs[i].LastModified.After(subs[j].LastModified)
	})

	// Build a lookup of parentID → subagents.
	subsByParent := map[string][]session.Session{}
	for _, sub := range subs {
		subsByParent[sub.ParentID] = append(subsByParent[sub.ParentID], sub)
	}

	// Interleave: each main session followed by its children.
	out := make([]session.Session, 0, len(s))
	for _, m := range mains {
		out = append(out, m)
		out = append(out, subsByParent[m.ID]...)
	}
	return out
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
	// The tab bar is always on row 0; light up the label under the pointer.
	hovered := -1
	if m.mouseY == 0 {
		if t, ok := tabAtX(m.mouseX); ok {
			hovered = t
		}
	}
	var parts []string
	for i, t := range tabBarLabels {
		switch {
		case i == m.activeTab:
			parts = append(parts, lipgloss.NewStyle().
				Bold(true).Foreground(lipgloss.Color("#7D56F4")).
				Underline(true).Render(t))
		case i == hovered:
			parts = append(parts, tabHoverStyle.Render(t))
		default:
			parts = append(parts, dimStyle.Render(t))
		}
	}
	return strings.Join(parts, dimStyle.Render("  │  "))
}

// tabBarLabels is the single source of truth for tab labels and their order;
// shared by renderTabBar (display) and tabAtX (click hit-testing).
var tabBarLabels = []string{"Sessions", "Options", "Shortcuts"}

// tabBarSepWidth is the visible width of the "  │  " separator between tabs.
const tabBarSepWidth = 5

// tabAtX maps an X column on the tab bar (row 0) to a tab index.
func tabAtX(x int) (int, bool) {
	col := 0
	for i, l := range tabBarLabels {
		if x >= col && x < col+len(l) {
			return i, true
		}
		col += len(l) + tabBarSepWidth
	}
	return 0, false
}

// sessionAtY maps a screen row Y to a session index in the current list view.
// Layout (see renderList): row 0 tab bar, row 1 header, row 2 blank. Wide mode
// adds a column header on row 3 and data rows from row 4; narrow mode packs 3
// lines per session starting at row 3.
func (m Model) sessionAtY(y int) (int, bool) {
	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.sessions) {
		end = len(m.sessions)
	}
	var rel int
	if m.width < 80 {
		if y < 3 {
			return 0, false
		}
		rel = (y - 3) / 3
	} else {
		if y < 4 {
			return 0, false
		}
		rel = y - 4
	}
	idx := m.offset + rel
	if idx < m.offset || idx >= end {
		return 0, false
	}
	return idx, true
}

// hoveredSession returns the session index currently under the mouse pointer,
// or -1 when the pointer isn't over a row.
func (m Model) hoveredSession() int {
	if idx, ok := m.sessionAtY(m.mouseY); ok {
		return idx
	}
	return -1
}

// withFooter pins the contextual footer to the bottom of the screen, padding
// the body with blank lines so the footer always sits on the last row instead
// of floating right below the content.
func (m Model) withFooter(body string) string {
	body = strings.TrimRight(body, "\n")
	footer := m.renderFooter()
	bodyRows := strings.Count(body, "\n") + 1
	// The footer is the final segment (no trailing newline), so total rendered
	// lines == bodyRows + pad. Fill exactly to the screen height so the footer
	// lands on the last row with no blank line below it.
	pad := m.height - bodyRows
	if pad < 1 {
		pad = 1
	}
	return body + strings.Repeat("\n", pad) + footer
}

// renderFooter returns a single dim line of contextual shortcuts.
func (m Model) renderFooter() string {
	var content string
	switch {
	case m.detail:
		content = "enter/esc back · ctrl+q quit"
	case m.activeTab == tabOptions:
		content = "j/k nav · space/enter toggle · tab Shortcuts · esc Sessions"
	case m.activeTab == tabShortcuts:
		content = "tab Sessions · shift+tab Options · ctrl+q quit"
	default: // tabSessions
		content = "j/k nav · enter detail · o options · a all/open · r refresh · ctrl+q quit"
	}
	return dimStyle.Render(content)
}

func (m Model) renderOptions() string {
	var b strings.Builder
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// --- Sons section ---
	b.WriteString(sectionHeaderStyle.Render("Sounds"))
	b.WriteString("\n")

	check := "[ ]"
	if m.cfg.SoundEnabled {
		check = "[x]"
	}

	bars := make([]string, optionsMaxCursor+1) // indices 0..optionsMaxCursor
	for i := range bars {
		bars[i] = unselectedBar
	}
	if m.optCursor <= optionsMaxCursor {
		bars[m.optCursor] = cursorBar
	}

	b.WriteString(bars[0])
	b.WriteString(fmt.Sprintf(" %s Enabled\n", check))

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
	line1 := fmt.Sprintf(" Sound: %s", sl.String())
	b.WriteString(bars[1])
	if m.cfg.SoundEnabled {
		b.WriteString(line1)
	} else {
		b.WriteString(dimStyle.Render(line1))
	}
	b.WriteString("\n\n")

	// --- Colonnes section ---
	b.WriteString(sectionHeaderStyle.Render("Columns"))
	b.WriteString("\n")

	colOptions := []struct {
		label   string
		enabled bool
		idx     int
	}{
		{"Cache", m.cfg.ShowCache, 2},
		{"Ctx", m.cfg.ShowCtx, 3},
		{"Msgs", m.cfg.ShowMsgs, 4},
		{"Age", m.cfg.ShowAge, 5},
		{"Model", m.cfg.ShowModel, 6},
		{"CLI version", m.cfg.ShowVersion, 7},
		{"Badges", m.cfg.ShowBadges, 8},
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

	// --- Display section ---
	b.WriteString(sectionHeaderStyle.Render("Display"))
	b.WriteString("\n")

	nfCheck := "[ ]"
	if m.cfg.NerdFonts {
		nfCheck = "[x]"
	}
	b.WriteString(bars[9])
	b.WriteString(fmt.Sprintf(" %s Nerd Fonts\n", nfCheck))

	// Visual hint line below the toggle: if the glyph renders as an icon the font works.
	if m.cfg.NerdFonts {
		glyph := lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50")).Bold(true).Render("") // fa-check-circle U+F058
		b.WriteString(fmt.Sprintf("    → %s if you see an icon, Nerd Fonts work\n", glyph))
	}

	return m.withFooter(b.String())
}

// renderShortcuts shows all keyboard shortcuts.
func (m Model) renderShortcuts() string {
	var b strings.Builder
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Header line: title on the left, version on the right (same as renderList).
	appTitle := titleStyle.Render("ClaudeWatcher")
	ver := dimStyle.Render("v" + version.Version)
	mode := "open"
	if m.includeEnded {
		mode = "all"
	}
	sessionInfo := dimStyle.Render(fmt.Sprintf("%d sessions · %s", len(m.sessions), mode))
	titlePart := appTitle + "  " + sessionInfo
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

	b.WriteString(headerStyle.Render("Shortcuts"))
	b.WriteString("\n\n")

	shortcuts := [][2]string{
		{"j / k", "navigate up/down"},
		{"enter", "open detail"},
		{"esc", "close detail"},
		{"a", "show all / open sessions"},
		{"r", "refresh now"},
		{"o", "open Options"},
		{"tab", "next tab"},
		{"shift+tab", "previous tab"},
		{"ctrl+q", "quit"},
	}
	colWidth := m.width/2 - 4
	half := (len(shortcuts) + 1) / 2
	for i := 0; i < half; i++ {
		lKey := fmt.Sprintf("%-12s", shortcuts[i][0])
		lDesc := fmt.Sprintf("%-*s", colWidth, shortcuts[i][1])
		line := fmt.Sprintf("  %s  %s",
			lipgloss.NewStyle().Bold(true).Render(lKey),
			dimStyle.Render(lDesc))
		if i+half < len(shortcuts) {
			rKey := fmt.Sprintf("%-12s", shortcuts[i+half][0])
			line += fmt.Sprintf("    %s  %s",
				lipgloss.NewStyle().Bold(true).Render(rKey),
				dimStyle.Render(shortcuts[i+half][1]))
		}
		b.WriteString(line + "\n")
	}

	// Badge reference section.
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Badge reference"))
	b.WriteString("\n\n")

	type badgeEntry struct {
		asciiTag  string
		nerdGlyph string
		style     lipgloss.Style
		label     string
	}
	entries := []badgeEntry{
		{"[P]", "\uEE0D", badgeSubStyle, "principal session"},
		{"[S]", "\U000F0B46", badgeSubStyle, "subagent"},
		{"[MULTI]", "\uEF38", badgeMultiStyle, "multi-day session"},
		{"[ERR]", "\uEA87", badgeErrStyle, "API error rate > 5%"},
		{"[Q:N]", "\U000F1571 N", badgeQueueStyle, "N queued tasks"},
	}
	for _, e := range entries {
		var tag string
		if m.cfg.NerdFonts {
			tag = e.style.Render(e.nerdGlyph)
		} else {
			tag = e.style.Render(e.asciiTag)
		}
		b.WriteString(fmt.Sprintf("  %-14s %s\n", tag, dimStyle.Render(e.label)))
	}

	return m.withFooter(b.String())
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

	return m.withFooter(b.String())
}

// sessionBadges returns the inline badges string for a session (e.g. "[S] [MULTI]").
// When dim is true (idle or ended sessions), all badges are rendered in dimStyle.
// When nerd is true, compact Nerd Font icons are used instead of ASCII text badges.
func sessionBadges(s session.Session, dim bool, nerd bool) string {
	badge := func(style lipgloss.Style, text string) string {
		if dim {
			return dimStyle.Render(text)
		}
		return style.Render(text)
	}
	var parts []string
	if nerd {
		// Nerd Font compact icons.
		if s.IsSubagent {
			parts = append(parts, badge(badgeSubStyle, "\U000F0B46")) // U+F0B46
		} else {
			parts = append(parts, badge(badgeSubStyle, "\uEE0D")) // U+EE0D
		}
		if s.AwaySummaryCount >= 1 {
			parts = append(parts, badge(badgeMultiStyle, "\uEF38")) // U+EF38
		}
		if s.ApiErrorRate > 0.05 {
			parts = append(parts, badge(badgeErrStyle, "\uEA87")) // U+EA87
		}
		if s.QueueDepth > 0 {
			parts = append(parts, badge(badgeQueueStyle, fmt.Sprintf("\U000F1571 %d", s.QueueDepth))) // U+F1571
		}
	} else {
		// Plain ASCII badges.
		if s.IsSubagent {
			parts = append(parts, badge(badgeSubStyle, "[S]"))
		} else {
			parts = append(parts, badge(badgeSubStyle, "[P]"))
		}
		if s.AwaySummaryCount >= 1 {
			parts = append(parts, badge(badgeMultiStyle, "[MULTI]"))
		}
		if s.ApiErrorRate > 0.05 {
			parts = append(parts, badge(badgeErrStyle, "[ERR]"))
		}
		if s.QueueDepth > 0 {
			parts = append(parts, badge(badgeQueueStyle, fmt.Sprintf("[Q:%d]", s.QueueDepth)))
		}
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
	wideColModel = 10
	wideColVer   = 8 // fits "2.1.197" with room to spare
	wideColBadge = 12
	// gap chars between columns: icon(1) + spaces between each col = 7 separators
	wideColGaps = 7
)

// visibleRows returns how many sessions fit on screen given the current layout.
// Narrow layout uses 3 lines per session; wide uses 1 line per session.
// Chrome above the list: tab bar + header + blank line (3 rows), plus the wide
// layout's column-header row (1). The single-line footer takes the last row.
func (m Model) visibleRows() int {
	if m.width < 80 {
		available := m.height - 4 // 3 chrome + 1 footer
		rows := available / 3
		if rows < 1 {
			return 1
		}
		return rows
	}
	available := m.height - 5 // 4 chrome + 1 footer
	if available < 1 {
		return 1
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

	hovered := m.hoveredSession()
	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.sessions) {
		end = len(m.sessions)
	}

	for i := m.offset; i < end; i++ {
		s := m.sessions[i]
		st := statusStyles[s.Status]
		icon := st.Render(session.StatusIcon(s.Status, m.cfg.NerdFonts))
		// Idle and ended sessions use dimStyle for badges and column values.
		dim := s.Status == session.StatusIdle || s.Status == session.StatusEnded

		title := s.Title
		if title == "" {
			title = "(no title)"
		}
		status := s.Status.Label()

		// line 1: icon + title ··· status, with dotted leader
		// budget: width - 2 (icon+space) - len(status) - 1 (space before status)
		// Reduce by 4 for subagents to account for the "    " indent prefix.
		indentW := 0
		if s.ParentID != "" {
			indentW = 4
		}
		leftBudget := m.width - 2 - len(status) - 1 - indentW
		if leftBudget < 8 {
			leftBudget = 8
		}
		title = truncate(title, leftBudget-2) // room for at least 2 dots
		fillN := leftBudget - len(title) - 1  // 1 = space after title
		if fillN < 1 {
			fillN = 1
		}
		line1 := fmt.Sprintf("%s %s %s %s",
			icon,
			title,
			dimStyle.Render(strings.Repeat("·", fillN)),
			st.Render(status))

		badges := sessionBadges(s, dim, m.cfg.NerdFonts)
		line2 := fmt.Sprintf("  %s  %s", truncate(s.ProjectName, m.width-2), badges)
		var line3parts []string
		if m.cfg.ShowCtx {
			line3parts = append(line3parts, "ctx "+contextPct(s.ContextTokens, s.Model))
		}
		if m.cfg.ShowMsgs {
			line3parts = append(line3parts, fmt.Sprintf("%d msgs", s.MessageCount))
		}
		if m.cfg.ShowAge {
			line3parts = append(line3parts, humanizeAgo(s.LastModified))
		}
		if m.cfg.ShowModel {
			if label := session.ModelLabel(s.Model); label != "" {
				line3parts = append(line3parts, label)
			}
		}
		if m.cfg.ShowVersion {
			if s.Version != "" {
				line3parts = append(line3parts, "cli "+s.Version)
			}
		}
		line3 := "  " + strings.Join(line3parts, " · ")

		// Subagents get a 4-space indent prefix on every line.
		indent := ""
		if s.ParentID != "" {
			indent = "    "
		}

		bar := unselectedBar
		switch {
		case i == m.cursor:
			bar = cursorBar
		case i == hovered:
			bar = hoverBar
		}
		b.WriteString(bar)
		b.WriteString(indent)
		b.WriteString(line1)
		b.WriteString("\n")
		b.WriteString(bar)
		b.WriteString(indent)
		b.WriteString(line2)
		b.WriteString("\n")
		b.WriteString(bar)
		b.WriteString(indent)
		// Non-selected rows: always dim. Selected rows: dim only for idle/ended.
		if i != m.cursor || dim {
			b.WriteString(dimStyle.Render(line3))
		} else {
			b.WriteString(line3)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// renderListWide: one row per session.
// columns: status(1) space project(wideColProj) title(flex) [ctx] [cache] [msgs] [ago] [badges]
func (m Model) renderListWide() string {
	var b strings.Builder

	hovered := m.hoveredSession()

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
	if m.cfg.ShowModel {
		optColsW += wideColModel + 1
	}
	if m.cfg.ShowVersion {
		optColsW += wideColVer + 1
	}
	if m.cfg.ShowBadges {
		optColsW += wideColBadge + 2
	}

	// Fixed: bar(1) + icon(1) + space(1) + project + space(1) = 4
	titleW := m.width - 4 - wideColProj - optColsW
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
	if m.cfg.ShowModel {
		hdr.WriteString(fmt.Sprintf(" %-*s", wideColModel, "MODEL"))
	}
	if m.cfg.ShowVersion {
		hdr.WriteString(fmt.Sprintf(" %-*s", wideColVer, "CLI"))
	}
	if m.cfg.ShowBadges {
		hdr.WriteString(fmt.Sprintf("  %-*s", wideColBadge, "BADGES"))
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
		// Subagents get a tree-branch prefix instead of just the icon.
		iconRaw := st.Render(session.StatusIcon(s.Status, m.cfg.NerdFonts))
		isSubagent := s.ParentID != ""
		// titleW for subagents is reduced by the extra prefix width (4 chars: "  └ ").
		rowTitleW := titleW
		if isSubagent {
			rowTitleW -= 4
			if rowTitleW < 6 {
				rowTitleW = 6
			}
		}
		title := s.Title
		if title == "" {
			title = "—"
		}
		// Idle and ended sessions use dimStyle for all column values and badges.
		dim := s.Status == session.StatusIdle || s.Status == session.StatusEnded

		var row strings.Builder
		projStr := fmt.Sprintf("%-*s", wideColProj, truncate(s.ProjectName, wideColProj))
		titleStr := fmt.Sprintf("%-*s", rowTitleW, truncate(title, rowTitleW))
		if dim {
			projStr = dimStyle.Render(projStr)
			titleStr = dimStyle.Render(titleStr)
		}
		if isSubagent {
			row.WriteString(fmt.Sprintf("  └ %s %s %s", iconRaw, projStr, titleStr))
		} else {
			row.WriteString(fmt.Sprintf("%s %s %s", iconRaw, projStr, titleStr))
		}

		if m.cfg.ShowCtx {
			ctxStr := fmt.Sprintf("%*s", wideColCtx, contextPct(s.ContextTokens, s.Model))
			if dim {
				ctxStr = dimStyle.Render(ctxStr)
			}
			row.WriteString(" ")
			row.WriteString(ctxStr)
		}
		if m.cfg.ShowCache {
			cacheStr := cachePct(s.CacheEfficiency, dim)
			row.WriteString(" ")
			row.WriteString(padRight(cacheStr, wideColCache))
		}
		if m.cfg.ShowMsgs {
			msgsStr := fmt.Sprintf("%*s", wideColMsg, fmt.Sprintf("%d", s.MessageCount))
			if dim {
				msgsStr = dimStyle.Render(msgsStr)
			}
			row.WriteString(" ")
			row.WriteString(msgsStr)
		}
		if m.cfg.ShowAge {
			ageStr := fmt.Sprintf("%*s", wideColAgo, humanizeAgo(s.LastModified))
			if dim {
				ageStr = dimStyle.Render(ageStr)
			}
			row.WriteString(" ")
			row.WriteString(ageStr)
		}
		if m.cfg.ShowModel {
			modelStr := fmt.Sprintf("%-*s", wideColModel, truncate(session.ModelLabel(s.Model), wideColModel))
			if dim {
				modelStr = dimStyle.Render(modelStr)
			}
			row.WriteString(" ")
			row.WriteString(modelStr)
		}
		if m.cfg.ShowVersion {
			verStr := fmt.Sprintf("%-*s", wideColVer, truncate(versionLabel(s.Version), wideColVer))
			if dim {
				verStr = dimStyle.Render(verStr)
			}
			row.WriteString(" ")
			row.WriteString(verStr)
		}
		if m.cfg.ShowBadges {
			badges := sessionBadges(s, dim, m.cfg.NerdFonts)
			row.WriteString("  ")
			row.WriteString(badges)
		}

		bar := unselectedBar
		switch {
		case i == m.cursor:
			bar = cursorBar
		case i == hovered:
			bar = hoverBar
		}
		b.WriteString(bar)
		b.WriteString(row.String())
		b.WriteString("\n")
	}
	return b.String()
}

// cachePct returns the cache efficiency as a colored string, or "--".
// When dim is true (idle or ended sessions), the value is rendered in dimStyle.
func cachePct(eff float64, dim bool) string {
	if eff < 0 {
		return dimStyle.Render("--")
	}
	pct := int(eff * 100)
	s := fmt.Sprintf("%d%%", pct)
	if dim {
		return dimStyle.Render(s)
	}
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
	b.WriteString(fmt.Sprintf("Status:   %s %s\n", st.Render(session.StatusIcon(s.Status, m.cfg.NerdFonts)), st.Render(s.Status.Label())))
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
	if label := session.ModelLabel(s.Model); label != "" {
		b.WriteString(fmt.Sprintf("  Model:    %s\n", label))
	}
	if s.Version != "" {
		b.WriteString(fmt.Sprintf("  CLI:      %s\n", s.Version))
	}
	b.WriteString(fmt.Sprintf("  Context:  %s (%d / %d tokens)\n",
		contextPct(s.ContextTokens, s.Model), s.ContextTokens, session.ContextWindowFor(s.Model)))
	b.WriteString(fmt.Sprintf("  Cache:    %s\n", cachePct(s.CacheEfficiency, false)))
	b.WriteString(fmt.Sprintf("  Messages: %d\n", s.MessageCount))
	b.WriteString(fmt.Sprintf("  Last:     %s (%s)\n",
		humanizeAgo(s.LastModified), s.LastModified.Format("2006-01-02 15:04:05")))
	if s.ApiErrorCount > 0 {
		b.WriteString(fmt.Sprintf("  API Err:  %d errors (%.0f%% of turns)\n",
			s.ApiErrorCount, s.ApiErrorRate*100))
	}
	if s.QueueDepth > 0 {
		b.WriteString(fmt.Sprintf("  Queue:    %d pending tasks\n", s.QueueDepth))
	}

	// Last assistant message preview
	if s.LastAssistant != "" {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Last message"))
		b.WriteString("\n")
		b.WriteString(wrapPreview(s.LastAssistant, m.width-4, 12))
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("jsonl: %s\n", s.JSONLPath)))
	return m.withFooter(b.String())
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

// versionLabel returns the CLI version string, or "—" when unknown.
func versionLabel(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

// contextPct returns the context usage as "46%" (or "—" if unknown).
func contextPct(tokens int, model string) string {
	if tokens <= 0 {
		return "—"
	}
	pct := tokens * 100 / session.ContextWindowFor(model)
	return fmt.Sprintf("%d%%", pct)
}

// wrapText splits text on existing \n, then word-wraps each line to width.
func wrapText(text string, width int) string {
	if width < 10 {
		width = 10
	}
	var result []string
	for _, line := range strings.Split(text, "\n") {
		if len(line) <= width {
			result = append(result, line)
			continue
		}
		words := strings.Fields(line)
		current := ""
		for _, w := range words {
			if current == "" {
				current = w
			} else if len(current)+1+len(w) <= width {
				current += " " + w
			} else {
				result = append(result, current)
				current = w
			}
		}
		if current != "" {
			result = append(result, current)
		}
	}
	return strings.Join(result, "\n")
}

// wrapPreview wraps text preserving \n, word-wraps to width, caps at maxLines,
// and adds a "  " prefix to each line.
func wrapPreview(s string, width, maxLines int) string {
	if width < 10 {
		width = 10
	}
	// word-wrap preserving existing newlines (use inner width for the "  " prefix)
	wrapped := wrapText(strings.TrimRight(s, "\n"), width-2)
	lines := strings.Split(wrapped, "\n")

	truncated := false
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	var out []string
	for _, l := range lines {
		out = append(out, "  "+l)
	}
	if truncated {
		out = append(out, "  [...]")
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
