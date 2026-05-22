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
