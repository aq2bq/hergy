package ops

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/check"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/source"
)

// RunningTeam is one running team with its team definition built: what hergy
// status lists and what the activity screen shows. Broken says why a definition
// that cannot be built could not be, in the words hergy status writes; the team
// is still listed then, so one such team does not hide the rest.
type RunningTeam struct {
	state runningstate.State
	// Name is the team's name: the label on its tab in herdr.
	Name domainname.Seen
	// Definition is the team definition named in the saved run state. It differs from Name for a team booted with --as.
	Definition string
	// Built is the built definition: its graph, its members and its workers.
	// nil when the definition cannot be built.
	Built *Resolved
	// Broken is why the definition cannot be built, as hergy status's 詳細 line
	// reads. Empty when it builds.
	Broken string
	// Elsewhere says the definition is not in its project's catalog at all.
	Elsewhere bool
	// Project is the registered project. Catalog is the reading of that
	// project used by the delivery check and the opened part.
	Project domainplace.Project
	// Place is the definition's layer: "local" for a project's definition,
	// "global" for one started in a project from the global layer.
	Place       domainplace.Place
	ProjectName string
	Catalog     *catalog.Catalog
	// Workspace is the workspace saved when this run began.
	WorkspaceID    string
	WorkspaceLabel string
	// TabID is the tab's id, for the answer location.
	TabID string
}

// readRunningTeams reads this machine's running teams and builds each one's team
// definition. It is the one reading hergy status and the activity screen call
// (6 A1), so the two cannot disagree about which teams are running, which
// definition a team was booted from, or why one's definition does not read.
//
// The reading of Herdr is handed in rather than done here, so one snapshot can
// be used for all the team lookups in this answer.
func readRunningTeams(env Environment, read session) []RunningTeam {
	cat := env.Catalog(source.Disk{})
	view := ProjectView{Path: env.ProjectRoot, Name: filepath.Base(env.ProjectRoot.String()), Env: env, Cat: cat}
	views := []ProjectView{view}
	seen := map[domainplace.Project]bool{env.ProjectRoot: true}
	for _, state := range currentTeamStates() {
		project := state.Launch().Project
		if seen[project] {
			continue
		}
		seen[project] = true
		other, err := env.AtProject(project)
		if err != nil {
			continue
		}
		views = append(views, ProjectView{Path: project, Name: filepath.Base(project.String()), Env: other, Cat: other.Catalog(source.Disk{})})
	}
	return readRunningTeamsIn(views, read, true)
}

// ProjectView is one project the activity reads: its root, the name a person
// reads, the environment its layer lives in, and the catalog over that layer.
type ProjectView struct {
	Path domainplace.Project
	Name string
	Env  Environment
	Cat  *catalog.Catalog
}

// readRunningTeamsIn keeps the runs whose registered projects are visible.
// Herdr supplies tab existence; state supplies each run's provenance.
func readRunningTeamsIn(views []ProjectView, read session, _ bool) []RunningTeam {
	running := team.RunningTeams(read.Tabs, currentTeamStates())
	out := make([]RunningTeam, 0, len(running))
	for _, entry := range running {
		view, ok := projectOf(entry, views)
		if !ok {
			continue
		}
		one := RunningTeam{
			Name: entry.Team.Observation(), Definition: entry.Definition, state: entry.State,
			Project: view.Path, ProjectName: view.Name, Catalog: catalogOfRun(view.Cat, entry.State.Launch()),
			Place: entry.State.Launch().Layer.At(view.Path),
		}
		if tab, found := teamTab(read.Tabs, entry.Team.Observation()); found {
			one.TabID = tab.ID
			one.WorkspaceID = entry.State.Launch().Workspace.ID
			one.WorkspaceLabel = entry.State.Launch().Workspace.Label
		}
		r, err := buildRunningIn(one.Catalog, view.Env.ProjectRoot, entry, entry.State.Launch().Layer.String())
		if err != nil {
			// A definition that does not read is not this reading's failure: the
			// row says why in the definition's own words (2.2), as hergy status
			// says it, so the reason is kept on the team rather than returned.
			one.Broken = brokenTeamLine(view.Env, err)
			one.Elsewhere = errors.Is(err, catalog.ErrNotFound)
			out = append(out, one)
			continue
		}
		one.Built = r
		one.Place = domainplace.Selection(r.Team.Definition.Layer).At(view.Path)
		out = append(out, one)
	}
	return out
}

// projectOf selects only the project registered by this run.
func projectOf(entry team.Running, views []ProjectView) (ProjectView, bool) {
	for _, view := range views {
		if entry.Project.Compare(view.Path) == domainplace.Same {
			return view, true
		}
	}
	return ProjectView{}, false
}

// teamTab is the tab labelled for one team, with the workspace it stands in.
func teamTab(tabs []team.Tab, name domainname.Seen) (team.Tab, bool) {
	for _, tab := range tabs {
		if tab.Label.String() == name.String() {
			return tab, true
		}
	}
	return team.Tab{}, false
}

// brokenTeamLine is a definition read failure as the status result presents it.
func brokenTeamLine(env Environment, err error) string {
	if readErr, ok := errors.AsType[*catalog.ReadError](err); ok {
		return check.BreakLine(env.Display, readErr.Path, err)
	}
	return strings.Join(strings.Fields(err.Error()), " ")
}

// judgeOf derives the live state of one team from a snapshot.
func judgeOf(one RunningTeam, read session, now time.Time) activity.Team {
	var graph *definition.Graph
	agents := agentsOf(read, one.Name)
	if one.Built != nil {
		graph = one.Built.Team.Definition.Graph
	} else {
		// The definition is the thing that could not be read, so the graph here
		// is made of the team's live agents alone: the row and the right-hand
		// column then name the members that are really there, by the short name
		// hergy gave them, rather than showing nothing at all (2.2).
		graph = nil
	}
	return activity.Of(activity.Input{
		Team:   one.Name,
		Graph:  graph,
		Agents: agents,
		Seen:   read.Seen,
		Now:    now,
	})
}

// agentsOf is one team's live agents as the judgement reads them: the name herdr
// knows each by, the node it stands for when the graph names it, its state and
// its state_change_seq.
func agentsOf(read session, name domainname.Seen) []activity.Agent {
	live := teamAgents(read.Tabs, read.Agents, name)
	out := make([]activity.Agent, 0, len(live))
	for _, agent := range live {
		out = append(out, activity.Agent{
			Herdr:  agent.Name,
			Node:   domainname.ShortIn(agent.Name, name),
			Status: agent.Status,
			Seq:    agent.Seq,
		})
	}
	return out
}

// teamAgents is one team's live agents: the ones herdr names after the team and
// stands in the team's own tab, and the ones in the tabs hergy made for the
// workers the team hired (labelled "worker:<team>/<name>", spec-activity 1). The
// tab is what says whose agent it is: a team named "core" beside a team named
// "core-hotfix" would otherwise take the other's members for its own.
func teamAgents(tabs []team.Tab, agents []protocol.Agent, name domainname.Seen) []protocol.Agent {
	wanted := map[string]bool{}
	for _, agent := range team.MemberAgents(tabs, name) {
		wanted[agent.String()] = true
	}
	workerTeam := name
	for _, tab := range tabs {
		if !domainname.IsWorkerTab(tab.Label, workerTeam) {
			continue
		}
		for _, agent := range tab.Agents {
			wanted[agent.String()] = true
		}
	}
	out := make([]protocol.Agent, 0, len(wanted))
	for _, agent := range agents {
		if agent.Name.String() != "" && wanted[agent.Name.String()] {
			out = append(out, agent)
		}
	}
	return out
}
