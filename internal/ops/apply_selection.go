package ops

import (
	"github.com/aq2bq/hergy/internal/i18n"
)

// ApplyPolicy preserves how each entry point handles an unavailable selection.
type ApplyPolicy uint8

// Apply selection policies.
const (
	ContinueAvailable ApplyPolicy = iota
	RequireWholeSelection
)

// ApplySelectionInput names the members, team filter and selection policy.
type ApplySelectionInput struct {
	Names  []string
	Team   string
	Policy ApplyPolicy
}

// ApplyLeft is a changed member that the partial-selection policy leaves alone.
type ApplyLeft struct {
	Member  PlannedMember
	WhyLeft string
}

// ApplySelection is the validated input for an apply and its unapplied remainder.
type ApplySelection struct {
	Selected  []PlannedMember
	Remaining []ApplyLeft
	Broken    []ApplyTeam
	Refusals  []string
}

// SelectApply resolves names and removes duplicate recipients before execution.
func SelectApply(plan DeliveryPlan, in ApplySelectionInput) (ApplySelection, error) {
	if in.Policy == RequireWholeSelection {
		return selectWhole(plan, in), nil
	}
	var out ApplySelection
	named := map[string]bool{}
	for _, name := range in.Names {
		member, ok := plannedByName(plan, name)
		if !ok {
			return out, i18n.Errorf("apply.refused_unknown", name)
		}
		if !member.Changed {
			return out, i18n.Errorf("apply.no_change", name)
		}
		named[member.Start.Name.String()] = true
		named[member.Start.Short.String()] = true
	}
	for _, row := range plan.Apply.Teams {
		if row.Error != "" {
			out.Broken = append(out.Broken, row)
		}
		if in.Team != "" && row.Team != in.Team {
			continue
		}
		for _, shown := range row.Members {
			if len(named) > 0 && !named[shown.Name] && !named[shown.Short] {
				continue
			}
			member, ok := plan.Members[shown.Name]
			if !ok {
				continue
			}
			if member.CanApply() {
				out.Selected = append(out.Selected, member)
				continue
			}
			why := member.WhyNot
			if why == "" {
				why = i18n.Tf("apply.refused_state", member.Start.Short.String())
			}
			out.Remaining = append(out.Remaining, ApplyLeft{Member: member, WhyLeft: why})
		}
	}
	return out, nil
}
func selectWhole(plan DeliveryPlan, in ApplySelectionInput) ApplySelection {
	var out ApplySelection
	if len(in.Names) == 0 {
		out.Refusals = []string{i18n.T("apply.none_selected")}
		return out
	}
	seen := map[string]bool{}
	for _, name := range in.Names {
		if seen[name] {
			continue
		}
		seen[name] = true
		member, ok := plan.Members[name]
		switch {
		case !ok:
			out.Refusals = append(out.Refusals, i18n.Tf("apply.refused_unknown", name))
		case !member.Changed:
			out.Refusals = append(out.Refusals, i18n.Tf("apply.no_change", name))
		case !member.CanApply():
			out.Refusals = append(out.Refusals, member.WhyNot)
		default:
			out.Selected = append(out.Selected, member)
		}
	}
	return out
}
func plannedByName(plan DeliveryPlan, name string) (PlannedMember, bool) {
	if member, ok := plan.Members[name]; ok {
		return member, true
	}
	for _, member := range plan.Members {
		if member.Start.Short.String() == name {
			return member, true
		}
	}
	return PlannedMember{}, false
}

// GroupByTeam preserves the selected order within each team's single execution.
func GroupByTeam(members []PlannedMember) [][]PlannedMember {
	var out [][]PlannedMember
	at := map[string]int{}
	for _, member := range members {
		index, ok := at[member.Team.Name.String()]
		if !ok {
			at[member.Team.Name.String()] = len(out)
			out = append(out, []PlannedMember{member})
			continue
		}
		out[index] = append(out[index], member)
	}
	return out
}

// Teams is the complete set reserved and cancelled by this selection.
func (s ApplySelection) Teams() []string {
	var out []string
	for _, group := range GroupByTeam(s.Selected) {
		out = append(out, group[0].Team.Name.String())
	}
	return out
}
