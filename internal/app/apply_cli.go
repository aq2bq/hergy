package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// This file is hergy apply: the one operation that puts a saved definition's
// drift into the teams running from it (d-36ef, d-3d30). It reads the same plan
// the screens read, so the command line and the sheet cannot disagree about what
// differs or about the way hergy would put each member right.

// runApply is hergy apply [<member>...] [--team <team>] [--dry] [--json].
func runApply(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	cat := env.Catalog(source.Disk{})
	if err := sharedAgreementStops(cat); err != nil {
		return err
	}
	read := newActivityRead(env)
	plan := read.plan(read.clock())
	if !plan.apply.Herdr.OK {
		return errors.New(plan.apply.Herdr.Text)
	}
	selected, remaining, err := chooseApply(plan, opts)
	if err != nil {
		return err
	}
	if opts.Dry {
		return printApplyPlan(stdout, selected, remaining, opts.JSON)
	}
	if len(selected) == 0 {
		fmt.Fprintln(stdout, i18n.T("apply.nothing"))
		printRemaining(stdout, remaining)
		return applyLeftError(remaining)
	}
	if err := requireHerdrPane(); err != nil {
		return err
	}
	if err := applyNow(ctx, env, selected, opts.Timeout, stdout, stderr); err != nil {
		return err
	}
	printRemaining(stdout, remaining)
	return applyLeftError(remaining)
}

// plannedByName finds one member of the plan by the name herdr sees first, then
// by the short name team.yaml writes.
func plannedByName(plan deliveryPlan, name string) (plannedMember, bool) {
	if member, ok := plan.members[name]; ok {
		return member, true
	}
	for _, member := range plan.members {
		if member.start.Short == name {
			return member, true
		}
	}
	return plannedMember{}, false
}

// chosen is one member the command applies or leaves, with the sentence for
// leaving it.
type chosen struct {
	member  plannedMember
	whyLeft string
}

// chooseApply is the members the command line names, or every member with drift,
// split into the ones hergy can apply and the ones it leaves. A name that is not
// a member with drift is an error: the person typed something hergy does not see.
func chooseApply(plan deliveryPlan, opts *options.Options) ([]plannedMember, []chosen, error) {
	named := map[string]bool{}
	for _, name := range opts.Members {
		member, ok := plannedByName(plan, name)
		if !ok {
			return nil, nil, i18n.Errorf("apply.refused_unknown", name)
		}
		if !member.changed {
			return nil, nil, i18n.Errorf("apply.no_change", name)
		}
		named[member.start.Name] = true
		named[member.start.Short] = true
	}
	var selected []plannedMember
	var remaining []chosen
	for _, row := range plan.apply.Teams {
		if opts.Team != "" && row.Team != opts.Team {
			continue
		}
		for _, shown := range row.Members {
			if len(named) > 0 && !named[shown.Name] && !named[shown.Short] {
				continue
			}
			member, ok := plan.members[shown.Name]
			if !ok {
				continue
			}
			if member.canApply() {
				selected = append(selected, member)
				continue
			}
			why := member.whyNot
			if why == "" {
				why = i18n.Tf("apply.refused_state", member.start.Short)
			}
			remaining = append(remaining, chosen{member: member, whyLeft: why})
		}
	}
	return selected, remaining, nil
}

// applyNow applies the selected members, one team at a time, with the launcher
// the other commands use, and records what was handed to whom.
func applyNow(ctx context.Context, env team.Env, selected []plannedMember, timeout int, stdout, stderr io.Writer) error {
	for _, group := range groupByTeam(selected) {
		projectEnv := env
		if scoped, err := env.AtProject(group[0].team.Project); err == nil {
			projectEnv = scoped
		}
		cat := projectEnv.Catalog(source.Disk{})
		r, err := buildRunningIn(cat, projectEnv.ProjectRoot,
			team.Running{Team: group[0].team.Name, Definition: group[0].team.Definition}, group[0].team.Place)
		if err != nil {
			return err
		}
		l := newLauncher(r, timeout, stdout).WithContext(ctx)
		var starts, resends []definition.Start
		for _, member := range group {
			if member.how == web.ApplyResend {
				resends = append(resends, member.start)
				continue
			}
			starts = append(starts, member.start)
		}
		if len(starts) > 0 {
			if err := l.Refresh(starts, os.Getenv("HERDR_PANE_ID")); err != nil {
				return err
			}
		}
		if len(resends) > 0 {
			if err := l.Reprompt(promptsToResend(cat, projectEnv.ProjectRoot, r, resends)); err != nil {
				return err
			}
		}
		all := make([]definition.Start, 0, len(group))
		for _, member := range group {
			all = append(all, member.start)
			fmt.Fprintf(stdout, "%s\n", i18n.Tf("apply.applied", member.start.Short, applyHowWord(member.how)))
		}
		recordLaunch(stderr, projectEnv, r, launchlog.CommandApply, all)
	}
	return nil
}

// applyHowWord is one way of applying, as a person reads it.
func applyHowWord(how string) string {
	switch how {
	case web.ApplyStart:
		return i18n.T("apply.how_start")
	case web.ApplyRestart:
		return i18n.T("apply.how_restart")
	case web.ApplyCannot:
		return i18n.T("apply.how_cannot")
	default:
		return i18n.T("apply.how_resend")
	}
}

// printApplyPlan is --dry: what would be applied, how, and what it would change.
func printApplyPlan(stdout io.Writer, selected []plannedMember, remaining []chosen, asJSON bool) error {
	if asJSON {
		list := web.ApplyList{Herdr: web.ActivityNote{OK: true}, Teams: []web.ApplyTeam{}}
		byTeam := map[string]int{}
		add := func(member plannedMember) {
			index, ok := byTeam[member.team.Name]
			if !ok {
				list.Teams = append(list.Teams, web.ApplyTeam{
					Team: member.team.Name, Definition: member.team.Definition,
					Project: member.team.Project, ProjectName: member.team.ProjectName,
					Notes: []string{}, Members: []web.ApplyMember{},
				})
				index = len(list.Teams) - 1
				byTeam[member.team.Name] = index
			}
			shown := web.ApplyMember{
				Name: member.start.Name, Short: member.start.Short, Role: member.start.Role,
				State: member.state, How: member.how, Changes: member.changes,
				CanApply: member.canApply(), WhyNot: member.whyNot,
				ConversationLost: member.how == web.ApplyRestart,
			}
			list.Teams[index].Members = append(list.Teams[index].Members, shown)
		}
		for _, member := range selected {
			add(member)
		}
		for _, one := range remaining {
			add(one.member)
		}
		for i := range list.Teams {
			list.Count += len(list.Teams[i].Members)
		}
		out, err := json.Marshal(list)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, string(out))
		return nil
	}
	fmt.Fprintln(stdout, i18n.T("apply.plan"))
	for _, group := range groupByTeam(selected) {
		fmt.Fprintf(stdout, "%s\n", group[0].team.Name)
		for _, member := range group {
			fmt.Fprintf(stdout, "  %s: %s\n", member.start.Short, applyHowWord(member.how))
			for _, change := range member.changes {
				fmt.Fprintf(stdout, "    - %s\n", change.Text)
			}
		}
	}
	printRemaining(stdout, remaining)
	return nil
}

// printRemaining is the members that stay, with the sentence saying why.
func printRemaining(stdout io.Writer, remaining []chosen) {
	for _, one := range remaining {
		fmt.Fprintf(stdout, "%s\n", i18n.Tf("apply.left", one.member.start.Short, one.whyLeft))
	}
}

// applyLeftError is the exit status: a member left is not a success, so hergy
// says so and answers 1.
func applyLeftError(remaining []chosen) error {
	if len(remaining) == 0 {
		return nil
	}
	return i18n.Errorf("apply.left_count", len(remaining))
}
