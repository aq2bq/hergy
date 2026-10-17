package effect

import (
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Preview is the "保存すると" lines of design 4: one line for each party, worker
// or part a change moves, or the one line that says no member's prompt moves.
// The member and the party are named as a person reads them (the short member
// name and its role), not by herdr name.
func Preview(changes []Change) []string {
	var lines []string
	for _, change := range changes {
		for _, part := range change.Parts {
			lines = append(lines, previewPart(change.Member, part)...)
		}
	}
	if len(lines) == 0 {
		return []string{i18n.T("effect.preview.none")}
	}
	return lines
}

// previewPart is the lines one part of one member's prompt gets. A part that
// names things (the turns, the workers) gets one line each, so a line names one
// party or one worker.
func previewPart(member string, part PartChange) []string {
	switch part.Part {
	case Turns:
		var out []string
		for _, n := range part.Removed {
			out = append(out, turnLine(member, n, true))
		}
		for _, n := range part.Added {
			out = append(out, turnLine(member, n, false))
		}
		return out
	case Workers:
		var out []string
		for _, n := range part.Removed {
			out = append(out, i18n.Tf("effect.preview.worker_removed", member, n.Name))
		}
		for _, n := range part.Added {
			out = append(out, i18n.Tf("effect.preview.worker_added", member, n.Name))
		}
		for _, n := range part.Changed {
			out = append(out, i18n.Tf("effect.preview.worker_profile", member, n.Name))
		}
		return out
	case Roster:
		return []string{i18n.Tf("effect.preview.roster", member)}
	case RoleDocument:
		return []string{i18n.Tf("effect.preview.role_document", member)}
	case Agreements:
		return []string{i18n.Tf("effect.preview.agreements", member)}
	case Launch:
		return []string{i18n.Tf("effect.preview.launch", member)}
	case Env:
		return []string{i18n.Tf("effect.preview.env", member)}
	default:
		return nil
	}
}

// turnLine is one turn's line. A party the prompt names with a role (a member)
// gets its role in brackets; the human, which has none, gets the human's own
// name.
func turnLine(member string, n Named, removed bool) string {
	if n.Role == "" {
		if removed {
			return i18n.Tf("effect.preview.turns_removed_human", member)
		}
		return i18n.Tf("effect.preview.turns_added_human", member)
	}
	if removed {
		return i18n.Tf("effect.preview.turns_removed_member", member, n.Name, n.Role)
	}
	return i18n.Tf("effect.preview.turns_added_member", member, n.Name, n.Role)
}

// Running is one team that is running now: the name it goes by, and the
// definition it was booted from (team.Running, the same two fields). effect
// keeps its own shape so it imports no top-layer package; the web UI maps
// team.Running onto it.
type Running struct {
	Team       string
	Definition string
}

// Handover is what to do so a running team takes a change up, one line per
// running team (design 3.0 の表). No running team gives the one line that says
// the next hergy up uses the change. The commands are the real ones: hergy
// reprompt (the whole team, or one member), hergy refresh for a launch change,
// and hergy down then hergy up for an env change.
func Handover(changes []Change, running []Running) []string {
	if len(running) == 0 {
		return []string{i18n.T("effect.handover.none_running")}
	}
	out := make([]string, 0, len(running))
	for _, team := range running {
		out = append(out, handoverTeam(changes, team))
	}
	return out
}

// handoverTeam is one running team's line. A change to the roster, a role
// document or an agreements reaches every member, so one reprompt --all covers
// them; otherwise only the members whose turns or workers moved are reprompted.
// A launch change is a refresh (the conversation is lost) and an env change is a
// down and up (env reaches a pane only when the pane is made).
func handoverTeam(changes []Change, team Running) string {
	var (
		all       bool
		members   []string
		refreshed []string
		env       bool
	)
	for _, change := range changes {
		for _, part := range change.Parts {
			switch part.Part {
			case Launch:
				refreshed = appendUnique(refreshed, change.Member)
			case Env:
				env = true
			case Roster, RoleDocument, Agreements:
				all = true
			case Turns, Workers:
				members = appendUnique(members, change.Member)
			}
		}
	}
	var lines []string
	if all {
		lines = append(lines, i18n.Tf("effect.handover.reprompt_all", team.Team))
	} else {
		for _, member := range members {
			lines = append(lines, i18n.Tf("effect.handover.reprompt_member", member, team.Team))
		}
	}
	for _, member := range refreshed {
		lines = append(lines, i18n.Tf("effect.handover.refresh_member", member, team.Team))
	}
	if env {
		lines = append(lines, i18n.Tf("effect.handover.down_up", team.Team, team.Definition, team.Team))
	}
	if len(lines) == 0 {
		return i18n.T("effect.handover.nothing")
	}
	return strings.Join(lines, i18n.T("list.separator"))
}

// appendUnique adds a name once, keeping the order the members come in.
func appendUnique(list []string, name string) []string {
	if slices.Contains(list, name) {
		return list
	}
	return append(list, name)
}
