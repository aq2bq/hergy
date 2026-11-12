package app

import (
	"io"
	"net/http"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// This file stops a team from the screens: the same result as hergy down, in the
// project the team belongs to, leaving the workspace open (d-73fc). It closes what
// hergy made even when no agent is running there (d-da92).

// Stop is POST /api/hq/stop.
func (h *hqRead) Stop(teamName string) (web.HQStopped, error) {
	read := h.act.readHerdr(h.act.clock())
	if read.err != nil {
		return web.HQStopped{}, hqRefuse(http.StatusServiceUnavailable, read.err.Error())
	}
	one, found := h.findRunning(read, teamName)
	if !found {
		return web.HQStopped{}, hqRefuse(http.StatusNotFound, i18n.Tf("error.team_not_running", teamName))
	}
	env, err := h.act.envAt(one.Project)
	if err != nil {
		return web.HQStopped{}, hqError(err)
	}
	// The panes' working directories are read before they close, and the launch
	// record says when the team was started, so the worktrees that came while it
	// ran can be listed afterwards (r-b65e, d-970c).
	dirs := teamPaneDirs(read.tabs, one.Name)
	owner := teamLeftovers(one.Project, one.Name.String())
	closed, err := h.closeTeam(env, one, read)
	if err != nil {
		return web.HQStopped{}, hqRefuse(http.StatusInternalServerError, err.Error())
	}
	h.finishLaunchFor(teamName)
	return web.HQStopped{
		Team: teamName, Definition: one.Definition, Project: one.Project,
		Workspace: web.HQWorkspace{ID: one.WorkspaceID, Label: one.WorkspaceLabel},
		Closed:    closed, StoppedAt: clockText(time.Now()),
		Leftovers: hqLeftoversOf(team.GitLeftovers(env.ProjectRoot, owner.Since, dirs)),
	}, nil
}

// findRunning is one running team the headquarters shows, by name.
func (h *hqRead) findRunning(read herdrNow, name string) (runningTeam, bool) {
	for _, one := range h.act.teamsOf(read) {
		if one.Name.String() == name {
			return one, true
		}
	}
	return runningTeam{}, false
}

// stopTarget is what stopping one team takes: the panes it closes, and the one
// way to close them. Stop and the confirmation both take it from here (ac-ddb6).
type stopTarget struct {
	members []string
	workers []string
	close   func(selfPane domainname.PaneID) error
}

// stopTargets is that one answer: a definition that reads closes every member's
// pane by the starts it was launched with; one that does not is closed by the
// names hergy gave its panes.
func (h *hqRead) stopTargets(env team.Env, one runningTeam, read herdrNow) stopTarget {
	cat := env.Catalog(source.Disk{})
	r, buildErr := buildRunningIn(cat, env.ProjectRoot, team.Running{Team: domainname.TeamFromObservation(one.Name.Observation()), Definition: one.Definition}, one.Place)
	workers := workerNamesOf(read.tabs, one.Name)
	if buildErr != nil {
		l := newLauncherWithPort(&resolved{team: team.Team{Name: domainname.TeamFromObservation(one.Name)}}, 30000, io.Discard, h.act.herdrPort()).WithWaitClock(h.stopNow, h.stopPause)
		agents := team.MemberAgents(read.tabs, one.Name)
		return stopTarget{members: seenStrings(agents), workers: seenStrings(workers), close: func(selfPane domainname.PaneID) error {
			_, err := l.DownByName(agents, selfPane, false)
			return err
		}}
	}
	l := newLauncherWithPort(r, 30000, io.Discard, h.act.herdrPort()).WithWaitClock(h.stopNow, h.stopPause)
	members := make([]string, 0, len(r.starts))
	for _, start := range r.starts {
		members = append(members, start.Short.String())
	}
	return stopTarget{members: members, workers: seenStrings(workers), close: func(selfPane domainname.PaneID) error {
		_, err := l.Down(r.starts, selfPane, false)
		return err
	}}
}

// closeTeam puts one team away through the one answer the confirmation reads.
func (h *hqRead) closeTeam(env team.Env, one runningTeam, read herdrNow) (web.HQClosed, error) {
	selfPane := currentPane()
	target := h.stopTargets(env, one, read)
	if err := target.close(selfPane); err != nil {
		return web.HQClosed{}, err
	}
	return web.HQClosed{Members: len(target.members), Workers: len(target.workers)}, nil
}

// finishLaunchFor marks this serve's start of a team as given up: the panes it
// made are being closed, so its progress must not keep saying it is starting.
func (h *hqRead) finishLaunchFor(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range h.order {
		op := h.ops[id]
		op.mu.Lock()
		if op.launch.Team == name && (op.launch.Phase == "starting" || op.launch.Phase == "awaiting") {
			if op.cancel != nil {
				op.cancel()
			}
			op.launch.Phase = "cancelled"
			op.launch.Waiting = nil
			op.finished = time.Now()
		}
		op.mu.Unlock()
	}
}

// hqLeftoversOf is what a stopped team left in git, as the stop sheet reads it.
func hqLeftoversOf(left team.Leftovers) web.HQLeftovers {
	out := web.HQLeftovers{Worktrees: []web.HQWorktree{}, Stashes: left.Stashes}
	for _, one := range left.Worktrees {
		out.Worktrees = append(out.Worktrees, web.HQWorktree{
			Path: one.Path, Branch: one.Branch, Dirty: one.Dirty, Ahead: one.Ahead, Base: one.Base,
			PaneCwd: one.PaneCwd,
		})
	}
	return out
}
