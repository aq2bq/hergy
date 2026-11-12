package ops

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"os"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launcher"
	"github.com/aq2bq/hergy/internal/screenlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/turns"
)

// StopInput describes execution, not presentation. A screen supplies its port
// and cancellation adapter; the CLI requires a managed pane. PlanOnly and Dry
// pass through the same resolution and leftover reading as execution.
type StopInput struct {
	Env             team.Env
	Target          StopTarget
	Layer           string
	Timeout         int
	SelfPane        domainname.PaneID
	RequirePane     bool
	ManagedPane     bool
	Dry             bool
	PlanOnly        bool
	Port            herdr.Port
	Now             func() time.Time
	Pause           func(time.Duration)
	PendingLaunches func(func(PendingLaunch))
	Observe         func(StopEvent)
}

// StopEvent lets a caller display a plan and launcher text before a pane closes.
// Failure is emitted before cleanup; no event writes to a process stream.
type StopEvent struct {
	Plan    *StopPlan
	Text    string
	Failure error
}

// StopPlan is the input's resolved provenance and names, shared by text, JSON
// and the screen's confirmation. Built is absent when closing needs no definition.
type StopPlan struct {
	Team         team.Running
	Built        *Resolved
	Reason       error
	Stopped      bool
	Worker       *StoppedWorker
	Project      domainplace.Project
	Members      []domainname.Seen
	MemberLabels []string
	Workers      []domainname.Seen
	Owner        LeftoverOwner
}

// StoppedWorker identifies the single worker selected by down.
type StoppedWorker struct{ Team, Herdr domainname.Seen }

// StopClosed counts successful pane closes. The watcher is separate so each
// entry point can select the counts it presents without altering the API.
type StopClosed struct{ Members, Workers, Watchers int }

// StopResult carries display text from the lower-level launcher without writing
// to a process stream. OperationErr distinguishes an execution failure from a
// later cleanup failure, preserving the CLI's reporting and exit codes.
type StopResult struct {
	Plan         StopPlan
	Closed       StopClosed
	Output       string
	PluginNote   string
	ShowHeader   bool
	OperationErr error
	Started      bool
	Leftovers    team.Leftovers
	HasLeftovers bool
}

// StopTeam owns preconditions, execution and cleanup for every whole-team stop.
// Only success and name-release timeout end the run's turn records and cancel
// a pending screen launch. Other errors and cancellation retain them (d-aad0).
func StopTeam(ctx context.Context, in StopInput) (out StopResult, err error) {
	if in.Target.team.Team.String() == "" {
		return out, i18n.Errorf("error.no_running_teams")
	}
	if in.Target.worker != nil {
		return out, i18n.Errorf("error.unhandled_command")
	}
	out.Plan = teamStopPlan(in)
	plan := &out.Plan
	dirs := teamPaneDirs(in.Target.tabs, plan.Team.Team.Observation())
	if in.PlanOnly {
		out.Leftovers = team.GitLeftovers(plan.Project, plan.Owner.Since, dirs)
		out.HasLeftovers = true
		out.ShowHeader = plan.Reason != nil
		if out.ShowHeader {
			stopEvent(in, StopEvent{Plan: plan})
		}
		return out, nil
	}
	// The old CLI printed a broken definition before the pane guard, and a
	// stopped-team plugin warning before it. Retain that output ordering.
	out.ShowHeader = plan.Reason != nil
	if out.ShowHeader {
		stopEvent(in, StopEvent{Plan: plan})
	}
	if plan.Stopped || plan.Reason != nil {
		out.PluginNote = PluginNote()
		stopEvent(in, StopEvent{Text: out.PluginNote})
	}
	if !in.Dry && in.RequirePane && (!in.ManagedPane || in.SelfPane == "") {
		return out, i18n.Errorf("error.not_under_herdr")
	}
	if !out.ShowHeader {
		out.ShowHeader = true
		stopEvent(in, StopEvent{Plan: plan})
	}
	if !plan.Stopped && plan.Reason == nil {
		out.PluginNote = PluginNote()
		stopEvent(in, StopEvent{Text: out.PluginNote})
	}
	text := &stopText{observe: in.Observe}
	l := stopLauncher(ctx, in, plan, text).WithContext(ctx)
	var closeErr error
	switch {
	case plan.Stopped:
		_, closeErr = l.DownStopped(plan.Team.Team.Observation(), in.SelfPane, in.Dry)
	case plan.Built == nil:
		_, closeErr = l.DownByName(plan.Members, in.SelfPane, in.Dry)
	default:
		_, closeErr = l.Down(plan.Built.Starts, in.SelfPane, in.Dry)
	}
	out.Output, out.Closed = text.String(), closedOf(l.ClosedPanes())
	if in.Dry {
		if closeErr == nil {
			out.Leftovers = team.GitLeftovers(plan.Project, plan.Owner.Since, dirs)
			out.HasLeftovers = true
		}
		return out, closeErr
	}
	out.Started, out.OperationErr = true, closeErr
	if ctx.Err() != nil {
		return out, context.Canceled
	}
	if closeErr != nil {
		stopEvent(in, StopEvent{Failure: closeErr})
	}
	if closeErr != nil {
		if _, timeout := errors.AsType[*launcher.TeamDownTimeout](closeErr); !timeout {
			return out, closeErr
		}
	}
	out.Leftovers = team.GitLeftovers(plan.Project, plan.Owner.Since, dirs)
	out.HasLeftovers = true
	cleanupErr := clearTurn(plan.Team.Team.String())
	finishLaunchFor(plan.Team.Team.Observation(), in.PendingLaunches, in.Now)
	if cleanupErr != nil {
		return out, cleanupErr
	}
	return out, closeErr
}

func teamStopPlan(in StopInput) StopPlan {
	chosen := in.Target.team
	project := stopProject(chosen.Project, in.Env.ProjectRoot)
	plan := StopPlan{Team: chosen, Project: project, Stopped: in.Target.stopped,
		Workers: workerNamesOf(in.Target.tabs, chosen.Team.Observation()), Owner: teamLeftovers(project, chosen.Team.String())}
	plan.Built, plan.Reason = BuildRunningIn(in.Env.Catalog(source.Disk{}), in.Env.ProjectRoot, chosen, in.Layer)
	if plan.Built != nil {
		for _, start := range plan.Built.Starts {
			plan.MemberLabels = append(plan.MemberLabels, start.Short.String())
		}
	} else {
		for _, member := range team.MemberAgents(in.Target.tabs, chosen.Team.Observation()) {
			plan.MemberLabels = append(plan.MemberLabels, member.String())
		}
	}
	if plan.Stopped {
		plan.Members = team.MemberPanes(in.Target.tabs, chosen.Team.Observation())
		plan.Reason = nil
		return plan
	}
	if plan.Reason != nil {
		plan.Members = team.MemberAgents(in.Target.tabs, chosen.Team.Observation())
		return plan
	}
	for _, start := range plan.Built.Starts {
		plan.Members = append(plan.Members, start.Name.Observation())
	}

	return plan
}

func stopLauncher(ctx context.Context, in StopInput, plan *StopPlan, text io.Writer) *launcher.Launcher {
	r := plan.Built
	if r == nil {
		r = &Resolved{Team: team.Team{Name: plan.Team.Team}}
	}
	port := in.Port
	if port == nil {
		port = herdrcli.New(os.Getenv("HERGY_HERDR"))
	}
	return NewLauncherWithPort(r, in.Timeout, text, stopPort{Port: port, ctx: ctx}).WithWaitClock(in.Now, stopPause(ctx, in.Pause))
}

// StopWorker closes one worker and lists its leftovers; it never clears the
// team's turn or cancels the team's pending launch.
func StopWorker(ctx context.Context, in StopInput) (out StopResult, err error) {
	worker := in.Target.worker
	if worker == nil {
		return out, i18n.Errorf("error.unhandled_command")
	}
	project := stopProject(in.Target.team.Project, in.Env.ProjectRoot)
	out.Plan = StopPlan{Team: team.Running{Team: domainname.TeamFromObservation(worker.Team)},
		Project: project, Worker: &StoppedWorker{Team: worker.Team, Herdr: worker.Herdr},
		Workers: []domainname.Seen{worker.Herdr}, Owner: workerLeftovers(project, worker.Team.String(), worker.Herdr.String())}
	dirs := workerPaneDirs(in.Target.tabs, worker.Label)
	if in.PlanOnly {
		out.Leftovers = team.GitLeftovers(project, out.Plan.Owner.Since, dirs)
		out.HasLeftovers = true
		return out, nil
	}
	out.ShowHeader = true
	stopEvent(in, StopEvent{Plan: &out.Plan})
	out.PluginNote = PluginNote()
	stopEvent(in, StopEvent{Text: out.PluginNote})
	if !in.Dry && in.RequirePane && (!in.ManagedPane || in.SelfPane == "") {
		return out, i18n.Errorf("error.not_under_herdr")
	}
	text := &stopText{observe: in.Observe}
	l := stopLauncher(ctx, in, &out.Plan, text).WithContext(ctx)
	_, err = l.DownWorker(worker.Label, in.SelfPane, in.Dry)
	out.Output, out.Closed = text.String(), closedOf(l.ClosedPanes())
	out.Started = !in.Dry
	if !in.Dry {
		out.OperationErr = err
	}
	if ctx.Err() != nil {
		return out, context.Canceled
	}
	if err != nil && !in.Dry {
		stopEvent(in, StopEvent{Failure: err})
	}
	if err == nil {
		out.Leftovers = team.GitLeftovers(project, out.Plan.Owner.Since, dirs)
		out.HasLeftovers = true
	}
	return out, err
}

func clearTurn(teamName string) error {
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	return turns.Clear(dir, teamName)
}

func closedOf(panes []launcher.ClosedPane) StopClosed {
	var count StopClosed
	for _, pane := range panes {
		switch {
		case pane.Watcher:
			count.Watchers++
		case pane.Worker:
			count.Workers++
		default:
			count.Members++
		}
	}
	return count
}

// stopProject uses the run's project whenever known. Only an unrecorded run
// falls back to the caller's project (stage C decision 2).
func stopProject(teamProject, callerProject domainplace.Project) domainplace.Project {
	if teamProject.IsZero() {
		return callerProject
	}
	return teamProject
}

func stopEvent(in StopInput, event StopEvent) {
	if in.Observe != nil {
		in.Observe(event)
	}
}

type stopText struct {
	bytes.Buffer
	observe func(StopEvent)
}

func (w *stopText) Write(text []byte) (int, error) {
	if w.observe != nil {
		w.observe(StopEvent{Text: string(text)})
	}
	return w.Buffer.Write(text)
}

func (w *stopText) WriteString(text string) (int, error) { return w.Write([]byte(text)) }
