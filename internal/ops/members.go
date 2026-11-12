package ops

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/launcher"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops/internal/spawn"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/words"
)

// MemberInput is an unresolved CLI member selection with an execution policy.
type MemberInput struct {
	Env                      Environment
	Team                     options.Input
	Member                   string
	All, Dry                 bool
	Timeout                  int
	RequirePane, ManagedPane bool
	Port                     Port
	Observe                  func(Event)
}

// MemberResult carries a dry plan or the next action after interruption.
type MemberResult struct {
	Plan *Plan
	Next string
}

func resolveMember(in MemberInput, dryRead bool) (*Resolved, error) {
	cat := in.Env.Catalog(source.Disk{})
	if dryRead {
		return resolveTeamDry(cat, in.Env.ProjectRoot, &options.Options{Team: in.Team})
	}
	read := readSession(in.Port)
	if read.Err != nil {
		return nil, read.Err
	}
	running := team.RunningTeams(read.Tabs, launchTeams(), read.AgentsRead)
	names := make([]domainname.TeamRef, len(running))
	for i, one := range running {
		names[i] = one.Team
	}
	var inferred domainname.Seen
	if in.Team.String() == "" && len(running) > 1 && CurrentPane() != "" {
		if agent, err := herdr.New(portOrDefault(in.Port)).Agent(CurrentPane()); err == nil {
			inferred = teamOfAgent(running, agent.Name)
		}
	}
	selected, err := in.Team.ResolveTeam(names, inferred)
	if err != nil {
		return nil, err
	}
	one, _ := team.FindRunning(running, selected.String())
	return buildRunning(cat, in.Env.ProjectRoot, one)
}

func memberPlan(in MemberInput, mode string, dryRead bool) (*Plan, error) {
	cat := in.Env.Catalog(source.Disk{})
	if err := sharedAgreementStops(cat); err != nil {
		return nil, err
	}
	r, err := resolveMember(in, dryRead)
	if err != nil {
		return nil, err
	}
	member := in.Member
	if in.All {
		member = ""
	}
	selected, err := r.SelectStarts(member)
	if err != nil {
		return nil, err
	}
	return &Plan{Resolved: r, Members: selected, Mode: mode}, nil
}

func requireMemberPane(in MemberInput) error {
	if in.RequirePane && (!in.ManagedPane || CurrentPane() == "") {
		return words.Errorf("runtime.managed-pane", i18n.Language())
	}
	return nil
}

// Refresh restarts existing members, preserving the no-new-pane rule.
func Refresh(ctx context.Context, in MemberInput) (MemberResult, error) {
	plan, err := memberPlan(in, "restart", false)
	out := MemberResult{Plan: plan}
	if err != nil {
		return out, err
	}
	r := plan.Resolved
	probe := operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe)
	newPanes, err := probe.RefreshDry(plan.Members, CurrentPane())
	if err != nil {
		return out, err
	}
	if len(newPanes.Members) > 0 {
		var names []string
		for _, name := range newPanes.Members {
			names = append(names, domainname.ShortIn(name.Observation(), r.Team.Name.Observation()).String())
		}
		return out, words.Errorf("refresh.no-pane", i18n.Language(), strings.Join(names, i18n.T("list.separator")))
	}
	member := in.Member
	if in.All {
		member = "--all"
	}
	plan.Next = "refresh " + member + " --team " + r.Team.Name.String()
	if in.Dry {
		return out, nil
	}
	if err := requireMemberPane(in); err != nil {
		return out, err
	}
	emit(in.Observe, Event{Plan: plan})
	l := operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe)
	err = execute(ctx, l, in.Observe, nil, func() error { return l.Refresh(plan.Members, CurrentPane()) })
	if ctx.Err() == nil && (err == nil || launcher.IsUndelivered(err)) {
		warning(in.Observe, recordReceivedLaunch(in.Env, r, launchlog.CommandRefresh, plan.Members, l, launchlog.Workspace{}))
	}
	return out, err
}

// Reprompt sends current bootstrap prompts without replacing conversations.
func Reprompt(ctx context.Context, in MemberInput) (MemberResult, error) {
	plan, err := memberPlan(in, "reprompt", in.Dry)
	out := MemberResult{Plan: plan}
	if err != nil {
		return out, err
	}
	r := plan.Resolved
	member := in.Member
	if in.All {
		member = "--all"
	}
	plan.Next = "reprompt " + member + " --team " + r.Team.Name.String()
	if in.Dry {
		return out, nil
	}
	if err := requireMemberPane(in); err != nil {
		return out, err
	}
	emit(in.Observe, Event{Plan: plan})
	l := operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe)
	sent := promptsToResend(in.Env.Catalog(source.Disk{}), in.Env.ProjectRoot, r, plan.Members)
	err = execute(ctx, l, in.Observe, nil, func() error { return l.Reprompt(sent) })
	if ctx.Err() == nil && (err == nil || launcher.IsUndelivered(err)) {
		warning(in.Observe, recordReceivedLaunch(in.Env, r, launchlog.CommandReprompt, plan.Members, l, launchlog.Workspace{}))
	}
	return out, err
}

// HireInput keeps unresolved hire words apart from the checked worker start.
type HireInput struct {
	MemberInput
	Name, Purpose, Task, From string
}

// Hire validates a permitted worker, starts it and records its hiring member.
func Hire(ctx context.Context, in HireInput) (MemberResult, error) {
	var out MemberResult
	if err := sharedAgreementStops(in.Env.Catalog(source.Disk{})); err != nil {
		return out, err
	}
	r, err := resolveMember(in.MemberInput, in.Dry)
	if err != nil {
		return out, err
	}
	worker, err := spawn.Plan(r.ModelTeamName(), r.Starts, r.Profiles, in.Name, in.Purpose, in.Task, in.From)
	if err != nil {
		return out, err
	}
	wireStart(worker, r.Team.Name.String())
	for _, note := range signalNotes([]definition.Start{*worker}) {
		emit(in.Observe, Event{Text: note})
	}
	out.Plan = &Plan{Resolved: r, Members: []definition.Start{*worker}, Mode: "spawn"}
	if in.Dry {
		return out, nil
	}
	if err := requireMemberPane(in.MemberInput); err != nil {
		return out, err
	}
	emit(in.Observe, Event{Plan: out.Plan})
	l := operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe)
	err = execute(ctx, l, in.Observe, nil, func() error { return l.Hire(worker, in.Task) })
	if ctx.Err() == nil && (err == nil || launcher.IsUndelivered(err)) {
		warning(in.Observe, recordHire(in.Env, r, *worker))
	}
	return out, err
}

// Nudge sends a short message to one stopped member, with its resolved sender.
func Nudge(ctx context.Context, in MemberInput, body string) error {
	r, err := resolveMember(in, in.Dry)
	if err != nil {
		return err
	}
	member, err := r.Member(in.Member)
	if err != nil {
		return err
	}
	if body == "" {
		body = i18n.T("nudge.default")
	}
	var from domainname.Seen
	if !in.Dry {
		from = senderName(r, CurrentPane(), in.Port)
	}
	text := sendText(from.String(), body)
	if in.Dry {
		emit(in.Observe, Event{Text: i18n.Tf("send.dry", member.Name.String(), text)})
		return nil
	}
	return operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe).Nudge(member, text)
}

// StopMember interrupts a turn while retaining its conversation and turn record.
func StopMember(ctx context.Context, in MemberInput) (MemberResult, error) {
	var out MemberResult
	r, err := resolveMember(in, in.Dry)
	if err != nil {
		return out, err
	}
	member, err := r.Member(in.Member)
	if err != nil {
		return out, err
	}
	if in.Dry {
		emit(in.Observe, Event{Text: i18n.Tf("stop.dry", member.Name.String())})
		return out, nil
	}
	stopped, err := operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe).Stop(member, CurrentPane())
	_, notStopped := errors.AsType[launcher.StopNotStopped](err)
	if stopped || notStopped {
		out.Next = "refresh " + member.Name.String() + " --team " + r.Team.Name.String()
	}
	return out, err
}

// FocusPolicy preserves the configured CLI selection and the live screen selection.
type FocusPolicy uint8

// Focus selection policies.
const (
	FocusConfigured FocusPolicy = iota
	FocusLive
)

// FocusInput identifies the focus request and its membership policy.
type FocusInput struct {
	MemberInput
	Policy   FocusPolicy
	TeamName string
	Views    []ProjectView
}

// Focus revalidates membership under the chosen entry-point policy.
func Focus(ctx context.Context, input FocusInput) error {
	in := input.MemberInput
	if input.Policy == FocusLive {
		return focusLive(input)
	}
	r, err := resolveMember(in, false)
	if err != nil {
		return err
	}
	member, err := r.Member(in.Member)
	if err != nil {
		return err
	}
	return operationLauncher(ctx, r, in.Timeout, in.Port, in.Observe).Focus(member)
}

// sendText formats the common sender prefix, including messages sent by tell.
func sendText(from, body string) string {
	if from == "" {
		from = i18n.T("send.who_human")
	}
	return i18n.Tf("send.from", from) + " " + body
}

// senderName reads the sender from a pane and matches it against resolved members.
func senderName(r *Resolved, pane domainname.PaneID, port Port) domainname.Seen {
	if pane == "" {
		return domainname.Seen{}
	}
	agent, err := herdr.New(portOrDefault(port)).Agent(pane)
	if err != nil {
		return domainname.Seen{}
	}
	for _, start := range r.Starts {
		if start.Name.String() == agent.Name.String() {
			return agent.Name
		}
	}
	return domainname.Seen{}
}

func focusLive(in FocusInput) error {
	read := readSession(in.Port)
	if read.Err == nil {
		for _, one := range readRunningTeamsIn(in.Views, read, false) {
			if one.Name.String() != in.TeamName {
				continue
			}
			for _, node := range judgeOf(one, read, time.Now()).Nodes {
				if node.Herdr.String() != in.Member || node.State.Slug() == "absent" {
					continue
				}
				if err := herdr.New(portOrDefault(in.Port)).Focus(node.Herdr); err != nil {
					return refuse("internal", errors.New(i18n.T("web.activity.focus_failed")))
				}
				return nil
			}
		}
	}
	return refuse("notfound", errors.New("対象のメンバーは起動していません。アクティビティを読み直してください"))
}
