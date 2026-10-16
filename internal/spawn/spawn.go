// Package spawn plans one worker a member hands a task to (hergy hire).
package spawn

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
)

// Plan returns the worker and notes on where the hire goes beyond the member's
// workers. Notes never stop the hire: the permission is only stated (段 6b makes
// it a refusal). team is the team name the hiring member runs under; starts are
// the team's members. name is the short worker name; the herdr name is
// "<team>-<name>", like the team's own members. profileName, when empty, is the
// one the member's workers pair with purpose.
func Plan(team string, starts []definition.Start, profiles map[string]profile.Profile, name, profileName, purpose, task, from string) (*definition.Start, []string, error) {
	if !profile.NamePattern.MatchString(name) {
		return nil, nil, i18n.Errorf("error.worker_name_pattern")
	}
	herdrName := team + "-" + name
	if !profile.NamePattern.MatchString(herdrName) {
		return nil, nil, i18n.Errorf("error.herdr_name_pattern", herdrName)
	}
	for i := range starts {
		if starts[i].Short == name || starts[i].Name == name {
			return nil, nil, i18n.Errorf("error.worker_exists", name)
		}
	}
	var lead *definition.Start
	if from != "" {
		found, err := lookup(starts, from)
		if err != nil {
			return nil, nil, err
		}
		lead = found
	} else {
		found, err := defaultLead(starts)
		if err != nil {
			return nil, nil, err
		}
		lead = found
	}

	// With no --profile the purpose names the pair: its profile is the one to
	// use. A purpose outside the member's workers then has no profile to fall
	// back to.
	chosen := profileName
	if chosen == "" {
		paired, ok := profileFor(lead.Workers, purpose)
		if !ok {
			return nil, nil, i18n.Errorf("error.purpose_needs_profile",
				purpose, lead.Name, availablePurposes(lead.Workers))
		}
		chosen = paired
	}
	profileValue, ok := profiles[chosen]
	if !ok {
		return nil, nil, i18n.Errorf("error.no_profile_named", chosen)
	}

	var parts = map[string]func() string{
		"team":      func() string { return team },
		"lead":      func() string { return lead.Name },
		"lead_role": func() string { return lead.Role },
		"purpose":   func() string { return purpose },
		"name":      func() string { return herdrName },
		"task":      func() string { return task },
		"herdr": func() string {
			return prompt.HerdrWorker(i18n.Tf("prompt.self_intro_worker", team, purpose, herdrName))
		},
	}
	text, err := prompt.Expand(prompt.WorkerTemplate(), parts)
	if err != nil {
		return nil, nil, err
	}

	worker := &definition.Start{
		Name:    herdrName,
		Short:   name,
		Role:    i18n.Tf("role.worker", purpose),
		Kind:    profileValue.Kind,
		Args:    profileValue.Args,
		Env:     profileValue.Env,
		Cwd:     lead.Cwd,
		Prompt:  text,
		Profile: profileValue.Name,
	}
	return worker, notes(lead, chosen, purpose), nil
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

// defaultLead is the only member that may hire; with none or several, --from is
// required.
func defaultLead(starts []definition.Start) (*definition.Start, error) {
	var spawners []string
	var only *definition.Start
	for i := range starts {
		if len(starts[i].Workers) > 0 {
			spawners = append(spawners, starts[i].Name)
			only = &starts[i]
		}
	}
	switch len(spawners) {
	case 1:
		return only, nil
	case 0:
		return nil, i18n.Errorf("error.from_required_none")
	default:
		return nil, i18n.Errorf("error.from_required_many", spawners)
	}
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

// notes says where the hire goes beyond the member's workers. Nothing here stops
// the hire: the permission is only stated.
func notes(lead *definition.Start, profileName, purpose string) []string {
	if len(lead.Workers) == 0 {
		return []string{fmt.Sprintf(i18n.T("error.no_may_spawn"), lead.Name)}
	}
	paired, ok := profileFor(lead.Workers, purpose)
	if !ok {
		return []string{fmt.Sprintf(i18n.T("error.purpose_outside"), purpose, lead.Name)}
	}
	if paired != profileName {
		return []string{fmt.Sprintf(i18n.T("error.profile_outside"), profileName, purpose, lead.Name)}
	}
	return nil
}

// availablePurposes is what a "this purpose has no profile" error can offer.
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
