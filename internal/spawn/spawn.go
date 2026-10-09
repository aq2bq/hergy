// Package spawn plans one worker a member hands a task to (hergy hire).
package spawn

import (
	"fmt"

	"github.com/aq2bq/hergy/internal/config"
	"github.com/aq2bq/hergy/internal/prompt"
)

// Plan returns the worker and notes on where the hire goes beyond the lead's
// may_spawn. Notes never stop the hire: the permission is only stated.
func Plan(cfg *config.Config, name, profile, purpose, task, from string) (*config.Member, []string, error) {
	if !config.NamePattern.MatchString(name) {
		return nil, nil, fmt.Errorf("worker name must match [a-z][a-z0-9_-]{0,31}")
	}
	for i := range cfg.Members {
		if cfg.Members[i].Name == name {
			return nil, nil, fmt.Errorf("%s is already a team member", name)
		}
	}
	chosen, ok := cfg.Profiles[profile]
	if !ok {
		return nil, nil, fmt.Errorf("no profile named %s", profile)
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
		"name":      func() string { return name },
		"task":      func() string { return task },
		"herdr": func() string {
			return prompt.Herdr(fmt.Sprintf("%sの%sワーカーの%sです。", cfg.Team, purpose, name))
		},
	}
	text, err := prompt.Expand(prompt.WorkerTemplate, parts)
	if err != nil {
		return nil, nil, err
	}

	worker := &config.Member{
		Name:    name,
		Role:    purpose + "ワーカー",
		Kind:    chosen.Kind,
		Args:    chosen.Args,
		Env:     chosen.Env,
		Cwd:     lead.Cwd,
		Prompt:  text,
		Profile: chosen.Name,
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
		return nil, fmt.Errorf("--from is required (no member has may_spawn)")
	default:
		return nil, fmt.Errorf("--from is required (members with may_spawn: %v)", spawners)
	}
}

func notes(lead *config.Member, profile, purpose string) []string {
	if lead.MaySpawn == nil {
		return []string{fmt.Sprintf("%s has no may_spawn in the config", lead.Name)}
	}
	var out []string
	if !contains(lead.MaySpawn.Purposes, purpose) {
		out = append(out, fmt.Sprintf("purpose %s is not in %s's may_spawn.purposes", purpose, lead.Name))
	}
	if !contains(lead.MaySpawn.Profiles, profile) {
		out = append(out, fmt.Sprintf("profile %s is not in %s's may_spawn.profiles", profile, lead.Name))
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
