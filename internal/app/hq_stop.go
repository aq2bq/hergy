package app

import (
	"io"
	"net/http"
	"os"
	"time"

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
	read := h.act.readHerdrFresh(h.act.clock())
	if read.err != nil {
		return web.HQStopped{}, hqRefuse(http.StatusServiceUnavailable, read.err.Error())
	}
	one, found := h.findRunning(read, teamName)
	if !found {
		return web.HQStopped{}, hqRefuse(http.StatusNotFound, i18n.Tf("error.team_not_running", teamName))
	}
	env, err := h.act.env.AtProject(one.Project)
	if err != nil {
		return web.HQStopped{}, hqError(err)
	}
	closed, err := h.closeTeam(env, one, read)
	if err != nil {
		return web.HQStopped{}, hqRefuse(http.StatusInternalServerError, err.Error())
	}
	h.finishLaunchFor(teamName)
	h.act.forgetHerdr()
	return web.HQStopped{
		Team: teamName, Definition: one.Definition, Project: one.Project,
		Workspace: web.HQWorkspace{ID: one.WorkspaceID, Label: one.WorkspaceLabel},
		Closed:    closed, StoppedAt: clockText(time.Now()),
	}, nil
}

// findRunning is one running team the headquarters shows, by name.
func (h *hqRead) findRunning(read herdrNow, name string) (runningTeam, bool) {
	for _, one := range h.act.teamsOf(read) {
		if one.Name == name {
			return one, true
		}
	}
	return runningTeam{}, false
}

// closeTeam puts one team away. A definition that reads closes every member's
// pane, including the ones whose agent ended; one that does not is closed by the
// names hergy gave its panes, so a broken definition never keeps a team up.
func (h *hqRead) closeTeam(env team.Env, one runningTeam, read herdrNow) (web.HQClosed, error) {
	cat := env.Catalog(source.Disk{})
	r, buildErr := buildRunningIn(cat, env.ProjectRoot, team.Running{Team: one.Name, Definition: one.Definition}, one.Place)
	selfPane := os.Getenv("HERDR_PANE_ID")
	counts := countsOf(one, read)
	if buildErr != nil {
		l := newLauncher(&resolved{team: team.Team{Name: one.Name}}, 30000, io.Discard)
		agents := team.MemberAgents(read.tabs, one.Name)
		if _, err := l.DownByName(agents, selfPane, false); err != nil {
			return web.HQClosed{}, err
		}
		return counts, nil
	}
	l := newLauncher(r, 30000, io.Discard)
	if _, err := l.Down(r.starts, selfPane, false); err != nil {
		return web.HQClosed{}, err
	}
	return counts, nil
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

// countsOf is how many panes a stop is about to close: the definition's members,
// and the worker tabs hergy made for the team.
func countsOf(one runningTeam, read herdrNow) web.HQClosed {
	members := 0
	if one.Built != nil {
		members = len(one.Built.starts)
	}
	workers := 0
	for _, tab := range read.tabs {
		if _, ok := workerOf(tab.Label, one.Name); ok {
			workers++
		}
	}
	return web.HQClosed{Members: members, Workers: workers}
}

// workerOf is the worker name a tab label carries ("worker:<team>/<worker>").
func workerOf(label, teamName string) (string, bool) {
	prefix := "worker:" + teamName + "/"
	if len(label) <= len(prefix) || label[:len(prefix)] != prefix {
		return "", false
	}
	return label[len(prefix):], true
}
