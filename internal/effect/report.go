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
//
// The roster is every member's line when it moves, so when it moves for all of
// them one line says so instead of one line per member (T1).
func Preview(changes []Change) []string {
	allRoster := len(changes) > 1
	for _, change := range changes {
		if !hasPart(change.Parts, Roster) {
			allRoster = false
		}
	}
	var lines []string
	rosterSaid := false
	for _, change := range changes {
		for _, part := range change.Parts {
			if part.Part == Roster && allRoster {
				if !rosterSaid {
					lines = append(lines, i18n.T("effect.preview.roster_all"))
					rosterSaid = true
				}
				continue
			}
			lines = append(lines, previewPart(change.Member, part)...)
		}
	}
	if len(lines) == 0 {
		return []string{i18n.T("effect.preview.none")}
	}
	return lines
}

// hasPart is whether a member's parts hold a part.
func hasPart(parts []PartChange, part Part) bool {
	for _, p := range parts {
		if p.Part == part {
			return true
		}
	}
	return false
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

// Command is one hergy command a handover holds, and the note that goes with
// it: Line is the command alone ("hergy refresh lead --team demo"), so a screen
// can put it on a copy button, and Note is the sentence after it ("会話は消えます"),
// empty when there is none. A down and an up are two commands, in the order to
// type them.
type Command struct {
	Line string
	Note string
}

// TeamCommands is what to type for one running team: the team's name, and its
// commands in the order to type them.
type TeamCommands struct {
	Team     string
	Commands []Command
}

// Commands is the commands to type so each running team takes the change up,
// one entry per running team (I16). A change is used only for the team whose
// definition it belongs to: a change to another definition's team.yaml is not
// this team's to take up. A team with nothing to take up holds no commands.
func Commands(changes []Change, running []Running) []TeamCommands {
	if len(running) == 0 {
		return nil
	}
	out := make([]TeamCommands, 0, len(running))
	for _, team := range running {
		out = append(out, TeamCommands{Team: team.Team, Commands: teamCommands(changesOfDefinition(changes, team.Definition), team)})
	}
	return out
}

// changesOfDefinition is the changes that belong to one definition.
func changesOfDefinition(changes []Change, definition string) []Change {
	var out []Change
	for _, change := range changes {
		if change.Definition == definition {
			out = append(out, change)
		}
	}
	return out
}

// Handover is what to do so a running team takes a change up, one line per
// running team (design 3.0 の表). No running team gives the one line that says
// the next hergy up uses the change. The lines are made from Commands.
func Handover(changes []Change, running []Running) []string {
	if len(running) == 0 {
		return []string{i18n.T("effect.handover.none_running")}
	}
	out := make([]string, 0, len(running))
	for _, team := range Commands(changes, running) {
		if len(team.Commands) == 0 {
			out = append(out, i18n.T("effect.handover.nothing"))
			continue
		}
		out = append(out, renderCommands(team.Commands))
	}
	return out
}

// renderCommands is a team's commands as one line: a command and its note, the
// commands separated by the list separator, except a down and the up after it,
// which the sentence joins with "のあと".
func renderCommands(commands []Command) string {
	var b strings.Builder
	for i, command := range commands {
		if i > 0 {
			if afterTheOther(commands[i-1], command) {
				b.WriteString(i18n.T("effect.handover.after"))
			} else {
				b.WriteString(i18n.T("list.separator"))
			}
		}
		b.WriteString(command.Line)
		if command.Note != "" {
			b.WriteString(i18n.Tf("effect.handover.note", command.Note))
		}
	}
	return b.String()
}

// afterTheOther is whether two commands are the pair a person types one after
// the other, hergy down then hergy up. The command words are hergy's own and
// are not translated, so they are read off the line.
func afterTheOther(previous, next Command) bool {
	return strings.HasPrefix(previous.Line, "hergy down") && strings.HasPrefix(next.Line, "hergy up")
}

// teamCommands is one running team's commands. A change to the roster, a role
// document or an agreements reaches every member, so one reprompt --all covers
// them; otherwise only the members whose turns or workers moved are reprompted.
// A launch change is a refresh (the conversation is lost), and a member that is
// refreshed gets its bootstrap prompt again, so it is not also reprompted and
// the refresh comes first (T3). An env change is a down and up (env reaches a
// pane only when the pane is made), which starts everyone afresh, so nothing
// else is put beside it.
func teamCommands(changes []Change, team Running) []Command {
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
	if env {
		return []Command{
			{Line: i18n.Tf("effect.handover.down", team.Team)},
			{Line: i18n.Tf("effect.handover.up", team.Definition, team.Team)},
		}
	}
	var commands []Command
	for _, member := range refreshed {
		commands = append(commands, Command{
			Line: i18n.Tf("effect.handover.refresh_member", member, team.Team),
			Note: i18n.T("effect.handover.refresh_note"),
		})
	}
	if all {
		commands = append(commands, Command{Line: i18n.Tf("effect.handover.reprompt_all", team.Team)})
	} else {
		for _, member := range members {
			if slices.Contains(refreshed, member) {
				continue
			}
			commands = append(commands, Command{Line: i18n.Tf("effect.handover.reprompt_member", member, team.Team)})
		}
	}
	return commands
}

// appendUnique adds a name once, keeping the order the members come in.
func appendUnique(list []string, name string) []string {
	if slices.Contains(list, name) {
		return list
	}
	return append(list, name)
}
