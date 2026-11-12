package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/team"
)

// This file is hergy apply: the one operation that puts a saved definition's
// drift into the teams running from it (d-36ef, d-3d30). It reads the same plan
// the screens read, so the command line and the sheet cannot disagree about what
// differs or about the way hergy would put each member right.

// runApply is hergy apply [<member>...] [--team <team>] [--dry] [--json].
func runApply(ctx context.Context, env team.Env, opts *options.Options, stdout, stderr io.Writer) error {
	printed := false
	result, err := ops.Apply(ctx, ops.ApplyInput{Env: env, Selection: ops.ApplySelectionInput{Names: opts.Members, Team: opts.Team.String(), Policy: ops.ContinueAvailable},
		Timeout: opts.Timeout, Dry: opts.Dry, RequirePane: true, ManagedPane: os.Getenv("HERDR_ENV") == "1", Observe: cliEvents(stdout, stderr, env),
		Accepted: func(selection ops.ApplySelection) { printBroken(stdout, selection.Broken); printed = true }})
	if !printed {
		printBroken(stdout, result.Selection.Broken)
	}
	if err != nil {
		return operationError(err)
	}
	selection := result.Selection
	if opts.Dry {
		if err := printApplyPlan(stdout, result.Plan, selection.Selected, selection.Remaining, opts.JSON); err != nil {
			return err
		}
		return result.FinalError
	}
	if len(selection.Selected) == 0 {
		fmt.Fprintln(stdout, i18n.T("apply.nothing"))
	}
	printRemaining(stdout, selection.Remaining)
	return result.FinalError
}

// printBroken says why a team's definition cannot be built, and what to do: the
// reason hergy status writes, then hergy validate.
func printBroken(stdout io.Writer, broken []ops.ApplyTeam) {
	for _, team := range broken {
		fmt.Fprintln(stdout, i18n.Tf("apply.broken_team", team.Team, team.Error))
	}
	if len(broken) > 0 {
		fmt.Fprintln(stdout, i18n.T("apply.broken_fix"))
	}
}

type chosen = ops.ApplyLeft

func groupByTeam(members []plannedMember) [][]plannedMember { return ops.GroupByTeam(members) }

// applyHowWord is one way of applying, as a person reads it after it was done.
func applyHowWord(how ops.ApplyHow) string {
	switch how {
	case ops.ApplyStart:
		return i18n.T("apply.how_start")
	case ops.ApplyRestart:
		return i18n.T("apply.how_restart")
	case ops.ApplyCannot:
		return i18n.T("apply.how_cannot")
	default:
		return i18n.T("apply.how_resend")
	}
}

// applyPlanWord is one way of applying, as the plan says it: the words the
// sheet's mark uses (送り直し・起動・起動し直し), because nothing is done yet
// (d-3d30, d-7b45).
func applyPlanWord(how ops.ApplyHow) string {
	switch how {
	case ops.ApplyStart:
		return i18n.T("apply.plan_start")
	case ops.ApplyRestart:
		return i18n.T("apply.plan_restart")
	case ops.ApplyCannot:
		return i18n.T("apply.plan_cannot")
	default:
		return i18n.T("apply.plan_resend")
	}
}

// teamNotes is each team's own notes, by team name: a worker's changed profile
// is the team's note, not a member's change (d-36ef), and the command line says
// it where the sheet does (r-7aad).
func teamNotes(teams []ops.ApplyTeam) map[string][]string {
	out := map[string][]string{}
	for _, team := range teams {
		if len(team.Notes) > 0 {
			out[team.Team] = team.Notes
		}
	}
	return out
}

// printApplyPlan is --dry: what would be applied, how, and what it would change.
// The plan carries the teams' own notes the way GET /api/apply does, so the
// command line and the sheet say the same thing.
func printApplyPlan(stdout io.Writer, plan deliveryPlan, selected []plannedMember, remaining []chosen, asJSON bool) error {
	notes := teamNotes(plan.Apply.Teams)
	if asJSON {
		list := jsonApplyList{Herdr: jsonReadNote{OK: true}, Teams: []jsonApplyTeam{}}
		byTeam := map[string]int{}
		add := func(member plannedMember) {
			index, ok := byTeam[member.Team.Name.String()]
			if !ok {
				own := notes[member.Team.Name.String()]
				if own == nil {
					own = []string{}
				}
				list.Teams = append(list.Teams, jsonApplyTeam{
					Team: member.Team.Name.String(), Definition: member.Team.Definition,
					Project: member.Team.Project, ProjectName: member.Team.ProjectName,
					Notes: own, Members: []jsonApplyMember{},
				})
				index = len(list.Teams) - 1
				byTeam[member.Team.Name.String()] = index
			}
			shown := jsonApplyMember{
				Name: member.Start.Name.String(), Short: member.Start.Short.String(), Role: member.Start.Role,
				State: member.State, How: string(member.How), Changes: member.Changes,
				CanApply: member.CanApply(), WhyNot: member.WhyNot,
				ConversationLost: member.How == ops.ApplyRestart,
			}
			list.Teams[index].Members = append(list.Teams[index].Members, shown)
		}
		for _, member := range selected {
			add(member)
		}
		for _, one := range remaining {
			add(one.Member)
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
	if len(selected) == 0 && len(remaining) == 0 {
		fmt.Fprintln(stdout, i18n.T("apply.nothing"))
		return nil
	}
	fmt.Fprintln(stdout, i18n.T("apply.plan"))
	applied := map[string]bool{}
	for _, group := range groupByTeam(selected) {
		applied[group[0].Team.Name.String()] = true
		fmt.Fprintf(stdout, "%s\n", group[0].Team.Name)
		for _, note := range notes[group[0].Team.Name.String()] {
			fmt.Fprintf(stdout, "  %s\n", note)
		}
		for _, member := range group {
			fmt.Fprintf(stdout, "  %s: %s\n", member.Start.Short.String(), applyPlanWord(member.How))
			for _, change := range member.Changes {
				fmt.Fprintf(stdout, "    - %s\n", change.Text)
			}
		}
	}
	// A team whose members are all left still shows the team's own notes: the
	// sheet shows them beside the members it cannot apply too.
	left := map[string]bool{}
	for _, one := range remaining {
		left[one.Member.Team.Name.String()] = true
	}
	for _, team := range plan.Apply.Teams {
		if applied[team.Team] || !left[team.Team] {
			continue
		}
		fmt.Fprintf(stdout, "%s\n", team.Team)
		for _, note := range team.Notes {
			fmt.Fprintf(stdout, "  %s\n", note)
		}
	}
	printRemaining(stdout, remaining)
	return nil
}

// printRemaining is the members that stay, with the sentence saying why.
func printRemaining(stdout io.Writer, remaining []chosen) {
	for _, one := range remaining {
		fmt.Fprintf(stdout, "%s\n", i18n.Tf("apply.left", one.Member.Start.Short.String(), one.WhyLeft))
	}
}
