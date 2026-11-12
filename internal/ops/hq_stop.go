package ops

import (
	"context"
	"net/http"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
)

// This file stops a team from the screens: the same result as hergy down, in the
// project the team belongs to, leaving the workspace open (d-73fc). It closes what
// hergy made even when no agent is running there (d-da92).

// Stop is POST /api/hq/stop.
func (h *hqRead) Stop(teamName string) (HQStopped, error) {
	read := h.act.readHerdr(h.act.clock())
	if read.err != nil {
		return HQStopped{}, hqRefuse(http.StatusServiceUnavailable, read.err.Error())
	}
	one, found := h.findRunning(read, teamName)
	if !found {
		return HQStopped{}, hqRefuse(http.StatusNotFound, i18n.Tf("error.team_not_running", teamName))
	}
	env, err := h.act.envAt(one.Project)
	if err != nil {
		return HQStopped{}, hqError(err)
	}
	result, err := StopTeam(context.Background(), h.stopInput(env, one, read))
	if err != nil {
		return HQStopped{}, hqRefuse(http.StatusInternalServerError, err.Error())
	}
	return HQStopped{Team: teamName, Definition: one.Definition, Project: one.Project,
		Workspace: HQWorkspace{ID: one.WorkspaceID, Label: one.WorkspaceLabel},
		Closed:    HQClosed{Members: result.Closed.Members, Workers: result.Closed.Workers},
		StoppedAt: clockText(time.Now()), Leftovers: hqLeftoversOf(result.Leftovers)}, nil
}

func (h *hqRead) stopInput(env Environment, one runningTeam, read herdrNow) StopInput {
	running := team.Running{Team: domainname.TeamFromObservation(one.Name), Definition: one.Definition, Project: one.Project}
	target := teamStopTarget(running, read.tabs)
	return StopInput{Env: env, Target: target, Layer: one.Place.Layer(), Timeout: 30000,
		SelfPane: CurrentPane(), Port: h.act.herdrPort(), Now: h.stopNow, Pause: h.stopPause,
		PendingLaunches: h.visitPendingLaunches, Locks: h.locks}
}

// findRunning is one running team the headquarters shows, by name.
func (h *hqRead) findRunning(read herdrNow, name string) (runningTeam, bool) {
	for _, one := range h.act.teamsOf(read) {
		if one.Name.String() == name {
			return one, true
		}
	}
	if one, ok := stoppedTeam(read.tabs, name); ok {
		view, record, found := projectOf(one, latestLaunches(), h.act.projectViews(), read.tabs, false)
		if found {
			tab, _ := teamTab(read.tabs, one.Team.Observation())
			return runningTeam{Name: one.Team.Observation(), Definition: one.Definition, Project: view.Path,
				Place: record.Layer.At(view.Path), WorkspaceID: tab.WorkspaceID,
				WorkspaceLabel: workspaceLabel(read.spaces, tab.WorkspaceID), TabID: tab.ID}, true
		}
	}
	return runningTeam{}, false
}

// stopTarget is presentation of the operation's plan, without its execution.
type stopTarget struct{ members, workers []string }

func (h *hqRead) stopTargets(env Environment, one runningTeam, read herdrNow) stopTarget {
	in := h.stopInput(env, one, read)
	in.PlanOnly = true
	result, _ := StopTeam(context.Background(), in)
	plan := result.Plan
	return stopTarget{members: plan.MemberLabels, workers: seenStrings(plan.Workers)}
}

// visitPendingLaunches adapts locked UI state to the operation's view. It makes
// no decision about which launch to cancel or which stop outcome permits it.
func (h *hqRead) visitPendingLaunches(visit func(PendingLaunch)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range h.order {
		op := h.ops[id]
		op.mu.Lock()
		visit(PendingLaunch{Team: op.launch.Team, Teams: op.teams, Phase: op.launch.Phase, Cancel: op.cancel,
			Finish: func(phase string, at time.Time) {
				op.launch.Phase = phase
				op.launch.Waiting = nil
				op.finished = at
			}})
		op.mu.Unlock()
	}
}

// hqLeftoversOf is what a stopped team left in git, as the stop sheet reads it.
func hqLeftoversOf(left Leftovers) HQLeftovers {
	out := HQLeftovers{Worktrees: []HQWorktree{}, Stashes: left.Stashes}
	for _, one := range left.Worktrees {
		out.Worktrees = append(out.Worktrees, HQWorktree(one))
	}
	return out
}

func seenStrings(names []domainname.Seen) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = n.String()
	}
	return out
}
