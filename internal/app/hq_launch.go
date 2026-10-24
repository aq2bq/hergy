package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// This file starts a team from the screens: the options the start sheet draws,
// the start itself in the background, its progress and its startup question, and
// giving it up (d-73fc, d-d589, spec 5 章). A start does what hergy up does —
// the same tab, members, prompts and launch record — with the workspace chosen
// rather than taken from the calling pane.

// LaunchOptions is GET /api/hq/launch/options.
func (h *hqRead) LaunchOptions(project, definitionName, place string) (web.HQLaunchOptions, error) {
	layer, err := placeLayer(place)
	if err != nil {
		return web.HQLaunchOptions{}, err
	}
	if project == "" && place == source.LayerGlobal {
		return h.globalOptions(definitionName), nil
	}
	env, err := h.projectEnv(project)
	if err != nil {
		return web.HQLaunchOptions{}, err
	}
	cat := env.Catalog(source.Disk{})
	def, err := cat.Definition(definitionName, layer)
	if err != nil {
		return web.HQLaunchOptions{}, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	read := h.act.readHerdrFresh(h.act.clock())
	out := web.HQLaunchOptions{
		Definition: definitionName, Place: place,
		Project:          &web.HQRef{Path: env.ProjectRoot, Name: h.projectNameIn(env.ProjectRoot), Display: h.act.env.DisplayPath(env.ProjectRoot)},
		Members:          memberOptions(def),
		Clash:            clashesIn(read, h.act, definitionName, env.ProjectRoot),
		SuggestedName:    definitionName + "-" + h.projectNameIn(env.ProjectRoot),
		NewWorkspaceName: definitionName,
		Workspaces:       h.workspaceChoices(read, cat.TeamYAML(definitionName, layer), env.ProjectRoot),
	}
	if layer == source.LayerGlobal {
		out.Projects = h.projectChoices(definitionName)
	}
	out.LastWorkspace = h.lastWorkspaceOf(read, cat.TeamYAML(definitionName, layer))
	return out, nil
}

// placeLayer is the layer a screen's place names: the project's own layer, or
// the global one. A screen reads "project" and "global"; the model calls the
// first "local" (source.LayerLocal).
func placeLayer(place string) (string, error) {
	switch place {
	case "project", source.LayerLocal:
		return source.LayerLocal, nil
	case source.LayerGlobal:
		return source.LayerGlobal, nil
	default:
		return "", hqRefuse(http.StatusUnprocessableEntity, i18n.T("error.project_place"))
	}
}

// globalOptions is the start sheet before a global definition's project is
// chosen: the projects that can take it. A project that defines the same name
// itself cannot (spec 5 章).
func (h *hqRead) globalOptions(definitionName string) web.HQLaunchOptions {
	return web.HQLaunchOptions{
		Definition: definitionName, Place: source.LayerGlobal,
		Members: []web.HQMember{}, Clash: []web.HQClash{}, Workspaces: []web.HQWorkspaceChoice{},
		SuggestedName: definitionName, NewWorkspaceName: definitionName,
		Projects: h.projectChoices(definitionName),
	}
}

// projectChoices is the projects a global definition can be started in: one that
// defines the same name itself cannot (spec 5 章). The reason sentence is what
// the sheet writes beside a project that cannot take it.
func (h *hqRead) projectChoices(definitionName string) []web.HQLaunchProject {
	out := []web.HQLaunchProject{}
	for _, project := range h.shownProjects() {
		env, err := h.act.env.AtProject(project.Path)
		if err != nil {
			continue
		}
		cat := env.Catalog(source.Disk{})
		one := web.HQLaunchProject{Path: project.Path, Name: project.Name, Available: true}
		if cat.HoldsDefinition(definitionName, source.LayerLocal) {
			one.Available = false
			one.Reason = i18n.Tf("web.hq.project_shadows", project.Name, project.Name, definitionName)
		}
		out = append(out, one)
	}
	return out
}

// shownProjects is the projects the headquarters shows: the listed ones, or the
// project serve started in when the config names none.
func (h *hqRead) shownProjects() []web.HQProject {
	projects, err := h.Projects()
	if err != nil {
		return nil
	}
	if projects.Mode == "current" {
		return []web.HQProject{projects.Current}
	}
	out := make([]web.HQProject, 0, len(projects.Projects))
	for _, one := range projects.Projects {
		if one.Listed {
			out = append(out, one)
		}
	}
	return out
}

// memberOptions is a definition's members, as the start sheet names them.
func memberOptions(def *definition.Definition) []web.HQMember {
	if def == nil || def.Graph == nil {
		return []web.HQMember{}
	}
	out := make([]web.HQMember, 0, len(def.Graph.Members))
	for _, member := range def.Graph.Members {
		out = append(out, web.HQMember{Name: member.Name, Herdr: def.Name + "-" + member.Name})
	}
	return out
}

// clashesIn is the running teams already using one name, read from the herdr
// read the caller just made: a clash is a decision, so it does not stand on the
// five-second memory (r-b7e1).
func clashesIn(read herdrNow, act *activityRead, name, _ string) []web.HQClash {
	out := []web.HQClash{}
	if read.err != nil {
		return out
	}
	for _, one := range act.teamsOf(read) {
		if one.Name == name {
			out = append(out, web.HQClash{Team: one.Name, Project: one.Project})
		}
	}
	return out
}

// workspaceChoices is the start sheet's workspaces in the order spec 5 章 gives:
// the one it last ran in, the ones starting with the project's name, the other
// free ones, and the ones with a team (which cannot be chosen).
func (h *hqRead) workspaceChoices(read herdrNow, teamYAML, project string) []web.HQWorkspaceChoice {
	last := h.lastWorkspaceOf(read, teamYAML)
	busy := map[string]runningTeam{}
	for _, one := range h.act.teamsOf(read) {
		if one.WorkspaceID != "" {
			busy[one.WorkspaceID] = one
		}
	}
	prefix := filepath.Base(project)
	choices := make([]web.HQWorkspaceChoice, 0, len(read.spaces))
	for _, space := range read.spaces {
		choice := web.HQWorkspaceChoice{
			HQWorkspace: web.HQWorkspace{ID: space.ID, Label: space.Label, Focused: space.Focused},
		}
		if one, taken := busy[space.ID]; taken {
			choice.Team, choice.TeamProject = one.Name, one.Project
			choice.Why = "busy"
			choices = append(choices, choice)
			continue
		}
		choice.Selectable = true
		switch {
		case last != nil && last.ID == space.ID:
			choice.Why = "last"
		case strings.HasPrefix(space.Label, prefix):
			choice.Why = "project"
		default:
			choice.Why = "free"
		}
		choices = append(choices, choice)
	}
	rank := map[string]int{"last": 0, "project": 1, "free": 2, "busy": 3}
	sort.SliceStable(choices, func(i, j int) bool {
		if rank[choices[i].Why] != rank[choices[j].Why] {
			return rank[choices[i].Why] < rank[choices[j].Why]
		}
		return choices[i].Label < choices[j].Label
	})
	return choices
}

// projectEnv is one project the headquarters may act on.
func (h *hqRead) projectEnv(path string) (team.Env, error) {
	allowed, err := h.allowedProjects()
	if err != nil {
		return team.Env{}, hqError(err)
	}
	want, err := team.ExpandProjectPath(path)
	if err != nil {
		return team.Env{}, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	for _, one := range allowed {
		if samePath(one, want) {
			return h.act.env.AtProject(want)
		}
	}
	return team.Env{}, hqRefuse(http.StatusUnprocessableEntity, i18n.T("web.error.project_unknown"))
}

// StartLaunch is POST /api/hq/launch: the start is checked here, and goes on in
// the background. The answer is the state the polling reads.
func (h *hqRead) StartLaunch(in web.HQLaunchIn) (web.HQLaunch, error) {
	env, err := h.startProjectEnv(in.Project)
	if err != nil {
		return web.HQLaunch{}, err
	}
	if err := h.startAllowed(in, env); err != nil {
		return web.HQLaunch{}, err
	}
	name, err := h.startName(in, env)
	if err != nil {
		return web.HQLaunch{}, err
	}
	// Build the team before a new workspace is made: a definition that does not
	// read must not leave a workspace behind.
	r, err := buildForStart(env, in, name)
	if err != nil {
		return web.HQLaunch{}, err
	}
	workspace, err := h.startWorkspace(in.Workspace, env)
	if err != nil {
		return web.HQLaunch{}, err
	}
	op := h.newOp(web.HQLaunch{
		Team: name, Definition: in.Definition, Place: in.Place,
		Project: env.ProjectRoot, ProjectName: h.projectNameIn(env.ProjectRoot),
		Workspace: workspace, Tab: web.HQTab{Label: name},
		Phase: "starting", Steps: []web.HQLaunchStep{},
	})
	ctx, cancel := context.WithCancel(h.context())
	op.mu.Lock()
	op.cancel = cancel
	op.mu.Unlock()
	// A read held before this start would hide the tab for up to five seconds;
	// the next read sees herdr as it is now, and the tab's own creation drops the
	// cache again.
	h.act.forgetHerdr()
	go h.runStart(ctx, op, env, r, in, workspace)
	return op.snapshot(), nil
}

// startProjectEnv is a project a start may run in: one config.yaml lists, or the
// project serve started in when the config lists none. A start is the
// headquarters acting on a listed project, not a way to reach any directory.
func (h *hqRead) startProjectEnv(path string) (team.Env, error) {
	paths, present, err := h.listedProjects()
	if err != nil {
		return team.Env{}, hqError(err)
	}
	if !present {
		return h.projectEnv(h.act.env.ProjectRoot)
	}
	want, err := team.ExpandProjectPath(path)
	if err != nil {
		return team.Env{}, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	for _, one := range paths {
		if samePath(one, want) {
			return h.act.env.AtProject(want)
		}
	}
	return team.Env{}, hqRefuse(http.StatusUnprocessableEntity, i18n.T("web.error.project_unknown"))
}

// startAllowed refuses a global definition in a project whose own layer holds
// the same name: that project's definition is the one hergy would use, so the
// start would not be the definition the sheet named (spec 5 章).
func (h *hqRead) startAllowed(in web.HQLaunchIn, env team.Env) error {
	layer, err := placeLayer(in.Place)
	if err != nil {
		return err
	}
	if layer != source.LayerGlobal {
		return nil
	}
	if env.Catalog(source.Disk{}).HoldsDefinition(in.Definition, source.LayerLocal) {
		return hqRefuse(http.StatusUnprocessableEntity, i18n.Tf("web.hq.project_shadows", h.projectNameIn(env.ProjectRoot), h.projectNameIn(env.ProjectRoot), in.Definition))
	}
	return nil
}

// startName is the name the team runs under, checked against the agents and
// teams that are live: a name taken is refused with the name hergy suggests
// (hergy up --as).
func (h *hqRead) startName(in web.HQLaunchIn, env team.Env) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = in.Definition
	}
	suggested := in.Definition + "-" + h.projectNameIn(env.ProjectRoot)
	if h.nameRunning(name) {
		return "", &web.HQError{
			Status:        http.StatusConflict,
			Text:          i18n.Tf("web.hq.name_taken", name),
			SuggestedName: suggested,
		}
	}
	return name, nil
}

// nameRunning reports whether a team of this name, or an agent of it, is live.
func (h *hqRead) nameRunning(name string) bool {
	read := h.act.readHerdrFresh(h.act.clock())
	if read.err == nil {
		for _, one := range h.act.teamsOf(read) {
			if one.Name == name {
				return true
			}
		}
	}
	agents, err := herdrAgents()
	if err != nil {
		return false
	}
	for _, agent := range agents {
		if protocol.NamedForTeam(name, agent.Name) {
			return true
		}
	}
	return false
}

// startWorkspace is the workspace a start asks for: one that is there and free,
// or one to make with this label (spec 5 章).
func (h *hqRead) startWorkspace(in web.HQWorkspaceIn, env team.Env) (web.HQWorkspace, error) {
	port := herdrcli.New(os.Getenv("HERGY_HERDR"))
	spaces, err := herdr.New(port).Workspaces()
	if err != nil {
		return web.HQWorkspace{}, hqRefuse(http.StatusServiceUnavailable, err.Error())
	}
	if label := strings.TrimSpace(in.New); label != "" {
		for _, space := range spaces {
			if space.Label == label {
				return web.HQWorkspace{}, hqRefuse(http.StatusConflict, i18n.Tf("web.hq.workspace_taken", label))
			}
		}
		space, err := herdr.New(port).CreateWorkspace(label, env.ProjectRoot)
		if err != nil {
			return web.HQWorkspace{}, hqRefuse(http.StatusServiceUnavailable, err.Error())
		}
		return web.HQWorkspace{ID: space.ID, Label: space.Label}, nil
	}
	for _, space := range spaces {
		if space.ID != in.ID {
			continue
		}
		read := h.act.readHerdrFresh(h.act.clock())
		for _, one := range h.act.teamsOf(read) {
			if one.WorkspaceID == space.ID {
				return web.HQWorkspace{}, hqRefuse(http.StatusConflict, i18n.Tf("web.hq.workspace_busy", space.Label, one.Name))
			}
		}
		return web.HQWorkspace{ID: space.ID, Label: space.Label}, nil
	}
	return web.HQWorkspace{}, hqRefuse(http.StatusUnprocessableEntity, i18n.Tf("web.hq.workspace_missing", in.ID))
}

// buildForStart builds the team at run time, from the definition's layer.
func buildForStart(env team.Env, in web.HQLaunchIn, name string) (*resolved, error) {
	cat := env.Catalog(source.Disk{})
	layer, err := placeLayer(in.Place)
	if err != nil {
		return nil, err
	}
	def, err := cat.Definition(in.Definition, layer)
	if err != nil {
		return nil, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	r, err := buildResolved(cat, env.ProjectRoot, def, name)
	if err != nil {
		return nil, hqRefuse(http.StatusUnprocessableEntity, err.Error())
	}
	return r, nil
}

// context is the serve's own lifetime, or a background one for a reading built
// by a test.
func (h *hqRead) context() context.Context {
	if h.act.ctx != nil {
		return h.act.ctx
	}
	return context.Background()
}

// newOp stores one start.
func (h *hqRead) newOp(launch web.HQLaunch) *launchOp {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	launch.ID = fmt.Sprintf("l-%d", h.seq)
	op := &launchOp{launch: launch}
	h.ops[launch.ID] = op
	h.order = append(h.order, launch.ID)
	h.forgetOld()
	return op
}

// forgetOld drops the starts that ended long ago: the polling of a closed sheet
// is the only reader, and the memory is not a record (api.md 4).
func (h *hqRead) forgetOld() {
	cut := time.Now().Add(-10 * time.Minute)
	kept := h.order[:0]
	for _, id := range h.order {
		op := h.ops[id]
		op.mu.Lock()
		old := !op.finished.IsZero() && op.finished.Before(cut)
		op.mu.Unlock()
		if old {
			delete(h.ops, id)
			continue
		}
		kept = append(kept, id)
	}
	h.order = kept
}

// snapshot is one start as it is now.
func (op *launchOp) snapshot() web.HQLaunch {
	op.mu.Lock()
	defer op.mu.Unlock()
	out := op.launch
	out.Steps = append([]web.HQLaunchStep{}, op.launch.Steps...)
	return out
}

// LaunchState is GET /api/hq/launches/{id}.
func (h *hqRead) LaunchState(id string) (web.HQLaunch, bool) {
	h.mu.Lock()
	op, ok := h.ops[id]
	h.mu.Unlock()
	if !ok {
		return web.HQLaunch{}, false
	}
	return op.snapshot(), true
}

// CancelLaunch is DELETE /api/hq/launches/{id}: stop waiting, leave the tab and
// panes for down to put away, as Ctrl-C does on the command line.
func (h *hqRead) CancelLaunch(id string) (web.HQLaunch, bool) {
	h.mu.Lock()
	op, ok := h.ops[id]
	h.mu.Unlock()
	if !ok {
		return web.HQLaunch{}, false
	}
	op.mu.Lock()
	if op.launch.Phase == "starting" || op.launch.Phase == "awaiting" {
		if op.cancel != nil {
			op.cancel()
		}
		op.launch.Phase = "cancelled"
		op.launch.Waiting = nil
		op.finished = time.Now()
	}
	out := op.launch
	out.Steps = append([]web.HQLaunchStep{}, op.launch.Steps...)
	op.mu.Unlock()
	h.act.forgetHerdr()
	return out, true
}

// activeLaunch is the start this serve is carrying out for one team, if any.
func (h *hqRead) activeLaunch(teamName string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range h.order {
		op := h.ops[id]
		op.mu.Lock()
		match := op.launch.Team == teamName &&
			(op.launch.Phase == "starting" || op.launch.Phase == "awaiting")
		op.mu.Unlock()
		if match {
			return id, true
		}
	}
	return "", false
}

// runStart runs one start in the background and keeps its progress.
func (h *hqRead) runStart(ctx context.Context, op *launchOp, env team.Env, r *resolved, in web.HQLaunchIn, workspace web.HQWorkspace) {
	l := newLauncher(r, 30000, io.Discard).WithContext(ctx)
	l.Observe(func(event launcher.Event) { h.observe(op, event) })
	command := startCommand(in.Definition, r.team.Name)
	err := l.BootIn(workspace.ID, command)
	if errors.Is(err, launcher.ErrCancelled) || ctx.Err() != nil {
		h.finish(op, "cancelled", "")
		return
	}
	if err != nil {
		h.finish(op, "failed", err.Error())
		return
	}
	label := workspace.Label
	if label == "" {
		label = l.WorkspaceLabel()
	}
	recordLaunchIn(io.Discard, env, r, launchlog.CommandUp, r.starts, launchlog.Workspace{Label: label, ID: l.WorkspaceID()})
	h.finish(op, "done", "")
}

// startCommand is the hergy up the start is the same as, made from the
// definition and the name it runs under. A startup question and its progress use
// the one function, so the two never name a different command (r-b7e1).
func startCommand(definition, name string) string {
	command := "hergy up " + definition
	if name != definition {
		command += " --as " + name
	}
	return command
}

// observe writes one step of the start, and where a startup question waits. The
// first event that names the tab drops the herdr memory: the next read of the
// activity and the headquarters sees the new tab (r-b7e1).
func (h *hqRead) observe(op *launchOp, event launcher.Event) {
	op.mu.Lock()
	if op.launch.Phase == "cancelled" || op.launch.Phase == "done" || op.launch.Phase == "failed" {
		op.mu.Unlock()
		return
	}
	fresh := false
	if event.TabID != "" && op.launch.Tab.ID != event.TabID {
		op.launch.Tab.ID = event.TabID
		fresh = true
	}
	if event.Waiting != nil {
		op.launch.Phase = "awaiting"
		op.launch.Waiting = &web.HQWaiting{
			Member: event.Waiting.Member, Herdr: event.Waiting.Herdr,
			Workspace: event.Waiting.Workspace, Tab: event.Waiting.Tab,
			Pane: event.Waiting.Pane,
		}
		op.launch.Waiting.Command = startCommand(op.launch.Definition, op.launch.Team)
	} else if op.launch.Phase == "awaiting" {
		op.launch.Phase = "starting"
		op.launch.Waiting = nil
	}
	if event.CreatedTab != "" {
		op.stepLocked("done", i18n.Tf("web.hq.tab_created", event.CreatedTab))
	}
	if event.CreatedPanes > 0 {
		op.stepLocked("done", i18n.Tf("web.hq.panes_created", event.CreatedPanes))
	}
	if event.Stage != "" {
		op.stepLocked("stage", event.Stage)
	}
	if event.State == i18n.T("progress.mark.bootstrap_submitted") && event.Member != "" {
		op.stepLocked("member", i18n.Tf("web.hq.member_started", event.Member))
	}
	op.mu.Unlock()
	if fresh {
		h.act.forgetHerdr()
	}
}

// stepLocked appends one line unless it is the one it just wrote.
func (op *launchOp) stepLocked(kind, text string) {
	if n := len(op.launch.Steps); n > 0 && op.launch.Steps[n-1].Text == text {
		return
	}
	op.launch.Steps = append(op.launch.Steps, web.HQLaunchStep{At: time.Now().Format(time.RFC3339), Kind: kind, Text: text})
}

// finish sets a start's last state, unless it was given up already, and drops the
// herdr memory so the next read sees the team as it stands (r-b7e1).
func (h *hqRead) finish(op *launchOp, phase, reason string) {
	op.mu.Lock()
	if op.launch.Phase == "cancelled" {
		op.mu.Unlock()
		h.act.forgetHerdr()
		return
	}
	op.launch.Phase = phase
	op.launch.Error = reason
	op.launch.Waiting = nil
	op.finished = time.Now()
	if phase == "done" {
		op.stepLocked("done", i18n.Tf("web.hq.started", op.launch.Team))
	}
	op.mu.Unlock()
	h.act.forgetHerdr()
}
