package ops

import (
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/options"
)

// downWorker is one worker hergy down closes on its own: the team it belongs to,
// the short name on its tab ("worker:<team>/<short>"), its herdr name, and the
// tab's label.
type downWorker struct {
	Team  domainname.Seen
	Short domainname.Seen
	Herdr domainname.Seen
	Label domainname.Seen
}

// runningWorkersOf is every worker of the running teams, read from the tabs hergy
// made for them ("worker:<team>/<worker>"). A tab whose team is not running is
// not one: only a running team's workers can be closed by name (d-fa05).
func runningWorkersOf(tabs []team.Tab, running []team.Running) []downWorker {
	live := make(map[string]bool, len(running))
	for _, one := range running {
		live[one.Team.String()] = true
	}
	var out []downWorker
	for _, tab := range tabs {
		worker, ok := tab.Label.WorkerTab()
		if !ok || !live[worker.Team().String()] {
			continue
		}
		out = append(out, downWorker{
			Team: worker.Team(), Short: worker.Short(), Herdr: worker.Agent(), Label: tab.Label,
		})
	}
	return out
}

// pickDown is hergy down's argument: a running team first, then, when no team
// carries the name, one worker of a running team, by its short name or its herdr
// name. A name that is both is refused, naming both, and so is a short name two
// teams' workers carry (d-fa05).
func pickDown(running []team.Running, tabs []team.Tab, input options.Input) (team.Running, *downWorker, error) {
	teams := make([]domainname.Seen, len(running))
	for i, one := range running {
		teams[i] = one.Team.Observation()
	}
	var workers []options.Worker
	for _, worker := range runningWorkersOf(tabs, running) {
		workers = append(workers, options.Worker{Team: worker.Team, Short: worker.Short, Herdr: worker.Herdr, Label: worker.Label})
	}
	choice, err := input.ResolveDown(teams, workers)
	if err != nil {
		return team.Running{}, nil, err
	}
	switch chosen := choice.(type) {
	case options.DownTeam:
		one, _ := team.FindRunning(running, chosen.Team.String())
		return one, nil, nil
	case options.DownWorker:
		worker := chosen.Worker
		return team.Running{}, &downWorker{Team: worker.Team, Short: worker.Short, Herdr: worker.Herdr, Label: worker.Label}, nil
	default:
		panic("unreachable down choice")
	}
}

// StopTarget is the resolved team or worker. Only the resolver can construct
// one, so execution cannot replace its observed identity with an unchecked name.
type StopTarget struct {
	team    team.Running
	worker  *downWorker
	tabs    []team.Tab
	stopped bool
}

// IsWorker reports which of the two stop operations accepts this target.
func (s StopTarget) IsWorker() bool { return s.worker != nil }

// ResolveStop resolves the CLI word against one reading of herdr, including a
// tab left by a start that stopped before a member ran.
func ResolveStop(input options.Input) (StopTarget, error) {
	tabs, agentsRead, err := herdrTabs()
	if err != nil {
		return StopTarget{}, err
	}
	running := team.RunningTeams(tabs, launchTeams(), agentsRead)
	chosen, worker, err := pickDown(running, tabs, input)
	if err == nil {
		if worker != nil {
			owner, _ := team.FindRunning(running, worker.Team.String())
			return StopTarget{team: owner, worker: worker, tabs: tabs}, nil
		}
		return teamStopTarget(chosen, tabs), nil
	}
	if _, exists := team.FindRunning(running, input.String()); exists {
		return StopTarget{}, err
	}
	if one, found := stoppedTeam(tabs, input.String()); found {
		return StopTarget{team: one, tabs: tabs, stopped: true}, nil
	}

	return StopTarget{}, err
}

// teamStopTarget selects a team already read by a screen. It uses the same
// stopped-pane route as the CLI when no member agent remains.
func teamStopTarget(one team.Running, tabs []team.Tab) StopTarget {
	_, running := team.FindRunning(team.RunningTeams(tabs, launchTeams(), true), one.Team.String())
	return StopTarget{team: one, tabs: tabs, stopped: !hasLiveAgent(tabs, one.Team.Observation()) && (!running || len(team.MemberPanes(tabs, one.Team.Observation())) > 0)}
}

// hasTeamTab includes a legacy team label and the team's worker tabs.
func hasTeamTab(tabs []team.Tab, name domainname.Seen) bool {
	for _, tab := range tabs {
		if protocol.NamesTeam(tab.Label.String(), name.String()) || domainname.IsWorkerTab(tab.Label, name) {
			return true
		}
	}
	return false
}

// stoppedTeam finds an empty team tab omitted from the running-team reading.
// Launch records supply the project and definition, never the existence.
func stoppedTeam(tabs []team.Tab, input string) (team.Running, bool) {
	if input == "" {
		return team.Running{}, false
	}
	for _, tab := range tabs {
		seen := tab.Label
		definition := ""
		if candidate, def, ok := tab.Label.TeamLabel(); ok {
			seen, definition = candidate, def
		}
		if worker, ok := tab.Label.WorkerIn(stopWord(input)); ok {
			seen = worker.Team()
		}
		if seen.String() != input || !hasTeamTab(tabs, seen) {
			continue
		}
		if definition == "" {
			definition = seen.String()
		}
		one := team.Running{Team: domainname.TeamFromObservation(seen), Definition: definition}
		for _, launch := range launchTeams() {
			if launch.Team == seen {
				one.Definition, one.Project = launch.Definition, launch.Project
			}
		}
		return one, true
	}
	return team.Running{}, false
}

func hasLiveAgent(tabs []team.Tab, name domainname.Seen) bool {
	running := team.RunningTeams(tabs, launchTeams(), true)
	for _, tab := range tabs {
		for _, agent := range tab.Agents {
			if teamOfAgent(running, agent) == name {
				return true
			}
		}
	}
	return false
}

// stopWord is unresolved input used only to scope an observed worker label.
// It cannot be passed to herdr or used as a model name.
type stopWord string

func (w stopWord) String() string { return string(w) }
