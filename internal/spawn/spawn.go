// Package spawn plans one worker a member hands a task to (hergy hire).
package spawn

import (
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
)

// Plan returns the worker for one hire, or stops it. A member may hire only for
// a purpose its workers: names (I6): a purpose outside that entry stops the hire
// and names the purposes the member may hire for. The profile comes from that
// entry, so a hire does not choose one (I12c). team is the team name the hiring
// member runs under; starts are the team's members. name is the short worker
// name; the herdr name is "<team>-<name>", like the team's own members. from
// names the hiring member; empty means the one member whose workers: name this
// purpose (d-6f03).
func Plan(team string, starts []definition.Start, profiles map[string]profile.Profile, name, purpose, task, from string) (*definition.Start, error) {
	if !profile.NamePattern.MatchString(name) {
		return nil, i18n.Errorf("error.worker_name_pattern")
	}
	herdrName := team + "-" + name
	if !profile.NamePattern.MatchString(herdrName) {
		return nil, i18n.Errorf("error.herdr_name_pattern", herdrName)
	}
	for i := range starts {
		if starts[i].Short == name || starts[i].Name == name {
			return nil, i18n.Errorf("error.worker_exists", name)
		}
	}
	lead, err := hiringMember(starts, purpose, from)
	if err != nil {
		return nil, err
	}

	// The purpose must be one the member's workers: names, and its profile is
	// the one that entry pairs it with.
	chosen, ok := profileFor(lead.Workers, purpose)
	if !ok {
		return nil, i18n.Errorf("error.hire_purpose_outside", lead.Short, purpose, availablePurposes(lead.Workers))
	}
	profileValue, ok := profiles[chosen]
	if !ok {
		return nil, i18n.Errorf("error.no_profile_named", chosen)
	}

	var parts = map[string]func() string{
		"team":      func() string { return team },
		"lead":      func() string { return lead.Name },
		"lead_role": func() string { return lead.Role },
		"purpose":   func() string { return purpose },
		"name":      func() string { return herdrName },
		"actor":     func() string { return i18n.Tf("prompt.actor", herdrName) },
		"task":      func() string { return task },
		"herdr": func() string {
			return prompt.HerdrWorker(i18n.Tf("prompt.self_intro_worker", team, purpose, herdrName))
		},
	}
	text, err := prompt.Expand(prompt.WorkerTemplate(), parts)
	if err != nil {
		return nil, err
	}

	worker := &definition.Start{
		Name:    herdrName,
		Short:   name,
		Role:    i18n.Tf("role.worker", purpose),
		Kind:    profileValue.Kind,
		Args:    profileValue.Args,
		Env:     profile.PaneEnv(profileValue.Env, herdrName),
		Cwd:     lead.Cwd,
		Prompt:  text,
		Profile: profileValue.Name,
	}
	return worker, nil
}

// hiringMember is who hires: the member --from names, or the one member whose
// workers: name this purpose (d-6f03).
func hiringMember(starts []definition.Start, purpose, from string) (*definition.Start, error) {
	if from != "" {
		return lookup(starts, from)
	}
	return defaultLead(starts, purpose)
}

// lookup finds one member as a start: by its herdr name first, then by its short
// name.
func lookup(starts []definition.Start, name string) (*definition.Start, error) {
	for i := range starts {
		if starts[i].Name == name || starts[i].Short == name {
			return &starts[i], nil
		}
	}
	return nil, i18n.Errorf("error.no_member", name)
}

// defaultLead is the member that hires when --from is not written: the one whose
// workers: name this purpose (d-6f03). One is the hiring member; two or more stop
// the hire, naming them so a person says which. When no member hires for this
// purpose, --from would not help (every member would refuse it), so the hire is
// refused with who may hire for what.
func defaultLead(starts []definition.Start, purpose string) (*definition.Start, error) {
	candidates := hirersFor(starts, purpose)
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return nil, i18n.Errorf("error.hire_purpose_nobody", purpose, hirings(starts))
	default:
		names := make([]string, len(candidates))
		for i, candidate := range candidates {
			names[i] = candidate.Short
		}
		return nil, i18n.Errorf("error.from_required_many", strings.Join(names, i18n.T("list.separator")))
	}
}

// hirings is who may hire for what, as a person reads it ("lead は 実装／spec は
// 調査"): every member whose workers: is not empty, with the purposes it names,
// in the written order. Members and purposes use different separators
// (spec/34 H2), so a member is not read as another purpose. No member hiring at
// all is "none".
func hirings(starts []definition.Start) string {
	var parts []string
	for i := range starts {
		if len(starts[i].Workers) == 0 {
			continue
		}
		parts = append(parts, i18n.Tf("hire.member_purposes", starts[i].Short, availablePurposes(starts[i].Workers)))
	}
	if len(parts) == 0 {
		return i18n.T("value.none")
	}
	return strings.Join(parts, i18n.T("hire.member_separator"))
}

// hirersFor is the members that may hire for one purpose: the members whose
// workers: name it.
func hirersFor(starts []definition.Start, purpose string) []*definition.Start {
	var out []*definition.Start
	for i := range starts {
		if _, ok := profileFor(starts[i].Workers, purpose); ok {
			out = append(out, &starts[i])
		}
	}
	return out
}

// profileFor is the profile a member's workers pair with a purpose.
func profileFor(workers []definition.WorkerNode, purpose string) (string, bool) {
	for _, worker := range workers {
		if worker.Purpose == purpose {
			return worker.Profile, true
		}
	}
	return "", false
}

// availablePurposes is the purposes a member may hire for, as one list: what a
// refused hire names, so a person reads what the member's workers: allows.
func availablePurposes(workers []definition.WorkerNode) string {
	if len(workers) == 0 {
		return i18n.T("value.none")
	}
	purposes := make([]string, len(workers))
	for i, worker := range workers {
		purposes[i] = worker.Purpose
	}
	return strings.Join(purposes, i18n.T("list.separator"))
}
