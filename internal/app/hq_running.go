package app

import (
	"github.com/aq2bq/hergy/internal/ops"
	"path/filepath"
	"time"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/activity"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/i18n"

	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// This file is the running teams and the workspaces as the headquarters reads
// them (d-73fc). Both stand on the activity reading: the teams, their states and
// their waiting members are judged once, by internal/activity, so the activity
// screen and the headquarters cannot disagree about a team (lead's addition 1).

// Running is GET /api/hq/running.
func (h *hqRead) Running() web.HQRunning {
	now := h.act.clock()
	read := h.act.readHerdr(now)
	out := web.HQRunning{Herdr: web.ActivityNote{OK: true}, Teams: []web.HQRunningTeam{}}
	if read.err != nil {
		out.Herdr = web.ActivityNote{OK: false, Text: i18n.T("web.activity.herdr_unavailable")}
		return out
	}
	for _, one := range h.act.teamsOf(read) {
		out.Teams = append(out.Teams, h.runningOf(one, read, now))
	}
	return out
}

// runningOf is one running team as the headquarters names it.
func (h *hqRead) runningOf(one runningTeam, read herdrNow, now time.Time) web.HQRunningTeam {
	judged := judgeOf(one, read, now)
	teamName := one.Name
	out := web.HQRunningTeam{
		Team:        domainname.TeamFromObservation(teamName),
		Definition:  one.Definition,
		Place:       one.Place.API(),
		Project:     one.Project,
		ProjectName: one.ProjectName,
		Workspace:   web.HQWorkspace{ID: one.WorkspaceID, Label: one.WorkspaceLabel},
		Tab:         web.HQTab{ID: one.TabID, Label: teamName.String()},
		Error:       one.Broken,
		Members:     []web.HQMember{},
		Workers:     []web.HQWorker{},
		Waiting:     []string{},
		NotUp:       []string{},
	}
	if id, active := h.activeLaunch(teamName.String()); active {
		out.Starting = true
		out.Launch = id
	}
	for _, node := range judged.Nodes {
		out.Members = append(out.Members, web.HQMember{
			Name: node.Name, Herdr: node.Herdr.String(), Live: node.Herdr.String() != "",
			State: node.State.Slug(),
		})
		// A member no agent is live for is one hergy has not started, or one whose
		// agent ended: a screen offers to start it (r-5d1a).
		if node.Herdr.String() == "" {
			out.NotUp = append(out.NotUp, node.Name)
		}
		if node.State == activity.Blocked || node.State == activity.HandedOver {
			out.Waiting = append(out.Waiting, node.Name)
		}
	}
	for _, hired := range judged.Hired {
		out.Workers = append(out.Workers, web.HQWorker{
			Name: domainname.ShortIn(hired.Herdr, teamName).String(),
			Tab:  protocolWorkerTab(teamName, domainname.ShortIn(hired.Herdr, teamName)),
			Live: hired.Herdr.String() != "",
		})
	}
	return out
}

// protocolWorkerTab is the label hergy gives one worker's tab.
func protocolWorkerTab(teamName, worker domainname.Seen) string {
	return domainname.RunningNames(domainname.TeamFromObservation(teamName)).WorkerTab(worker.String())
}

// Workspaces is GET /api/hq/workspaces: herdr's workspaces, the team in each, and
// the team that last ran there.
func (h *hqRead) Workspaces() web.HQWorkspaces {
	now := h.act.clock()
	read := h.act.readHerdr(now)
	out := web.HQWorkspaces{Workspaces: []web.HQWorkspace{}}
	if read.err != nil {
		return out
	}
	teams := map[string]runningTeam{}
	for _, one := range h.act.teamsOf(read) {
		teams[one.WorkspaceID] = one
	}
	last := h.lastTeamsByWorkspace()
	for _, space := range read.spaces {
		out.Workspaces = append(out.Workspaces, h.workspaceOf(space, teams[space.ID], last[space.Label]))
	}
	return out
}

// workspaceOf is one workspace with the team in it and the team that last ran
// there.
func (h *hqRead) workspaceOf(space herdr.Workspace, running runningTeam, last string) web.HQWorkspace {
	one := web.HQWorkspace{ID: space.ID, Label: space.Label, Focused: space.Focused, LastTeam: last}
	if running.Name.String() != "" {
		one.Team = running.Name.String()
		one.TeamProject = running.Project
	}
	return one
}

// lastTeamsByWorkspace is the newest team each workspace label appears with in
// the launch record.
func (h *hqRead) lastTeamsByWorkspace() map[string]string {
	out := map[string]string{}
	for _, record := range h.launchRecords() {
		if record.Workspace != "" && record.Team.String() != "" {
			out[record.Workspace] = record.Team.String()
		}
	}
	return out
}

// lastWorkspaceOf is the workspace the definition last ran in, when it is still
// free: the start sheet offers it first (spec 5 章). A definition is told by the
// team.yaml its launch read, so two projects' same-named definitions do not take
// each other's launch (M1).
func (h *hqRead) lastWorkspaceOf(read herdrNow, teamYAML domainplace.Project) *web.HQWorkspace {
	record, ok, err := ops.LatestLaunch(teamYAML)
	if err != nil || !ok || record.WorkspaceID == "" {
		return nil
	}
	squatter := ""
	for _, tab := range read.tabs {
		if tab.WorkspaceID == record.WorkspaceID && team.HoldsMember(tab, tab.Label) {
			squatter = tab.Label.String()
			break
		}
	}
	if squatter != "" {
		return nil
	}
	return &web.HQWorkspace{ID: record.WorkspaceID, Label: record.Workspace}
}

// projectNameIn is one project's name among the projects this serve shows.
func (h *hqRead) projectNameIn(path domainplace.Project) string {
	for _, view := range nameProjects(h.act.projectViews()) {
		if view.Path.Compare(path) == domainplace.Same {
			return view.Name
		}
	}
	return filepath.Base(path.String())
}
