// Package status holds what hergy status shows: one row per member or worker
// with its live state, and the judgement over those rows. The CLI and the WebUI
// both print the same thing, so the rows and the words live here rather than in
// internal/app. Nothing here reads Herdr; the caller fills the rows.
package status

import (
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Row is one member or worker of a team: the name herdr knows it by, the state
// herdr reports for its agent, whether an agent is live under that name, and
// whether the row is a hired worker. Live is false for a member that has no
// agent, so the caller can print it as absent.
//
// Title, PaneID, Role and Kind are what hergy status does not print but the
// WebUI's running-team screen shows: what the agent's terminal says it is
// doing, the pane it runs in, the role a member reads, and the engine (claude,
// opencode) behind it.
//
// Screen and UnchangedMinutes are hergy's own reading of a working member, not
// herdr's: Screen is the hash of the member's terminal (herdr agent read,
// detection source), empty for a member with no live agent and for workers, and
// UnchangedMinutes says how long that screen has stood while the member worked,
// nil when it changed or hergy had not seen it before.
type Row struct {
	Name             string
	State            string
	Live             bool
	Worker           bool
	Title            string
	PaneID           string
	Role             string
	Kind             string
	Screen           string
	UnchangedMinutes *int
}

// Text is one row's state, as hergy status prints it: the live state with the
// word that state carries ("working" that has stood for a while, "blocked" that
// is waiting for permission), "not present" when no agent is live, and the
// worker mark on a hired worker.
func (r Row) Text() string {
	state := r.State
	switch {
	case !r.Live:
		state = i18n.T("status.not_live")
	case r.State == "working" && r.UnchangedMinutes != nil:
		state = i18n.Tf("status.unchanged", *r.UnchangedMinutes)
	case r.State == "blocked":
		state += i18n.T("status.blocked_suffix")
	}
	if r.Worker {
		state += i18n.T("status.worker_suffix")
	}
	return state
}

// Verdict is the judgement over a team's rows: the names waiting on the human,
// and how many are working. Only rows with a live agent count.
type Verdict struct {
	Waiting []string
	Working int
}

// Text is the judgement line, empty when the team is working with nobody
// waiting: the names waiting on the human, else that nobody is working.
func (v Verdict) Text() string {
	switch {
	case len(v.Waiting) > 0:
		return i18n.Tf("status.human_waiting", strings.Join(v.Waiting, i18n.T("list.separator")))
	case v.Working == 0:
		return i18n.T("status.nobody_working")
	}
	return ""
}

// Judge reads the judgement from the rows. A row with no live agent is left
// out.
func Judge(rows []Row) Verdict {
	v := Verdict{}
	for _, row := range rows {
		if !row.Live {
			continue
		}
		switch row.State {
		case "working":
			v.Working++
		case "blocked":
			v.Waiting = append(v.Waiting, row.Name)
		}
	}
	return v
}

// Counts is how many of the rows are members and how many are workers.
func Counts(rows []Row) (members, workers int) {
	for _, row := range rows {
		if row.Worker {
			workers++
		} else {
			members++
		}
	}
	return members, workers
}
