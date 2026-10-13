// Package spawn plans one worker a member hands a task to (hergy hire).
package spawn

import (
	"fmt"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/prompt"
)

// Plan returns the worker and notes on where the hire goes beyond the lead's
// may_spawn. Notes never stop the hire: the permission is only stated. name is
// the short worker name; the herdr name is "<team>-<name>", like the team's own
// members.
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
	chosen, ok := cfg.Profiles[profile]
	if !ok {
		return nil, nil, i18n.Errorf("error.no_profile_named", profile)
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
		Kind:      chosen.Kind,
		Args:      chosen.Args,
		Env:       chosen.Env,
		Cwd:       lead.Cwd,
		Prompt:    text,
		Profile:   chosen.Name,
	}
	return worker, notes(lead, chosen.Name, purpose), nil
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

func notes(lead *config.Member, profile, purpose string) []string {
	if lead.MaySpawn == nil {
		return []string{fmt.Sprintf(i18n.T("error.no_may_spawn"), lead.Name)}
	}
	var out []string
	if !contains(lead.MaySpawn.Purposes, purpose) {
		out = append(out, fmt.Sprintf(i18n.T("error.purpose_outside"), purpose, lead.Name))
	}
	if !contains(lead.MaySpawn.Profiles, profile) {
		out = append(out, fmt.Sprintf(i18n.T("error.profile_outside"), profile, lead.Name))
	}
	return out
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
