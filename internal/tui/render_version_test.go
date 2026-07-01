package tui

import (
	"strings"
	"testing"

	"github.com/ludo/claudewatcher/internal/config"
	"github.com/ludo/claudewatcher/internal/session"
)

// TestRenderListWideShowsCLIVersion verifies the CLI column header and a
// session's version render in the wide layout when ShowVersion is enabled,
// and disappear when it is off.
func TestRenderListWideShowsCLIVersion(t *testing.T) {
	m := Model{
		width:  120,
		height: 20,
		cfg:    config.Config{ShowModel: true, ShowVersion: true},
		sessions: []session.Session{
			{ID: "a", ProjectName: "proj", Title: "hello", Status: session.StatusActive,
				Model: "claude-opus-4-8", Version: "2.1.197"},
		},
	}

	out := m.renderListWide()
	if !strings.Contains(out, "CLI") {
		t.Errorf("wide layout missing CLI header:\n%s", out)
	}
	if !strings.Contains(out, "2.1.197") {
		t.Errorf("wide layout missing version value:\n%s", out)
	}

	m.cfg.ShowVersion = false
	out = m.renderListWide()
	if strings.Contains(out, "2.1.197") {
		t.Errorf("version shown despite ShowVersion=false:\n%s", out)
	}
}
