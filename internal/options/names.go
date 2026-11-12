package options

import (
	"errors"
	"strings"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
)

// Input is an unresolved command word. Parsing does not assign it model meaning.
type Input struct{ raw string }

func (w Input) String() string { return w.raw }

// DefinitionName resolves a definition command's word without narrowing the
// names of existing directories. New names are checked separately by new.
func (w Input) DefinitionName() domainname.DefinitionName {
	n, _ := domainname.ParseDefinitionName(w.raw)
	return n
}

// ResolveTeam selects an observed running team. A pane may supply an inferred
// team only when the command omitted its own word.
func (w Input) ResolveTeam(teams []domainname.TeamRef, inferred domainname.Seen) (domainname.TeamRef, error) {
	raw := w.raw
	if raw == "" {
		raw = inferred.String()
	}
	if raw != "" {
		for _, team := range teams {
			if team.String() == raw {
				return team, nil
			}
		}
		return domainname.TeamRef{}, errors.New(i18n.Tf("error.team_not_running", raw) + "\n" + i18n.Tf("next", "status"))
	}
	switch len(teams) {
	case 1:
		return teams[0], nil
	case 0:
		return domainname.TeamRef{}, i18n.Errorf("error.no_running_teams")
	default:
		names := make([]string, len(teams))
		for i, team := range teams {
			names[i] = team.String()
		}
		return domainname.TeamRef{}, errors.New(i18n.Tf("error.need_running_team", strings.Join(names, i18n.T("list.separator"))) + "\n" + i18n.Tf("next", "status"))
	}
}

// Worker is a worker read from a running team's worker tab, not a new model name.
type Worker struct{ Team, Short, Herdr, Label domainname.Seen }

// DownChoice is the closed team/worker sum returned after resolving down's word.
type DownChoice interface{ downChoice() }

// DownTeam selects a whole observed team.
type DownTeam struct{ Team domainname.Seen }

func (DownTeam) downChoice() {}

// DownWorker selects exactly one observed worker.
type DownWorker struct{ Worker Worker }

func (DownWorker) downChoice() {}

// ResolveDown resolves down's one positional word. Team and worker candidates
// remain observations, so legacy names can still be selected and closed.
func (w Input) ResolveDown(teams []domainname.Seen, workers []Worker) (DownChoice, error) {
	raw := w.raw
	var chosen domainname.Seen
	if raw == "" {
		switch len(teams) {
		case 1:
			return DownTeam{Team: teams[0]}, nil
		case 0:
			return nil, i18n.Errorf("error.no_running_teams")
		default:
			names := make([]string, len(teams))
			for i, t := range teams {
				names[i] = t.String()
			}
			return nil, errors.New(i18n.Tf("error.need_running_team", strings.Join(names, i18n.T("list.separator"))) + "\n" + i18n.Tf("next", "status"))
		}
	}
	for _, team := range teams {
		if team.String() == raw {
			chosen = team
			break
		}
	}
	var matched []Worker
	for _, worker := range workers {
		if worker.Short.String() == raw || worker.Herdr.String() == raw {
			matched = append(matched, worker)
		}
	}
	switch {
	case chosen.String() != "" && len(matched) > 0:
		return nil, i18n.Errorf("error.down_both", raw, chosen, matched[0].Herdr)
	case chosen.String() != "":
		return DownTeam{Team: chosen}, nil
	case len(matched) == 1:
		return DownWorker{Worker: matched[0]}, nil
	case len(matched) > 1:
		names := make([]string, len(matched))
		for i, worker := range matched {
			names[i] = worker.Herdr.String()
		}
		return nil, i18n.Errorf("error.worker_ambiguous", raw, strings.Join(names, i18n.T("list.separator")))
	default:
		return nil, errors.New(i18n.Tf("error.down_not_found", raw) + "\n" + i18n.Tf("next", "status"))
	}
}
