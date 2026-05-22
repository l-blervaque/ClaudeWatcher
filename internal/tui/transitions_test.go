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
