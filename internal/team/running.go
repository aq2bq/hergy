package team

import "strings"

// Running is a team herdr reports as running: the name it runs under (which is
// what its tab is labelled) and the team definition it was booted from.
type Running struct {
	Team       string
	Definition string
}

// Tab is one of herdr's tabs as telling hergy's own tabs apart needs it: the
// label on it, and the names of the live agents in it. hergy names a member's
// agent "<team>-<member>", so the agents in a tab say which team the tab is,
// even when hergy has no launch record of it (d-a72d).
type Tab struct {
	Label  string
	Agents []string
}

// Launch is one team this machine recorded a launch of: the name it ran under
// and the definition it was booted from. It is where a running team's definition
// is read from, now that the tab's label is only the team's name (d-a72d).
type Launch struct {
	Team       string
	Definition string
}

// RunningTeams reads herdr's tabs and this machine's launch record, and returns
// the running teams, in herdr's order. A team name seen twice is kept once, and
// a tab that is nobody's team is skipped.
//
// A tab is one of hergy's when a live agent in it is named after the tab's label
// ("<label>-<member>"), which is how a team is still found after a boot that
// failed once the tab was made and left no record, or when the label still has
// the older shape "team:<team>@<definition>" or "team:<team>", which carried the
// definition itself. A tab whose label hergy recorded a launch of is hergy's
// too. Another tab, with another tool's agents in it, is not.
//
// The definition comes from the newest record of that team; the older label is
// the fallback for a team this machine has no record of, and the team's own name
// is the last one (a team booted without --as runs from the definition of its
// name).
func RunningTeams(tabs []Tab, launches []Launch) []Running {
	known := latestDefinitions(launches)
	seen := map[string]bool{}
	var out []Running
	for _, tab := range tabs {
		name, definition, ok := runningOf(tab, known)
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, Running{Team: name, Definition: definition})
	}
	return out
}

// latestDefinitions is the newest launch of each team name, by name.
func latestDefinitions(launches []Launch) map[string]string {
	out := make(map[string]string, len(launches))
	for _, launch := range launches {
		if launch.Team != "" && launch.Definition != "" {
			out[launch.Team] = launch.Definition
		}
	}
	return out
}

// runningOf reads one tab: the team it stands for, and the definition behind it.
// known is the launch record, by team name.
func runningOf(tab Tab, known map[string]string) (name, definition string, ok bool) {
	if rest, older := strings.CutPrefix(tab.Label, "team:"); older {
		name, definition, ok = splitTeamLabel(rest)
		if !ok {
			return "", "", false
		}
		if recorded, found := known[name]; found {
			definition = recorded
		}
		return name, definition, true
	}
	if tab.Label == "" {
		return "", "", false
	}
	if recorded, found := known[tab.Label]; found {
		return tab.Label, recorded, true
	}
	if !holdsMember(tab) {
		return "", "", false
	}
	return tab.Label, tab.Label, true
}

// holdsMember says whether a tab holds an agent hergy started for the team the
// tab is labelled after.
func holdsMember(tab Tab) bool {
	for _, agent := range tab.Agents {
		if strings.HasPrefix(agent, tab.Label+"-") {
			return true
		}
	}
	return false
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
