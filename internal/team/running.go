package team

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/protocol"
)

// Running is a team herdr reports as running: the name it runs under (which is
// what its tab is labelled), the team definition it was booted from, and the
// project the launch record it was read from names, when the record names one.
type Running struct {
	Team       string
	Definition string
	Project    string
}

// Tab is one of herdr's tabs as telling hergy's own tabs apart needs it: its id,
// the label on it, the workspace it stands in, the names of the live agents in
// it, and the names hergy put on its panes. hergy names a member's agent and its
// pane "<team>-<member>", so the agents and panes in a tab say whose team the tab
// is, even when hergy has no launch record of it (d-a72d), and even when no agent
// is running there yet (d-da92).
type Tab struct {
	ID          string
	Label       string
	WorkspaceID string
	Agents      []string
	// Panes are the labels of the panes hergy named in this tab. A pane a person
	// opened has no label.
	Panes []string
	// PaneDirs is where those panes stand, one per Pane. A team this machine has
	// no launch record of is read as the project its panes stand in (d-73fc).
	PaneDirs []string
}

// Launch is one team this machine recorded a launch of: the name it ran under,
// the definition it was booted from, and the project it ran in. The project is
// what tells two projects' teams of one name apart, so a team is read from the
// record of its own project rather than the newest of its name (r-87c8).
type Launch struct {
	Team       string
	Definition string
	Project    string
}

// RunningTeams reads herdr's tabs and this machine's launch record, and returns
// the running teams, in herdr's order. A team name seen twice is kept once, and
// a tab that is nobody's team is skipped.
//
// A tab is one of hergy's when a live agent or a named pane in it is named after
// the tab's label ("<label>-<member>"), which is how a team is still found after
// a boot that failed once the tab was made and left no record, and after every
// agent ended (d-da92), or when the label still has the older shape
// "team:<team>@<definition>" or "team:<team>", which carried the definition
// itself. Another tab is not, even one a person labelled with the name
// of a team that has stopped: the launch record is history, so it says where a
// definition is read from, not which tab is a team's.
//
// agentsRead says whether the live agents could be read. When they could not,
// there is nothing to tell a team's tab from a person's tab with the same name,
// and a label this machine recorded a launch of is taken as a team's rather than
// losing the team.
//
// The definition comes from the record of that team in the project its tab stands
// in, and from the newest record of its name when the tab does not say where it
// stands; the older label is the fallback for a team this machine has no record
// of, and the team's own name is the last one (a team booted without --as runs
// from the definition of its name).
func RunningTeams(tabs []Tab, launches []Launch, agentsRead bool) []Running {
	known := newLaunches(launches)
	seen := map[string]bool{}
	var out []Running
	for _, tab := range tabs {
		name, definition, project, ok := runningOf(tab, known, agentsRead)
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, Running{Team: name, Definition: definition, Project: project})
	}
	return out
}

// launches is this machine's launch record as reading one tab needs it: every
// record of a team name, oldest first. Reading one tab means choosing among them
// by the project the tab stands in, so the name alone is not the key (r-87c8).
type launches struct {
	byTeam map[string][]Launch
}

// newLaunches groups the record by team name, oldest first, the order it was read
// in.
func newLaunches(all []Launch) *launches {
	out := &launches{byTeam: make(map[string][]Launch, len(all))}
	for _, launch := range all {
		if launch.Team != "" && launch.Definition != "" {
			out.byTeam[launch.Team] = append(out.byTeam[launch.Team], launch)
		}
	}
	return out
}

// forTab is the record behind one tab. A tab whose panes hergy named says which
// project the team runs in, so the record of that project and name is the one
// read: another project's team of the same name is a different team (r-87c8). A
// tab whose named panes stand in no recorded project has no record — a boot that
// failed once the tab was made leaves the panes and no record — and a tab with no
// named pane to read stands on the newest record of its name, as it always has.
func (l *launches) forTab(name string, tab Tab) (Launch, bool) {
	records := l.byTeam[name]
	if len(records) == 0 {
		return Launch{}, false
	}
	dirs := namedPaneDirs(name, tab)
	if len(dirs) > 0 {
		for _, record := range slices.Backward(records) {
			if record.Project != "" && underAny(record.Project, dirs) {
				return record, true
			}
		}
		return Launch{}, false
	}
	return records[len(records)-1], true
}

// namedPaneDirs is where the panes hergy named for one team in this tab stand, in
// the order of the panes. A pane a person opened has no name of the team's, and
// the tab's PaneDirs pair with its Panes by position.
func namedPaneDirs(name string, tab Tab) []string {
	var out []string
	for i, pane := range tab.Panes {
		if !protocol.NamedForTeam(name, pane) || i >= len(tab.PaneDirs) {
			continue
		}
		out = append(out, tab.PaneDirs[i])
	}
	return out
}

// underAny reports whether one of the directories stands in the project.
func underAny(project string, dirs []string) bool {
	for _, dir := range dirs {
		if rel, err := filepath.Rel(project, dir); err == nil &&
			rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// runningOf reads one tab: the team it stands for, the definition behind it, and
// the project the record it was read from names. known is the launch record and
// agentsRead says whether the live agents could be read.
func runningOf(tab Tab, known *launches, agentsRead bool) (name, definition, project string, ok bool) {
	if rest, older := strings.CutPrefix(tab.Label, "team:"); older {
		name, definition, ok = splitTeamLabel(rest)
		if !ok {
			return "", "", "", false
		}
		if recorded, found := known.forTab(name, tab); found {
			definition, project = recorded.Definition, recorded.Project
		}
		return name, definition, project, true
	}
	if tab.Label == "" {
		return "", "", "", false
	}
	recorded, found := known.forTab(tab.Label, tab)
	if !HoldsMember(tab, tab.Label) {
		// The record alone does not make a tab a team's: it is the history of
		// what this machine started, and a person's tab can carry the name of a
		// team that has stopped. Only when the agents could not be read at all
		// is the record the best guess left.
		if !agentsRead && found {
			return tab.Label, recorded.Definition, recorded.Project, true
		}
		return "", "", "", false
	}
	if !found {
		return tab.Label, tab.Label, "", true
	}
	return tab.Label, recorded.Definition, recorded.Project, true
}

// HoldsMember reports whether a tab holds a member of the team of this name: an
// agent or a pane hergy named "<team>-<member>". An agent that is not running
// leaves its named pane behind, and that pane is what still says the tab is the
// team's (d-da92). It is what tells a team's tab from another tab that carries
// the same label.
func HoldsMember(tab Tab, name string) bool {
	for _, agent := range tab.Agents {
		if protocol.NamedForTeam(name, agent) {
			return true
		}
	}
	for _, pane := range tab.Panes {
		if protocol.NamedForTeam(name, pane) {
			return true
		}
	}
	return false
}

// MemberAgents is the live agents hergy started for one team: the agents named
// "<team>-<member>" that stand in a tab labelled for that team (the team's name
// alone, or the older "team:<team>@<definition>" or "team:<team>" shape). Another
// team whose name merely starts the same way ("qw2-x" beside "qw2") keeps its own
// tab, so its agents are not here, and neither are the agents of a person's tab
// that happens to carry the name.
func MemberAgents(tabs []Tab, name string) []string {
	var out []string
	for _, tab := range tabs {
		if !protocol.NamesTeam(tab.Label, name) {
			continue
		}
		for _, agent := range tab.Agents {
			if protocol.NamedForTeam(name, agent) {
				out = append(out, agent)
			}
		}
	}
	return out
}

// MemberPanes is the panes hergy named for one team in the tab labelled for it:
// the panes that are still there after their agent ended (d-da92).
func MemberPanes(tabs []Tab, name string) []string {
	var out []string
	for _, tab := range tabs {
		if !protocol.NamesTeam(tab.Label, name) {
			continue
		}
		for _, pane := range tab.Panes {
			if protocol.NamedForTeam(name, pane) {
				out = append(out, pane)
			}
		}
	}
	return out
}

// splitTeamLabel reads the older "<team>@<definition>" or "<team>". The older
// form names the team definition after the team.
func splitTeamLabel(rest string) (teamName, definition string, ok bool) {
	if before, after, ok0 := strings.Cut(rest, "@"); ok0 {
		teamName, definition = before, after
		if teamName == "" || definition == "" {
			return "", "", false
		}
		return teamName, definition, true
	}
	if rest == "" {
		return "", "", false
	}
	return rest, rest, true
}

// FindRunning returns the running team with this name.
func FindRunning(running []Running, name string) (Running, bool) {
	for _, team := range running {
		if team.Team == name {
			return team, true
		}
	}
	return Running{}, false
}

// RunningNames is the team names, in order.
func RunningNames(running []Running) []string {
	names := make([]string, len(running))
	for i, team := range running {
		names[i] = team.Team
	}
	return names
}
