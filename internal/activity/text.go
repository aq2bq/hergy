package activity

import (
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/words"
)

// Text is the label a person reads in hergy status.
func (s State) Text() string { return s.Term().Text(i18n.English) }

// Term is the vocabulary ID the activity API gives the screen.
func (s State) Term() words.TermID {
	switch s {
	case WaitingAnswer:
		return words.TermWaitingForPermission
	case WaitingTurn:
		return words.TermHanded
	case Stalled:
		return words.TermStalled
	default:
		return words.TermWorking
	}
}

// Term is the vocabulary ID for a member's or worker's state.
func (s NodeState) Term() words.TermID {
	switch s {
	case Blocked:
		return words.TermWaitingForPermission
	case HandedOver:
		return words.TermHanded
	case Active:
		return words.TermWorking
	case Idle:
		return words.TermIdle
	default:
		return words.TermNotRunning
	}
}

// Slug is one node's state in the running-team state summaries: blocked,
// handed, active, idle or absent.
func (s NodeState) Slug() string {
	switch s {
	case Blocked:
		return "blocked"
	case HandedOver:
		return "handed"
	case Active:
		return "active"
	case Idle:
		return "idle"
	default:
		return "absent"
	}
}
