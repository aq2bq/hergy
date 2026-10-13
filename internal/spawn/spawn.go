// Package spawn plans one worker a member hands a task to (hergy hire).
package spawn

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
)

// Plan returns the worker and notes on where the hire goes beyond the lead's
// may_spawn. Notes never stop the hire: the permission is only stated. name is
// the short worker name; the herdr name is "<team>-<name>", like the team's own
// members. profile, when empty, is the one may_spawn pairs with purpose.
func Plan(cfg *config.Config, name, profile, purpose, task, from string) (*config.Member, []string, error) {
	if !config.NamePattern.MatchString(name) {
		return nil, nil, i18n.Errorf("error.worker_name_pattern")
	}
	herdrName := cfg.Team + "-" + name
	if !config.NamePattern.MatchString(herdrName) {
		return nil, nil, i18n.Errorf("error.herdr_name_pattern", herdrName)
	}
	for i := range cfg.Members {
		if cfg.Members[i].ShortName == name || cfg.Members[i].Name == name {
			return nil, nil, i18n.Errorf("error.worker_exists", name)
		}
	}
	var lead *config.Member
	if from != "" {
		found, err := cfg.Member(from)
		if err != nil {
			return nil, nil, err
		}
		lead = found
	} else {
		found, err := defaultLead(cfg)
		if err != nil {
			return nil, nil, err
		}
		lead = found
	}

	// With no --profile the purpose names the pair: its profile is the one to
	// use. A purpose outside may_spawn then has no profile to fall back to.
	chosen := profile
	if chosen == "" {
		paired, ok := lead.MaySpawn.ProfileFor(purpose)
		if !ok {
			return nil, nil, i18n.Errorf("error.purpose_needs_profile",
				purpose, lead.Name, availablePurposes(lead.MaySpawn))
		}
		chosen = paired
	}
	profileValue, ok := cfg.Profiles[chosen]
	if !ok {
		return nil, nil, i18n.Errorf("error.no_profile_named", chosen)
	}

	var parts = map[string]func() string{
		"team":      func() string { return cfg.Team },
		"lead":      func() string { return lead.Name },
		"lead_role": func() string { return lead.Role },
		"purpose":   func() string { return purpose },
		"name":      func() string { return herdrName },
		"task":      func() string { return task },
		"herdr": func() string {
			return prompt.HerdrWorker(i18n.Tf("prompt.self_intro_worker", cfg.Team, purpose, herdrName))
		},
	}
	text, err := prompt.Expand(prompt.WorkerTemplate(), parts)
	if err != nil {
		return nil, nil, err
	}

	worker := &config.Member{
		Name:      herdrName,
		ShortName: name,
		Role:      i18n.Tf("role.worker", purpose),
		Kind:      profileValue.Kind,
		Args:      profileValue.Args,
		Env:       profileValue.Env,
		Cwd:       lead.Cwd,
		Prompt:    text,
		Profile:   profileValue.Name,
	}
	return worker, notes(lead, chosen, purpose), nil
}

// defaultLead is the only member that may spawn; with none or several, --from is required.
func defaultLead(cfg *config.Config) (*config.Member, error) {
	var spawners []string
	var only *config.Member
	for i := range cfg.Members {
		if cfg.Members[i].MaySpawn != nil {
			spawners = append(spawners, cfg.Members[i].Name)
			only = &cfg.Members[i]
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

// notes says where the hire goes beyond the lead's may_spawn. Nothing here stops
// the hire: the permission is only stated.
func notes(lead *config.Member, profile, purpose string) []string {
	if lead.MaySpawn == nil {
		return []string{fmt.Sprintf(i18n.T("error.no_may_spawn"), lead.Name)}
	}
	paired, ok := lead.MaySpawn.ProfileFor(purpose)
	if !ok {
		return []string{fmt.Sprintf(i18n.T("error.purpose_outside"), purpose, lead.Name)}
	}
	if paired != profile {
		return []string{fmt.Sprintf(i18n.T("error.profile_outside"), profile, purpose, lead.Name)}
	}
	return nil
}

// availablePurposes is what a "this purpose has no profile" error can offer.
func availablePurposes(m *config.MaySpawn) string {
	purposes := m.Purposes()
	if len(purposes) == 0 {
		return i18n.T("value.none")
	}
	return strings.Join(purposes, i18n.T("list.separator"))
}
