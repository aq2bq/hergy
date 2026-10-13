package team

import "strings"

// Running is a team herdr reports as running: the team name it runs under and
// the team definition name (the teams/<name>/ directory) it was booted from.
// hergy reads both from the team's tab label, "team:<team>@<definition>", or the
// older "team:<team>", where the team definition name is the team name.
type Running struct {
	Team       string
	Definition string
}

// RunningTeams reads the tab labels herdr reports and returns the running
// teams, in the order herdr gave them. A label that is not a team tab, or that
// names an empty team or definition, is skipped; a team name seen twice is kept
// once.
func RunningTeams(labels []string) []Running {
	seen := map[string]bool{}
	var out []Running
	for _, label := range labels {
		rest, ok := strings.CutPrefix(label, "team:")
		if !ok || rest == "" {
			continue
		}
		teamName, definition, ok := splitTeamLabel(rest)
		if !ok || seen[teamName] {
			continue
		}
		seen[teamName] = true
		out = append(out, Running{Team: teamName, Definition: definition})
	}
	return out
}

// splitTeamLabel reads "<team>@<definition>" or the older "<team>". The older
// form names the team definition after the team.
func splitTeamLabel(rest string) (teamName, definition string, ok bool) {
	if at := strings.IndexByte(rest, '@'); at >= 0 {
		teamName, definition = rest[:at], rest[at+1:]
		if teamName == "" || definition == "" {
			return "", "", false
		}
		return teamName, definition, true
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
