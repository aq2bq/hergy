package ops

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/protocol"

	"github.com/aq2bq/hergy/internal/check"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
)

// Refusal identifies a precondition for an adapter's error presentation.
type Refusal struct {
	Kind              string
	At, SuggestedName string
	Err               error
}

func (e *Refusal) Error() string          { return e.Err.Error() }
func (e *Refusal) Unwrap() error          { return e.Err }
func refuse(kind string, err error) error { return &Refusal{Kind: kind, Err: err} }

// WorkspaceInput preserves the current-workspace CLI policy and the screen's
// explicit free workspace or new label as different inputs.
type WorkspaceInput struct {
	Current bool
	ID, New string
}

// StartInput is a team start, shared by CLI and screens.
type StartInput struct {
	Locks                         *OperationLocks
	Env                           team.Env
	Definition                    domainname.DefinitionName
	Name                          string
	Place                         domainplace.Selection
	Workspace                     WorkspaceInput
	ProjectName                   string
	Command                       string
	RequirePane, ManagedPane, Dry bool
	Timeout                       int
	Port                          Port
	Observe                       func(Event)
}

// StartPlan is published only once all start preconditions hold and the start
// lock is owned. The operation retains that lock until boot and recording end.
type StartPlan struct {
	Resolved  *Resolved
	Workspace herdr.Workspace
	Command   string
}

// StartTeam owns definition checks, its start lock, name collision checking,
// workspace selection, boot, cancellation and the confirmed-recipient record.
func StartTeam(ctx context.Context, in StartInput) (*Plan, error) {
	r, err := resolveStart(in)
	if err != nil {
		return nil, err
	}
	in.Definition = r.Team.Definition.Name
	plan := &Plan{Resolved: r, Members: r.Starts, Mode: "boot", Next: "up " + r.Team.Definition.Name.String()}
	if r.Team.Name.String() != r.Team.Definition.Name.String() {
		plan.Next += " --as " + r.Team.Name.String()
	}
	if in.Dry {
		return plan, nil
	}
	release, err := source.LockTeamStart(r.Team.Definition.TeamYAML.String())
	if err != nil {
		return nil, refuse("conflict", i18n.Errorf("web.rename.busy"))
	}
	defer release()
	r, err = resolveStart(in)
	if err != nil {
		return nil, err
	}
	plan.Resolved, plan.Members = r, r.Starts
	if in.RequirePane && (!in.ManagedPane || CurrentPane() == "") {
		return nil, i18n.Errorf("error.not_under_herdr")
	}
	if _, err = startName(in, true); err != nil {
		return nil, err
	}
	child, releaseOperation, err := in.Locks.acquire(ctx, []string{r.Team.Name.String()})
	if err != nil {
		return nil, err
	}
	defer releaseOperation()
	ctx = child
	workspace := herdr.Workspace{}
	if !in.Workspace.Current {
		workspace, err = ChooseWorkspace(in.Env, in.Workspace, in.Port)
		if err != nil {
			return nil, err
		}
	}
	command := in.Command
	if command == "" {
		command = StartCommand(in.Definition.String(), r.Team.Name.String())
	}
	emit(in.Observe, Event{Start: &StartPlan{Resolved: r, Workspace: workspace, Command: command}})
	emit(in.Observe, Event{Plan: plan})
	for _, note := range SignalNotes(r.Starts) {
		emit(in.Observe, Event{Text: note})
	}
	l := operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe).WithWatcher(watcherCommand(r.Team.Name.String(), in.Env.ProjectRoot))
	transform := func(err error) error {
		if launcher.IsUndelivered(err) {
			return err
		}
		f := l.Failure(err)
		project := in.ProjectName
		if project == "" {
			project = filepath.Base(in.Env.ProjectRoot.String())
		}
		return &LaunchFailure{Failed: f, Project: project, Command: FailedStartCommand(f, r.Team.Name.String(), command)}
	}
	err = execute(ctx, l, in.Observe, transform, func() error {
		if in.Workspace.Current {
			return l.Boot(command)
		}
		return l.BootIn(workspace.ID, command)
	})
	if ctx.Err() == nil && (err == nil || launcher.IsUndelivered(err)) {
		label := workspace.Label
		if label == "" {
			label = l.WorkspaceLabel()
		}
		warning(in.Observe, RecordReceivedLaunch(in.Env, r, launchlog.CommandUp, r.Starts, l, launchlog.Workspace{Label: label, ID: l.WorkspaceID()}))
	}
	return plan, err
}

func resolveStart(in StartInput) (*Resolved, error) {
	cat := in.Env.Catalog(source.Disk{})
	layer := ""
	if in.Place != "" {
		layer = in.Place.Layer()
	}
	if err := sharedAgreementStops(cat); err != nil {
		return nil, refuse("invalid", err)
	}
	if in.Definition.String() == "" {
		def, err := cat.Definition("", layer)
		if err != nil {
			return nil, refuse("invalid", err)
		}
		in.Definition = def.Name
	}
	name, err := startName(in, false)
	if err != nil {
		return nil, err
	}
	build := func() (*Resolved, error) {
		def, err := cat.Definition(in.Definition.String(), layer)
		if err != nil {
			return nil, refuse("invalid", err)
		}
		r, err := BuildResolved(cat, in.Env.ProjectRoot, def, name)
		if err != nil {
			return nil, refuse("invalid", err)
		}
		return r, nil
	}
	if in.Place == domainplace.Selection(source.LayerGlobal) && cat.HoldsDefinition(in.Definition.String(), source.LayerLocal) {
		return nil, refuse("invalid", i18n.Errorf("web.hq.project_shadows", in.ProjectName, in.ProjectName, in.Definition))
	}
	r, err := build()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func sharedAgreementStops(cat *catalog.Catalog) error {
	docs := cat.SharedAgreements()
	if len(docs) == 0 {
		return nil
	}
	return errors.New(check.SharedAgreementText(cat, docs))
}

// CheckStartName is a read-only form of the operation's name precondition.
func CheckStartName(in StartInput) (string, error) { return startName(in, true) }
func startName(in StartInput, checkRunning bool) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = in.Definition.String()
	}
	if _, err := domainname.NewTeamName(name); strings.TrimSpace(in.Name) != "" && err != nil {
		return "", &Refusal{Kind: "invalid", At: "name", Err: i18n.Errorf("error.team_name_pattern", name)}
	}
	if checkRunning && nameRunning(name, ReadSession(in.Port)) {
		return "", &Refusal{Kind: "conflict", At: "name", SuggestedName: in.Definition.String() + "-" + in.ProjectName, Err: i18n.Errorf("error.team_name_taken", name)}
	}
	return name, nil
}

func nameRunning(name string, read Session) bool {
	if read.Err != nil {
		return false
	}
	running := team.RunningTeams(read.Tabs, LaunchTeams(), read.AgentsRead)
	prospective, _ := domainname.ParseDefinitionName(name)
	candidate := domainname.DefinitionNames(prospective)
	var known []domainname.Seen
	for _, one := range running {
		known = append(known, one.Team.Observation())
		if one.Team.String() == name {
			return true
		}
	}
	for _, agent := range read.Agents {
		if !candidate.Contains(agent.Name.String()) {
			continue
		}
		owner, _, ok := domainname.Resolve(agent.Name, known)
		if !ok || len(owner.String()) <= len(name) {
			return true
		}
	}
	return false
}

// ChooseWorkspace resolves the screen's explicit choice, rejecting occupied workspaces.
func ChooseWorkspace(env team.Env, in WorkspaceInput, port Port) (herdr.Workspace, error) {
	h := herdr.New(portOrDefault(port))
	spaces, err := h.Workspaces()
	if err != nil {
		return herdr.Workspace{}, refuse("unavailable", err)
	}
	if label := strings.TrimSpace(in.New); label != "" {
		for _, space := range spaces {
			if space.Label == label {
				return herdr.Workspace{}, refuse("conflict", i18n.Errorf("web.hq.workspace_taken", label))
			}
		}
		space, err := h.CreateWorkspace(label, env.ProjectRoot.String())
		if err != nil {
			return herdr.Workspace{}, refuse("unavailable", err)
		}
		return space, nil
	}
	for _, space := range spaces {
		if space.ID != in.ID {
			continue
		}
		read := ReadSession(port)
		for _, one := range team.RunningTeams(read.Tabs, LaunchTeams(), read.AgentsRead) {
			for _, tab := range read.Tabs {
				if tab.WorkspaceID == space.ID && protocol.NamesTeam(tab.Label.String(), one.Team.String()) {
					return herdr.Workspace{}, refuse("conflict", i18n.Errorf("web.hq.workspace_busy", space.Label, one.Team))
				}
			}
		}
		return space, nil
	}
	return herdr.Workspace{}, refuse("invalid", i18n.Errorf("web.hq.workspace_missing", in.ID))
}

// StartCommand is the recovery command shared by startup progress and errors.
func StartCommand(definition, name string) string {
	command := "hergy up " + definition
	if name != definition {
		command += " --as " + name
	}
	return command
}
func watcherCommand(name string, project domainplace.Project) []string {
	return []string{Executable(), "turns", name, "-C", project.String(), "--lang", i18n.Language()}
}
