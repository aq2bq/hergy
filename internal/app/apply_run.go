package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// This file applies the drift between the saved definitions and the running
// teams (d-36ef): the screens' POST /api/apply. The operation is the one sheet a
// person works with; what hergy does for each member follows from the drift the
// reading found (web.ApplyMember.how). The work goes on after the answer, so the
// sheet reads its progress and any startup question the way it reads a start's.

// ApplyMembers is POST /api/apply: apply the drift to the named members, by
// herdr name. Nothing is applied when one named member has nothing to apply or
// cannot be applied now: the sheet sends what it read, and a refusal names the
// members to look at.
func (h *hqRead) ApplyMembers(names []string) (web.HQLaunch, error) {
	if len(names) == 0 {
		return web.HQLaunch{}, hqRefuse(http.StatusUnprocessableEntity, i18n.T("apply.none_selected"))
	}
	now := h.act.clock()
	read := h.act.readHerdrFresh(now)
	if read.err != nil {
		return web.HQLaunch{}, hqRefuse(http.StatusServiceUnavailable, read.err.Error())
	}
	plan := h.act.plan(now)
	if !plan.apply.Herdr.OK {
		return web.HQLaunch{}, hqRefuse(http.StatusServiceUnavailable, plan.apply.Herdr.Text)
	}
	selected, refusals := applySelection(plan, names)
	if len(refusals) > 0 {
		return web.HQLaunch{}, hqRefuse(http.StatusUnprocessableEntity, strings.Join(refusals, "\n"))
	}
	for _, group := range groupByTeam(selected) {
		if _, busy := h.activeLaunch(group[0].team.Name); busy {
			return web.HQLaunch{}, hqRefuse(http.StatusConflict, i18n.Tf("apply.refused_busy", group[0].team.Name))
		}
	}
	op := h.newOp(applyLaunch(selected))
	ctx, cancel := context.WithCancel(h.context())
	op.mu.Lock()
	op.cancel = cancel
	op.mu.Unlock()
	// A read held before this apply would hide a start it makes: the next read
	// sees herdr as it is now.
	h.act.forgetHerdr()
	go h.runApply(ctx, op, selected)
	return op.snapshot(), nil
}

// applySelection is the members the request names, or one sentence for each name
// that cannot be applied.
func applySelection(plan deliveryPlan, names []string) ([]plannedMember, []string) {
	selected := make([]plannedMember, 0, len(names))
	var refusals []string
	for _, name := range names {
		member, ok := plan.members[name]
		switch {
		case !ok:
			refusals = append(refusals, i18n.Tf("apply.refused_unknown", name))
		case !member.changed:
			refusals = append(refusals, i18n.Tf("apply.no_change", name))
		case !member.canApply():
			refusals = append(refusals, member.whyNot)
		default:
			selected = append(selected, member)
		}
	}
	return selected, refusals
}

// groupByTeam is the selected members by running team, in the order the request
// named them, so one team's members are applied together.
func groupByTeam(members []plannedMember) [][]plannedMember {
	var out [][]plannedMember
	at := map[string]int{}
	for _, member := range members {
		index, ok := at[member.team.Name]
		if !ok {
			at[member.team.Name] = len(out)
			out = append(out, []plannedMember{member})
			continue
		}
		out[index] = append(out[index], member)
	}
	return out
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
	first := selected[0].team
	same := true
	for _, member := range selected {
		if member.team.Name != first.Name {
			same = false
			break
		}
	}
	if same {
		launch.Team = first.Name
		launch.Definition = first.Definition
		launch.Project = first.Project
		launch.ProjectName = first.ProjectName
		launch.Place = apiPlaceOf(first.Place)
		launch.Workspace = web.HQWorkspace{ID: first.WorkspaceID, Label: first.WorkspaceLabel}
		launch.Tab = web.HQTab{ID: first.TabID, Label: first.Name}
	}
	return launch
}

// shortsOfStarts is the selected members as a person reads them.
func shortsOfStarts(members []plannedMember) []string {
	out := make([]string, 0, len(members))
	for _, member := range members {
		out = append(out, member.start.Short)
	}
	return out
}

// runApply applies one group of members after another. A team is built from its
// own project, and its members are started, restarted or handed the prompt again
// with the launcher the command line uses; the record keeps the plain prompt, so
// the next comparison does not see the preamble as a change.
func (h *hqRead) runApply(ctx context.Context, op *launchOp, selected []plannedMember) {
	for _, group := range groupByTeam(selected) {
		env, err := h.act.envAt(group[0].team.Project)
		if err != nil {
			h.finish(op, "failed", err.Error())
			return
		}
		cat := env.Catalog(source.Disk{})
		r, err := buildRunningIn(cat, env.ProjectRoot,
			team.Running{Team: group[0].team.Name, Definition: group[0].team.Definition}, group[0].team.Place)
		if err != nil {
			h.finish(op, "failed", err.Error())
			return
		}
		if err := h.applyGroup(ctx, op, env, cat, r, group); err != nil {
			if errors.Is(err, launcher.ErrCancelled) || ctx.Err() != nil {
				h.finish(op, "cancelled", "")
				return
			}
			h.finish(op, "failed", err.Error())
			return
		}
	}
	// The members are on their current prompts now, so the band is read again.
	h.act.InvalidateDelivery()
	h.finish(op, "done", "")
}

// applyGroup applies one team's members, and records what they were handed.
func (h *hqRead) applyGroup(ctx context.Context, op *launchOp, env team.Env, cat *catalog.Catalog, r *resolved, group []plannedMember) error {
	l := newLauncher(r, 30000, io.Discard).WithContext(ctx).WithoutNextHint()
	l.Observe(func(event launcher.Event) { h.observe(op, event) })
	var starts, restarts, resends []definition.Start
	for _, member := range group {
		switch member.how {
		case web.ApplyStart:
			starts = append(starts, member.start)
		case web.ApplyRestart:
			restarts = append(restarts, member.start)
		default:
			resends = append(resends, member.start)
		}
	}
	if len(starts)+len(restarts) > 0 {
		if err := l.Refresh(append(starts, restarts...), os.Getenv("HERDR_PANE_ID")); err != nil {
			return err
		}
	}
	if len(resends) > 0 {
		if err := l.Reprompt(promptsToResend(cat, env.ProjectRoot, r, resends)); err != nil {
			return err
		}
	}
	all := make([]definition.Start, 0, len(group))
	for _, member := range group {
		all = append(all, member.start)
		h.stepApplied(op, member)
	}
	recordLaunch(io.Discard, env, r, launchlog.CommandApply, all)
	return nil
}

// stepApplied is one member's line of the apply's progress.
func (h *hqRead) stepApplied(op *launchOp, member plannedMember) {
	key := "apply.progress_resend"
	switch member.how {
	case web.ApplyStart:
		key = "apply.progress_start"
	case web.ApplyRestart:
		key = "apply.progress_restart"
	}
	op.mu.Lock()
	op.stepLocked("member", i18n.Tf(key, member.start.Short))
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
