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
	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/source"
)

// RunningTeam is one running team with its team definition built: what hergy
// status lists and what the activity screen shows. Broken says why a definition
// that cannot be built could not be, in the words hergy status writes; the team
// is still listed then, so one such team does not hide the rest.
type RunningTeam struct {
	// Name is the team's name: the label on its tab in herdr.
	Name domainname.Seen
	// Definition is the team definition the team was booted from, as the launch
	// record names it. It differs from Name for a team booted with --as.
	Definition string
	// Built is the built definition: its graph, its members and its workers.
	// nil when the definition cannot be built.
	Built *Resolved
	// Broken is why the definition cannot be built, as hergy status's 詳細 line
	// reads. Empty when it builds.
	Broken string
	// Elsewhere says the definition is not in its project's catalog at all.
	Elsewhere bool
	// Project is the listed project the team belongs to (d-73fc): the launch
	// record's project, or the project whose layer holds the definition. Name is
	// what a person reads, and Catalog is that project's reading, which the
	// delivery check and the opened part use.
	Project domainplace.Project
	// Place is the definition's layer: "local" for a project's definition,
	// "global" for one started in a project from the global layer.
	Place       domainplace.Place
	ProjectName string
	Catalog     *catalog.Catalog
	// Workspace is the herdr workspace the team's tab stands in.
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
	return readRunningTeamsIn([]ProjectView{view}, read, true)
}

// ProjectView is one project the activity reads: its root, the name a person
// reads, the environment its layer lives in, and the catalog over that layer.
type ProjectView struct {
	Path domainplace.Project
	Name string
	Env  Environment
	Cat  *catalog.Catalog
}

// readRunningTeamsIn reads the running teams that belong to the given projects:
// a team belongs to the project its launch record names, and a team with no
// record belongs to the project whose layer holds its definition. Teams outside
// these projects are left out (d-73fc: a serve shows the projects it lists).
//
// The Herdr read is handed in rather than done here, so one snapshot can be used
// for all the team lookups in this answer.
func readRunningTeamsIn(views []ProjectView, read session, lenient bool) []RunningTeam {
	running := team.RunningTeams(read.Tabs, launchTeams(), read.AgentsRead)
	records := latestLaunches()
	out := make([]RunningTeam, 0, len(running))
	for _, entry := range running {
		view, recorded, ok := projectOf(entry, records, views, read.Tabs, lenient)
		if !ok {
			continue
		}
		one := RunningTeam{
			Name: entry.Team.Observation(), Definition: entry.Definition,
			Project: view.Path, ProjectName: view.Name, Catalog: view.Cat,
			Place: recorded.Layer.At(view.Path),
		}
		if tab, found := teamTab(read.Tabs, entry.Team.Observation()); found {
			one.TabID = tab.ID
			one.WorkspaceID = tab.WorkspaceID
			one.WorkspaceLabel = workspaceLabel(read.Workspaces, tab.WorkspaceID)
		}
		r, err := buildRunningIn(view.Cat, view.Env.ProjectRoot, entry, recorded.Layer.String())
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

// projectOf is the project one running team belongs to, and its newest launch
// record. The record the reading chose says the project when it names one: it is
// the record of the project the team's panes stand in, so another project's team
// of the same name is not taken for this one's (r-87c8). A team whose record
// names a project outside the views is left out: it is another project's, and a
// serve shows only the projects it lists. A team with no record belongs to the one
// view whose layer holds its definition, or, when the record says nothing, whose
// panes its own stand among (d-73fc).
func projectOf(entry team.Running, records launchRecords, views []ProjectView, tabs []team.Tab, lenient bool) (ProjectView, launchlog.Record, bool) {
	if !entry.Project.IsZero() {
		for _, view := range views {
			if entry.Project.Compare(view.Path) == domainplace.Same {
				record, _ := records.At(view.Path, entry.Team.String())
				return view, record, true
			}
		}
		if lenient && len(views) == 1 {
			record, _ := records.At(views[0].Path, entry.Team.String())
			return views[0], record, true
		}
		return ProjectView{}, launchlog.Record{}, false
	}
	// A team with no record belongs to the project its named panes stand in: a
	// boot that failed once the tab was made leaves the panes in the project the
	// work runs in, and another project's team is not taken for this one's
	// (d-73fc).
	for _, view := range views {
		if panesIn(view.Path, tabs, entry.Team.Observation()) {
			record, _ := records.At(view.Path, entry.Team.String())
			return view, record, true
		}
	}
	for _, view := range views {
		if view.Cat.HoldsDefinition(entry.Definition, source.LayerLocal) {
			return view, launchlog.Record{}, true
		}
	}
	// The CLI reads one project and keeps every team it cannot place, saying why
	// its definition does not read; the screens show only teams they can place.
	if lenient && len(views) == 1 {
		return views[0], launchlog.Record{}, true
	}
	return ProjectView{}, launchlog.Record{}, false
}

// panesIn reports whether one team's named panes stand in a project. The panes
// are paired with their tab's PaneDirs by position.
func panesIn(project domainplace.Project, tabs []team.Tab, name domainname.Seen) bool {
	for _, tab := range tabs {
		if !protocol.NamesTeam(tab.Label.String(), name.String()) {
			continue
		}
		for i, pane := range tab.Panes {
			if !domainname.BelongsTo(pane, name) || i >= len(tab.PaneDirs) {
				continue
			}
			dir := tab.PaneDirs[i].Project()
			if project.Contains(dir) {
				return true
			}
		}
	}
	return false
}

// launchRecords is this machine's launch record as reading a running team needs
// it: the newest record of each project and team name, and of each team name
// alone, for a team whose tab does not say where it stands (r-87c8).
type launchKey struct {
	project domainplace.Project
	team    string
}

// launchRecords indexes the latest launch of each project and team.
type launchRecords struct {
	byPlace map[launchKey]launchlog.Record
	byTeam  map[string]launchlog.Record
}

// At is the newest record of one team in one project.
func (r launchRecords) At(project domainplace.Project, name string) (launchlog.Record, bool) {
	record, ok := r.byPlace[launchKey{project, name}]
	return record, ok
}

// latestLaunches is the newest launch record of each project and team name, and
// of each team name alone.
func latestLaunches() launchRecords {
	log, err := launchlog.Open()
	if err != nil {
		return launchRecords{}
	}
	records, err := log.Read()
	if err != nil {
		return launchRecords{}
	}
	out := launchRecords{
		byPlace: make(map[launchKey]launchlog.Record, len(records)),
		byTeam:  make(map[string]launchlog.Record, len(records)),
	}
	for _, record := range records {
		if record.Team.String() == "" || record.IsHire() {
			continue
		}
		out.byTeam[record.Team.String()] = record
		if !record.Project.Project().IsZero() {
			out.byPlace[launchKey{record.Project.Project(), record.Team.String()}] = record
		}
	}
	return out
}

// teamTab is the tab labelled for one team, with the workspace it stands in.
func teamTab(tabs []team.Tab, name domainname.Seen) (team.Tab, bool) {
	for _, tab := range tabs {
		if protocol.NamesTeam(tab.Label.String(), name.String()) && team.HoldsMember(tab, name) {
			return tab, true
		}
	}
	return team.Tab{}, false
}

// workspaceLabel is one workspace's label, or the id when herdr did not list it.
func workspaceLabel(spaces []herdr.Workspace, id string) string {
	for _, space := range spaces {
		if space.ID == id {
			return space.Label
		}
	}
	return id
}

// session is one read of herdr as the activity needs it: the tabs, each with the
// live agents in it, the live agents themselves, whether the agent list could be
// read at all, when each agent's state was first seen, and, when herdr could not
// be reached at all, why not (2.7).

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
