package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// This file starts a member of a running team from the screens: the same result
// as hergy refresh <member>, in the project the team belongs to (r-5d1a). The
// member written in the definition that no agent of the team is live for is
// given a pane in the team's tab and started there, the way hergy up starts one;
// the members already running keep their conversation. It goes on after the
// answer, and its progress and its startup question are read the way a start's
// are (GET /api/hq/launches/{id}).

// RefreshMembers is POST /api/hq/refresh: hergy refresh, from the screens.
func (h *hqRead) RefreshMembers(in web.HQRefreshIn) (web.HQLaunch, error) {
	name := strings.TrimSpace(in.Team)
	now := h.act.clock()
	read := h.act.readHerdrFresh(now)
	if read.err != nil {
		return web.HQLaunch{}, hqRefuse(http.StatusServiceUnavailable, read.err.Error())
	}
	one, found := h.findRunning(read, name)
	if !found {
		return web.HQLaunch{}, hqRefuse(http.StatusNotFound, i18n.Tf("error.team_not_running", name))
	}
	if one.Built == nil {
		return web.HQLaunch{}, hqRefuse(http.StatusUnprocessableEntity, i18n.Tf("web.hq.raise_broken", name))
	}
	// One operation at a time on a team: a start already out is making
	// panes in the same tab, and a second one would make them over it.
	if _, busy := h.activeLaunch(name); busy {
		return web.HQLaunch{}, hqRefuse(http.StatusConflict, i18n.Tf("web.hq.raise_busy", name))
	}
	env, err := h.act.env.AtProject(one.Project)
	if err != nil {
		return web.HQLaunch{}, hqError(err)
	}
	// The definition is read as it is now: the member was written into it after
	// the team was booted, and this start is what hands it the current rules.
	r, err := buildRunningIn(env.Catalog(source.Disk{}), env.ProjectRoot,
		team.Running{Team: one.Name, Definition: one.Definition}, one.Place)
	if err != nil {
		return web.HQLaunch{}, hqRefuse(http.StatusUnprocessableEntity, i18n.Tf("web.hq.raise_broken", name))
	}
	selected, err := h.startTargets(one, read, r, in.Members, now)
	if err != nil {
		return web.HQLaunch{}, err
	}
	tab := web.HQTab{ID: one.TabID, Label: name}
	op := h.newOp(web.HQLaunch{
		Action: web.ActionRefresh, Team: name, Definition: one.Definition, Place: apiPlaceOf(one.Place),
		Project: one.Project, ProjectName: one.ProjectName,
		Workspace: web.HQWorkspace{ID: one.WorkspaceID, Label: one.WorkspaceLabel},
		Tab:       tab, Members: shortsOf(selected), Phase: "starting", Steps: []web.HQLaunchStep{},
	})
	ctx, cancel := context.WithCancel(h.context())
	op.mu.Lock()
	op.cancel = cancel
	op.mu.Unlock()
	// A read held before this start would hide the pane it makes, and the member
	// it starts: the next read sees herdr as it is now.
	h.act.forgetHerdr()
	go h.runRefresh(ctx, op, env, r, selected)
	return op.snapshot(), nil
}

// startTargets is the members a start starts: the ones the screen named, each of
// them a member the definition has and no agent of the team is live for. A member
// that is already running is refused, because starting it again would end the
// conversation it holds and nobody asked for that.
func (h *hqRead) startTargets(one runningTeam, read herdrNow, r *resolved, names []string, now time.Time) ([]definition.Start, error) {
	if len(names) == 0 {
		return nil, hqRefuse(http.StatusUnprocessableEntity, i18n.T("web.hq.raise_none"))
	}
	notUp := map[string]bool{}
	for _, node := range judgeOf(one, read, now).Nodes {
		if node.Herdr == "" {
			notUp[node.Name] = true
		}
	}
	selected := make([]definition.Start, 0, len(names))
	for _, name := range names {
		member, err := r.member(strings.TrimSpace(name))
		if err != nil {
			return nil, hqRefuse(http.StatusUnprocessableEntity, err.Error())
		}
		if !notUp[member.Short] {
			return nil, hqRefuse(http.StatusConflict, i18n.Tf("web.hq.raise_running", member.Short))
		}
		selected = append(selected, *member)
	}
	return selected, nil
}

// runRefresh starts the selected members in the background and keeps the
// progress. It is hergy refresh <member>, the same launcher call the command line
// makes, so a member with no pane is given one in the team's tab and a member
// whose agent ended is started again in the pane hergy named after it.
func (h *hqRead) runRefresh(ctx context.Context, op *launchOp, env team.Env, r *resolved, selected []definition.Start) {
	l := newLauncher(r, 30000, io.Discard).WithContext(ctx)
	l.Observe(func(event launcher.Event) { h.observe(op, event) })
	err := l.Refresh(selected, os.Getenv("HERDR_PANE_ID"))
	if errors.Is(err, launcher.ErrCancelled) || ctx.Err() != nil {
		h.finish(op, "cancelled", "")
		return
	}
	if err != nil {
		h.finish(op, "failed", err.Error())
		return
	}
	recordLaunch(io.Discard, env, r, launchlog.CommandRefresh, selected)
	// The member started is on its current prompt now, and the others' may have
	// moved with the definition: the band's count is read again (r-a8fa).
	h.act.InvalidateDelivery()
	h.finish(op, "done", "")
}

// shortsOf is the members as a person reads them in the progress answer: the
// short names team.yaml writes.
func shortsOf(members []definition.Start) []string {
	out := make([]string, 0, len(members))
	for i := range members {
		out = append(out, members[i].Short)
	}
	return out
}
