package ops

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/launcher"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/words"
)

// ApplyInput requests one complete apply under a named selection policy.
type ApplyInput struct {
	Now                           func() time.Time
	Env                           Environment
	Views                         []ProjectView
	read                          func() session
	Selection                     ApplySelectionInput
	Timeout                       int
	Dry, RequirePane, ManagedPane bool
	Port                          Port
	Observe                       func(Event)
	Locks                         *OperationLocks
	Accepted                      func(ApplySelection)
}

// ApplyResult holds the common plan, selection, and partial-policy exit result.
type ApplyResult struct {
	Plan       DeliveryPlan
	Selection  ApplySelection
	FinalError error
}

// Applied is a confirmed member and the way its changes were applied.
type Applied struct{ Member PlannedMember }

// Apply owns selection, reservation, execution and confirmed-recipient records.
func Apply(ctx context.Context, in ApplyInput) (ApplyResult, error) {
	var out ApplyResult
	if err := sharedAgreementStops(in.Env.Catalog(source.Disk{})); err != nil {
		return out, err
	}
	var read session
	if in.read != nil {
		read = in.read()
	} else {
		read = readSession(in.Port)
	}
	views := in.Views
	if views == nil {
		views = projectViews(in.Env, source.Disk{}, nil)
	}
	now := time.Now
	if in.Now != nil {
		now = in.Now
	}
	out.Plan = computeDelivery(deliveryInput{Views: views, Session: read, Now: now()})
	if !out.Plan.Apply.Herdr.OK {
		return out, refuse("unavailable", errors.New(out.Plan.Apply.Herdr.Text))
	}
	selection, err := selectApply(out.Plan, in.Selection)
	out.Selection = selection
	if err != nil {
		return out, err
	}
	if len(selection.Refusals) > 0 {
		return out, refuse("invalid", errors.New(strings.Join(selection.Refusals, "\n")))
	}
	out.FinalError = applyRemainingError(selection, in.Dry)
	if in.Dry || len(selection.Selected) == 0 {
		return out, nil
	}
	if in.RequirePane && (!in.ManagedPane || CurrentPane() == "") {
		return out, words.Errorf("runtime.managed-pane", i18n.Language())
	}
	child, release, err := in.Locks.acquire(ctx, selection.Teams())
	if err != nil {
		return out, err
	}
	defer release()
	ctx = child
	if in.Accepted != nil {
		in.Accepted(selection)
	}
	var missing []error
	for _, group := range groupByTeam(selection.Selected) {
		err := applyGroup(ctx, in, group)
		if err == nil {
			continue
		}
		if !launcher.IsUndelivered(err) {
			return out, err
		}
		missing = append(missing, err)
	}
	err = errors.Join(missing...)
	if err != nil {
		emit(in.Observe, Event{Failure: err})
		return out, &OperationError{Err: err}
	}
	return out, nil
}
func applyGroup(ctx context.Context, in ApplyInput, group []PlannedMember) error {
	env, err := in.Env.AtProject(group[0].Team.Project)
	if err != nil {
		return err
	}
	cat := env.Catalog(source.Disk{})
	r, err := buildRunningIn(cat, env.ProjectRoot, team.Running{Team: domainname.TeamFromObservation(group[0].Team.Name), Definition: group[0].Team.Definition}, group[0].Team.Place.Layer())
	if err != nil {
		return err
	}
	l := operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe).WithoutNextHint()
	var starts, resends []definition.Start
	for _, member := range group {
		if member.How == ApplyResend {
			resends = append(resends, member.Start)
		} else {
			starts = append(starts, member.Start)
		}
	}
	run := func() error { return applyChanges(l, r, env, group, starts, resends) }
	err = run()
	if ctx.Err() != nil || errors.Is(err, launcher.ErrCancelled) || (err != nil && !launcher.IsUndelivered(err)) {
		return execute(ctx, l, in.Observe, nil, func() error { return err })
	}
	var all []definition.Start
	for _, member := range group {
		all = append(all, member.Start)
		if l.Received(member.Start.Name.String()) {
			emit(in.Observe, Event{Applied: &Applied{Member: member}})
		}
	}
	warning(in.Observe, recordReceivedLaunch(env, r, launchlog.CommandApply, all, l, launchlog.Workspace{}))
	return err
}
func applyFix(group []PlannedMember) string {
	names := make([]string, 0, len(group))
	for _, member := range group {
		names = append(names, member.Start.Short.String())
	}
	return i18n.Tf("web.apply.command", strings.Join(names, " "))
}
func applyRemainingError(selection ApplySelection, dry bool) error {
	if len(selection.Remaining) > 0 {
		key := "apply.left_count"
		if dry {
			key = "apply.plan_left_count"
		}
		return i18n.Errorf(key, len(selection.Remaining))
	}
	if len(selection.Broken) > 0 {
		return i18n.Errorf("apply.broken_left", len(selection.Broken))
	}
	return nil
}

// projectViews reads the projects exposed by this environment.
func projectViews(env Environment, reader source.Reader, resolve func(domainplace.Project) (Environment, error)) []ProjectView {
	makeView := func(env Environment) ProjectView {
		return ProjectView{Path: env.ProjectRoot, Name: filepath.Base(env.ProjectRoot.String()), Env: env, Cat: env.Catalog(reader)}
	}
	cfg, err := env.ReadConfigFile()
	if err != nil {
		return []ProjectView{makeView(env)}
	}
	paths, present, err := env.ProjectPaths(cfg)
	if err != nil || !present {
		return []ProjectView{makeView(env)}
	}
	var views []ProjectView
	for _, path := range paths {
		var one Environment
		var err error
		if resolve != nil {
			one, err = resolve(path)
		} else {
			one, err = env.AtProject(path)
		}
		if err == nil {
			views = append(views, makeView(one))
		}
	}
	return nameProjects(views)
}

// nameProjects disambiguates equal project labels in one reading.
func nameProjects(views []ProjectView) []ProjectView {
	count := map[string]int{}
	for _, view := range views {
		count[view.Name]++
	}
	for i := range views {
		if count[views[i].Name] > 1 {
			views[i].Name = filepath.Join(filepath.Base(filepath.Dir(views[i].Path.String())), views[i].Name)
		}
	}
	return views
}

func applyChanges(l *launcher.Launcher, r *Resolved, env Environment, group []PlannedMember, starts, resends []definition.Start) error {
	var missing []error
	cat := env.Catalog(source.Disk{})

	if len(starts) > 0 {
		err := l.Refresh(starts, CurrentPane())
		if err != nil {
			if !launcher.IsUndelivered(err) {
				if errors.Is(err, launcher.ErrCancelled) {
					return err
				}
				f := StartFailure(l.Failure(err))
				if f.Member == "" && f.Panes == 0 {
					return err
				}
				return &LaunchFailure{Failed: f, Project: group[0].Team.ProjectName, Command: applyFix(group)}
			}
			missing = append(missing, err)
		}
	}
	if len(resends) > 0 {
		if err := l.Reprompt(promptsToResend(cat, env.ProjectRoot, r, resends)); err != nil {
			if !launcher.IsUndelivered(err) {
				return err
			}
			missing = append(missing, err)
		}
	}
	return errors.Join(missing...)
}
