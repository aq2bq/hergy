package team

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
)

// Running is a registered run with an exactly matching tab in herdr. State
// supplies the definition and project even while the tab is still empty.
type Running struct {
	State      runningstate.State
	Team       domainname.TeamRef
	Definition string
	Project    domainplace.Project
}

// Tab is herdr's tab and the agents, named panes and working directories read
// with it. Membership and cleanup use the contents; identity uses its label.
type Tab struct {
	ID          string
	Label       domainname.Seen
	WorkspaceID string
	Agents      []domainname.Seen
	// Panes are the labels of the panes hergy named in this tab. A pane a person
	// opened has no label.
	Panes []domainname.Seen
	// PaneDirs is where the panes stand, one directory per named pane. It is
	// used for leftover worktree reporting, never for team identity.
	PaneDirs []domainplace.Seen
}

// RunningTeams intersects registered run state with exact tab labels. A new
// tab belongs to its run even before a pane is named or an agent starts.
func RunningTeams(tabs []Tab, states []runningstate.State) []Running {
	byName := make(map[string]runningstate.State, len(states))
	for _, state := range states {
		byName[state.Name().String()] = state
	}
	seen := map[string]bool{}
	var out []Running
	for _, tab := range tabs {
		state, exists := byName[tab.Label.String()]
		if !exists || seen[tab.Label.String()] {
			continue
		}
		launch := state.Launch()
		seen[tab.Label.String()] = true
		out = append(out, Running{Team: state.Name().Ref(), Definition: launch.Definition.String(), Project: launch.Project, State: state})
	}
	return out
}

// MemberAgents finds live members in the tab whose label exactly names the run.
func MemberAgents(tabs []Tab, name domainname.Seen) []domainname.Seen {
	var out []domainname.Seen
	for _, tab := range tabs {
		if tab.Label.String() != name.String() {
			continue
		}
		for _, agent := range tab.Agents {
			if domainname.BelongsTo(agent, name) {
				out = append(out, agent)
			}
		}
	}
	return out
}

// MemberPanes is the panes hergy named for one team in the tab labelled for it:
// the panes that are still there after their agent ended (d-da92).
func MemberPanes(tabs []Tab, name domainname.Seen) []domainname.Seen {
	var out []domainname.Seen
	for _, tab := range tabs {
		if tab.Label.String() != name.String() {
			continue
		}
		for _, pane := range tab.Panes {
			if domainname.BelongsTo(pane, name) {
				out = append(out, pane)
			}
		}
	}
	return out
}

// FindRunning returns the running team with this name.
func FindRunning(running []Running, name string) (Running, bool) {
	for _, team := range running {
		if team.Team.String() == name {
			return team, true
		}
	}
	return Running{}, false
}

// RunningNames is the team names, in order.
func RunningNames(running []Running) []string {
	names := make([]string, len(running))
	for i, team := range running {
		names[i] = team.Team.String()
	}
	return names
}
