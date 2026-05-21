# Options Menu + Sound Notifications — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ajouter un panneau Options (touche `o`) à ClaudeWatcher permettant d'activer des notifications sonores via `afplay` lors des transitions de statut de session, avec configuration persistée en JSON.

**Architecture:** Deux nouveaux packages (`internal/config`, `internal/audio`) exposant des interfaces minimales ; `model.go` reçoit trois nouveaux champs (`cfg`, `options`, `prevStatus`) et une fonction privée `detectTransitions` extraite dans `internal/tui/transitions.go` pour la testabilité.

**Tech Stack:** Go stdlib (`os/exec`, `encoding/json`, `os`), Bubbletea (déjà présent), `afplay` (macOS).

---

## Fichiers

| Action   | Chemin                                    | Responsabilité                          |
|----------|-------------------------------------------|-----------------------------------------|
| Créer    | `internal/config/config.go`               | Struct Config + Load + Save             |
| Créer    | `internal/config/config_test.go`          | Tests Load/Save                         |
| Créer    | `internal/audio/audio.go`                 | Play(name) via afplay goroutine         |
| Créer    | `internal/audio/audio_test.go`            | Smoke tests                             |
| Créer    | `internal/tui/transitions.go`             | detectTransitions(prev, curr) bool      |
| Créer    | `internal/tui/transitions_test.go`        | Tests détection de transitions          |
| Modifier | `internal/tui/model.go`                   | Nouveaux champs, options panel, wiring  |

---

## Task 1 — Package config (Load / Save)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1 : Écrire les tests**

```go
// internal/config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ludo/claudewatcher/internal/config"
)

func TestLoadReturnsDefaultsWhenFileAbsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SoundEnabled != false {
		t.Errorf("SoundEnabled = %v, want false", cfg.SoundEnabled)
	}
	if cfg.SoundName != "glass" {
		t.Errorf("SoundName = %q, want \"glass\"", cfg.SoundName)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := config.Config{SoundEnabled: true, SoundName: "ping"}
	if err := config.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() after Save error = %v", err)
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := config.Save(config.Config{SoundEnabled: false, SoundName: "funk"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	path := filepath.Join(tmp, ".config", "claudewatcher", "config.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("config file not created at %s", path)
	}
}
```

- [ ] **Step 2 : Vérifier que les tests échouent**

```bash
go test ./internal/config/...
```

Résultat attendu : erreur de compilation (`no Go files in internal/config`).

- [ ] **Step 3 : Implémenter `config.go`**

```go
// internal/config/config.go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	SoundEnabled bool   `json:"sound_enabled"`
	SoundName    string `json:"sound_name"`
}

var defaults = Config{
	SoundEnabled: false,
	SoundName:    "glass",
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "claudewatcher", "config.json"), nil
}

func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return defaults, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaults, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4 : Vérifier que les tests passent**

```bash
go test ./internal/config/...
```

Résultat attendu : `ok  github.com/ludo/claudewatcher/internal/config`

- [ ] **Step 5 : Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add Config struct with Load and Save"
```

---

## Task 2 — Package audio (Play via afplay)

**Files:**
- Create: `internal/audio/audio.go`
- Create: `internal/audio/audio_test.go`

- [ ] **Step 1 : Écrire les tests**

```go
// internal/audio/audio_test.go
package audio_test

import (
	"testing"

	"github.com/ludo/claudewatcher/internal/audio"
)

func TestPlayKnownSoundsDoNotPanic(t *testing.T) {
	audio.Play("glass")
	audio.Play("ping")
	audio.Play("funk")
}

func TestPlayUnknownFallsBackSilently(t *testing.T) {
	audio.Play("unknown")
	audio.Play("")
}
```

- [ ] **Step 2 : Vérifier que les tests échouent**

```bash
go test ./internal/audio/...
```

Résultat attendu : erreur de compilation (`no Go files in internal/audio`).

- [ ] **Step 3 : Implémenter `audio.go`**

```go
// internal/audio/audio.go
package audio

import "os/exec"

var soundFiles = map[string]string{
	"glass": "/System/Library/Sounds/Glass.aiff",
	"ping":  "/System/Library/Sounds/Ping.aiff",
	"funk":  "/System/Library/Sounds/Funk.aiff",
}

func Play(name string) {
	path, ok := soundFiles[name]
	if !ok {
		path = soundFiles["glass"]
	}
	go exec.Command("afplay", path).Run() //nolint:errcheck
}
```

- [ ] **Step 4 : Vérifier que les tests passent**

```bash
go test ./internal/audio/...
```

Résultat attendu : `ok  github.com/ludo/claudewatcher/internal/audio`

- [ ] **Step 5 : Commit**

```bash
git add internal/audio/audio.go internal/audio/audio_test.go
git commit -m "feat(audio): add Play function via afplay goroutine"
```

---

## Task 3 — Détection des transitions de statut

**Files:**
- Create: `internal/tui/transitions.go`
- Create: `internal/tui/transitions_test.go`

- [ ] **Step 1 : Écrire les tests**

```go
// internal/tui/transitions_test.go
package tui

import (
	"testing"

	"github.com/ludo/claudewatcher/internal/session"
)

func TestDetectTransitions(t *testing.T) {
	tests := []struct {
		name string
		prev map[string]session.Status
		curr []session.Session
		want bool
	}{
		{
			name: "aucun changement",
			prev: map[string]session.Status{"a": session.StatusActive},
			curr: []session.Session{{ID: "a", Status: session.StatusActive}},
			want: false,
		},
		{
			name: "active vers waiting déclenche",
			prev: map[string]session.Status{"a": session.StatusActive},
			curr: []session.Session{{ID: "a", Status: session.StatusWaiting}},
			want: true,
		},
		{
			name: "active vers idle déclenche",
			prev: map[string]session.Status{"a": session.StatusActive},
			curr: []session.Session{{ID: "a", Status: session.StatusIdle}},
			want: true,
		},
		{
			name: "active vers ended déclenche",
			prev: map[string]session.Status{"a": session.StatusActive},
			curr: []session.Session{{ID: "a", Status: session.StatusEnded}},
			want: true,
		},
		{
			name: "nouvelle session ne déclenche pas",
			prev: map[string]session.Status{},
			curr: []session.Session{{ID: "new", Status: session.StatusWaiting}},
			want: false,
		},
		{
			name: "waiting vers waiting ne déclenche pas",
			prev: map[string]session.Status{"a": session.StatusWaiting},
			curr: []session.Session{{ID: "a", Status: session.StatusWaiting}},
			want: false,
		},
		{
			name: "waiting vers active ne déclenche pas",
			prev: map[string]session.Status{"a": session.StatusWaiting},
			curr: []session.Session{{ID: "a", Status: session.StatusActive}},
			want: false,
		},
		{
			name: "deux sessions dont une transite",
			prev: map[string]session.Status{
				"a": session.StatusActive,
				"b": session.StatusActive,
			},
			curr: []session.Session{
				{ID: "a", Status: session.StatusActive},
				{ID: "b", Status: session.StatusWaiting},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectTransitions(tt.prev, tt.curr)
			if got != tt.want {
				t.Errorf("detectTransitions() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2 : Vérifier que les tests échouent**

```bash
go test ./internal/tui/...
```

Résultat attendu : erreur de compilation (`undefined: detectTransitions`).

- [ ] **Step 3 : Implémenter `transitions.go`**

```go
// internal/tui/transitions.go
package tui

import "github.com/ludo/claudewatcher/internal/session"

// detectTransitions retourne true si au moins une session a transité vers
// Waiting, Idle ou Ended depuis le dernier scan.
// Les nouvelles sessions (absentes de prev) ne déclenchent pas de son.
func detectTransitions(prev map[string]session.Status, curr []session.Session) bool {
	for _, s := range curr {
		prevStatus, seen := prev[s.ID]
		if !seen || prevStatus == s.Status {
			continue
		}
		if s.Status == session.StatusWaiting ||
			s.Status == session.StatusIdle ||
			s.Status == session.StatusEnded {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4 : Vérifier que les tests passent**

```bash
go test ./internal/tui/...
```

Résultat attendu : `ok  github.com/ludo/claudewatcher/internal/tui`

- [ ] **Step 5 : Commit**

```bash
git add internal/tui/transitions.go internal/tui/transitions_test.go
git commit -m "feat(tui): add detectTransitions for sound trigger logic"
```

---

## Task 4 — Panneau Options dans model.go

**Files:**
- Modify: `internal/tui/model.go`

Cette tâche ajoute les champs, le rendu du panneau et la gestion des touches. Le son est câblé dans la Task 5.

- [ ] **Step 1 : Ajouter les imports manquants et les nouveaux champs dans `Model`**

Dans `internal/tui/model.go`, mettre à jour l'import et le struct :

```go
import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ludo/claudewatcher/internal/config"
	"github.com/ludo/claudewatcher/internal/scanner"
	"github.com/ludo/claudewatcher/internal/session"
)
```

```go
type Model struct {
	sessions     []session.Session
	cursor       int
	width        int
	height       int
	err          error
	detail       bool
	includeEnded bool
	options      bool
	optCursor    int
	cfg          config.Config
	prevStatus   map[string]session.Status
}
```

- [ ] **Step 2 : Charger la config dans `NewModel`**

```go
func NewModel() Model {
	cfg, _ := config.Load()
	return Model{
		cfg:        cfg,
		prevStatus: make(map[string]session.Status),
	}
}
```

- [ ] **Step 3 : Mettre à jour `View()` pour insérer le panneau options**

```go
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}
	if m.width == 0 {
		return "Loading..."
	}
	if m.options {
		return m.renderOptions()
	}
	if m.detail && len(m.sessions) > 0 {
		return m.renderDetail()
	}
	return m.renderList()
}
```

- [ ] **Step 4 : Ajouter `renderOptions()`**

```go
func (m Model) renderOptions() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Options"))
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render("Sons"))
	b.WriteString("\n")

	check := "[ ]"
	if m.cfg.SoundEnabled {
		check = "[x]"
	}
	bar0, bar1 := unselectedBar, unselectedBar
	if m.optCursor == 0 {
		bar0 = cursorBar
	} else {
		bar1 = cursorBar
	}

	b.WriteString(bar0)
	b.WriteString(fmt.Sprintf(" %s Activé\n", check))

	sounds := []string{"glass", "ping", "funk"}
	labels := []string{"Glass", "Ping", "Funk"}
	var sl strings.Builder
	for i, name := range sounds {
		if name == m.cfg.SoundName {
			sl.WriteString(fmt.Sprintf("[%s]", labels[i]))
		} else {
			sl.WriteString(labels[i])
		}
		if i < len(sounds)-1 {
			sl.WriteString("  ")
		}
	}
	line1 := fmt.Sprintf(" Son : %s", sl.String())
	b.WriteString(bar1)
	if m.cfg.SoundEnabled {
		b.WriteString(line1)
	} else {
		b.WriteString(dimStyle.Render(line1))
	}
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("esc fermer · j/k nav · espace/enter toggle"))
	return b.String()
}
```

- [ ] **Step 5 : Mettre à jour `Update()` pour gérer les touches du panneau options**

Remplacer le bloc `case tea.KeyMsg:` entier dans `Update()` par :

```go
case tea.KeyMsg:
	if m.options {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.optCursor < 1 {
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
					sounds := []string{"glass", "ping", "funk"}
					for i, s := range sounds {
						if s == m.cfg.SoundName {
							m.cfg.SoundName = sounds[(i+1)%len(sounds)]
							break
						}
					}
				}
			}
		case "esc":
			m.options = false
			config.Save(m.cfg) //nolint:errcheck
		}
		return m, nil
	}
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
	case "o":
		if !m.detail {
			m.options = true
		}
	case "enter":
		m.detail = !m.detail
	case "esc":
		m.detail = false
	}
```

- [ ] **Step 6 : Mettre à jour la ligne d'aide dans `renderList()`**

Remplacer :

```go
b.WriteString(dimStyle.Render("j/k nav · enter detail · a all/open · r refresh · q quit"))
```

par :

```go
b.WriteString(dimStyle.Render("j/k nav · enter detail · o options · a all/open · r refresh · q quit"))
```

- [ ] **Step 7 : Vérifier la compilation**

```bash
go build ./...
```

Résultat attendu : aucune erreur.

- [ ] **Step 8 : Commit**

```bash
git add internal/tui/model.go
git commit -m "feat(tui): add Options panel with sound toggle and sound selector"
```

---

## Task 5 — Câblage son sur les transitions de statut

**Files:**
- Modify: `internal/tui/model.go`

- [ ] **Step 1 : Ajouter l'import audio dans `model.go`**

Ajouter dans le bloc import :

```go
"github.com/ludo/claudewatcher/internal/audio"
```

- [ ] **Step 2 : Mettre à jour le handler `sessionsMsg` dans `Update()`**

Remplacer le bloc `case sessionsMsg:` existant :

```go
case sessionsMsg:
	m.err = msg.err
	m.sessions = sortSessions(msg.sessions)
	if m.cursor >= len(m.sessions) {
		m.cursor = len(m.sessions) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
```

par :

```go
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
```

- [ ] **Step 3 : Vérifier la compilation et les tests**

```bash
go build ./... && go test ./...
```

Résultat attendu : compilation propre, tous les tests passent.

- [ ] **Step 4 : Tester manuellement**

```bash
go run ./cmd/cw/
```

- Appuyer sur `o` → panneau Options s'ouvre
- `j`/`k` → curseur se déplace entre Activé et Son
- `espace` sur Activé → `[ ]` devient `[x]`
- `enter` sur Son → cycle Glass → Ping → Funk → Glass
- `esc` → panneau se ferme, config sauvegardée dans `~/.config/claudewatcher/config.json`
- Vérifier le fichier : `cat ~/.config/claudewatcher/config.json`
- Relancer l'app → les préférences sont restaurées

- [ ] **Step 5 : Compiler le binaire**

```bash
go build -o cw ./cmd/cw/
```

- [ ] **Step 6 : Commit final**

```bash
git add internal/tui/model.go
git commit -m "feat(tui): trigger sound on session status transitions"
```
