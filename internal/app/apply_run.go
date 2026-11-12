package app

import (
	"context"
	"errors"

	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// This file applies the drift between the saved definitions and the running
// teams (d-36ef): the screens' POST /api/apply. The operation is the one sheet a
// person works with; what hergy does for each member follows from the drift the
// reading found (web.ApplyMember.How). The work goes on after the answer, so the
// sheet reads its progress and any startup question the way it reads a start's.

// ApplyMembers is POST /api/apply: apply the drift to the named members, by
// herdr name. Nothing is applied when one named member has nothing to apply or
// cannot be applied now: the sheet sends what it read, and a refusal names the
// members to look at.
func (h *hqRead) ApplyMembers(names []string) (web.HQLaunch, error) {
	ctx, cancel := context.WithCancel(h.context())
	type answer struct {
		launch web.HQLaunch
		err    error
	}
	ready := make(chan answer, 1)
	go func() {
		defer cancel()
		var op *launchOp
		input := h.applyInput(names, func(event ops.Event) {
			if op != nil {
				h.observeApply(op, event)
			}
		})
		input.Accepted = func(selection ops.ApplySelection) {
			op = h.newOp(applyLaunch(selection.Selected))
			op.mu.Lock()
			op.cancel = cancel
			op.teams = selection.Teams()
			op.mu.Unlock()
			ready <- answer{launch: op.snapshot()}
		}
		_, err := ops.Apply(ctx, input)
		if op == nil {
			ready <- answer{err: hqOperationError(err)}
			return
		}
		h.finishOperation(ctx, op, err)
	}()
	result := <-ready
	return result.launch, result.err
}

func (h *hqRead) applyInput(names []string, observe func(ops.Event)) ops.ApplyInput {
	return ops.ApplyInput{Now: h.act.clock, Env: h.act.env, Views: h.act.projectViews(), Read: func() ops.Session { return sessionOf(h.act.readHerdr(h.act.clock())) },
		Selection: ops.ApplySelectionInput{Names: names, Policy: ops.RequireWholeSelection}, Timeout: 30000, Port: h.act.herdrPort(), Observe: observe, Locks: h.locks}
}
func (h *hqRead) observeApply(op *launchOp, event ops.Event) {
	if event.Progress != nil {
		h.observe(op, *event.Progress)
	}
	if event.Applied != nil {
		h.stepApplied(op, event.Applied.Member)
	}
}
func applySelection(plan deliveryPlan, names []string) ([]plannedMember, []string) {
	selection, err := ops.SelectApply(plan, ops.ApplySelectionInput{Names: names, Policy: ops.RequireWholeSelection})
	if err != nil {
		return nil, []string{err.Error()}
	}
	return selection.Selected, selection.Refusals
}

// applyLaunch is the answer's starting state: the teams and members it applies,
// and nothing else. A request of one team names it; one that spans teams leaves
// the team and its project empty, because naming one would be a lie about the
// other.
func applyLaunch(selected []plannedMember) web.HQLaunch {
	launch := web.HQLaunch{
		Action: web.ActionApply, Phase: "starting", Steps: []web.HQLaunchStep{},
		Members: shortsOfStarts(selected),
	}
	first := selected[0].Team
	same := true
	for _, member := range selected {
		if member.Team.Name != first.Name {
			same = false
			break
		}
	}
	if same {
		launch.Team = first.Name.String()
		launch.Definition = first.Definition
		launch.Project = first.Project
		launch.ProjectName = first.ProjectName
		launch.Place = first.Place.API()
		launch.Workspace = web.HQWorkspace{ID: first.WorkspaceID, Label: first.WorkspaceLabel}
		launch.Tab = web.HQTab{ID: first.TabID, Label: first.Name.String()}
	}
	return launch
}

// shortsOfStarts is the selected members as a person reads them.
func shortsOfStarts(members []plannedMember) []string {
	out := make([]string, 0, len(members))
	for _, member := range members {
		out = append(out, member.Start.Short.String())
	}
	return out
}

// runApply applies one group of members after another. A team is built from its
// own project, and its members are started, restarted or handed the prompt again
// with the launcher the command line uses; the record keeps the plain prompt, so
// the next comparison does not see the preamble as a change.
func (h *hqRead) runApply(ctx context.Context, op *launchOp, selected []plannedMember) {
	var names []string
	for _, member := range selected {
		names = append(names, member.Start.Name.String())
	}
	input := h.applyInput(names, func(event ops.Event) { h.observeApply(op, event) })
	input.Accepted = func(selection ops.ApplySelection) { op.mu.Lock(); op.teams = selection.Teams(); op.mu.Unlock() }
	_, err := ops.Apply(ctx, input)
	h.finishOperation(ctx, op, err)
}

// stepApplied is one member's line of the apply's progress.
func (h *hqRead) stepApplied(op *launchOp, member plannedMember) {
	key := "apply.progress_resend"
	switch member.How {
	case ops.ApplyResend, ops.ApplyCannot:
	case ops.ApplyStart:
		key = "apply.progress_start"
	case ops.ApplyRestart:
		key = "apply.progress_restart"
	}
	op.mu.Lock()
	op.stepLocked("member", i18n.Tf(key, member.Start.Short.String()))
	op.mu.Unlock()
}

// ApplyMembers is POST /api/apply on the activity reading: the headquarters
// keeps the progress, as it does for a start.
func (a *activityRead) ApplyMembers(names []string) (web.HQLaunch, error) {
	if a.hq == nil {
		return web.HQLaunch{}, errors.New(i18n.T("web.error.no_hq"))
	}
	return a.hq.ApplyMembers(names)
}
