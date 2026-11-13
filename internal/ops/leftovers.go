package ops

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"sort"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
)

// workerNamesOf is the herdr names of one team's workers, by their tabs.
func workerNamesOf(tabs []team.Tab, teamName domainname.Seen) []domainname.Seen {
	var out []domainname.Seen
	for _, tab := range tabs {
		if worker, ok := tab.Label.WorkerIn(teamName); ok {
			out = append(out, worker.Agent())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// teamPaneDirs is where the panes of one team stood: every named pane in the
// tab labelled with the team's name and in the worker tabs hergy made for it.
// It is read before the panes close, so the worktrees are known afterwards
// (r-b65e).
func teamPaneDirs(tabs []team.Tab, name domainname.Seen) []domainplace.Seen {
	workerTeam := name
	var dirs []domainplace.Seen
	for _, tab := range tabs {
		if !protocol.NamesTeam(tab.Label.String(), name.String()) && !domainname.IsWorkerTab(tab.Label.Observation(), workerTeam) {
			continue
		}
		dirs = append(dirs, tab.PaneDirs...)
	}
	return dirs
}

// workerPaneDirs is where one worker's pane stood, read before it closes.
func workerPaneDirs(tabs []team.Tab, label domainname.Seen) []domainplace.Seen {
	for _, tab := range tabs {
		if tab.Label == label {
			return append([]domainplace.Seen(nil), tab.PaneDirs...)
		}
	}
	return nil
}

// LeftoverOwner identifies the team or worker and its time in the current
// run. Without readable state, worktrees are listed without a time cutoff.
type LeftoverOwner struct {
	Name  string
	Since time.Time
	Known bool
	// Hired says the time is when the worker was hired, not when a team started.
	Hired bool
}

// teamLeftovers reads the current run's start time.
func teamLeftovers(projectRoot domainplace.Project, teamName string) LeftoverOwner {
	since, known := teamStartedAt(projectRoot, teamName)
	return LeftoverOwner{Name: teamName, Since: since, Known: known}
}

// workerLeftovers reads the worker's hire time in the current run.
func workerLeftovers(projectRoot domainplace.Project, teamName, herdrName string) LeftoverOwner {
	since, known := workerHiredAt(projectRoot, teamName, herdrName)
	return LeftoverOwner{Name: herdrName, Since: since, Known: known, Hired: true}
}

// teamStartedAt is the beginning of this team's current run in this project.
func teamStartedAt(projectRoot domainplace.Project, teamName string) (time.Time, bool) {
	state, exists := projectTeamState(projectRoot, teamName)
	if !exists {
		return time.Time{}, false
	}
	at := state.Launch().StartedAt
	return at, !at.IsZero()
}

// workerHiredAt uses only this run's hire. An unregistered worker has no time;
// the beginning of a team is not evidence that the worker was hired then.
func workerHiredAt(projectRoot domainplace.Project, teamName, herdrName string) (time.Time, bool) {
	state, exists := projectTeamState(projectRoot, teamName)
	if !exists {
		return time.Time{}, false
	}
	for _, worker := range state.Workers() {
		if worker.Name == herdrName {
			return worker.Time, !worker.Time.IsZero()
		}
	}
	return time.Time{}, false
}

// TeamIdentity is the observed identity and source of a running team.
type TeamIdentity team.Running

// Worktree is the retained git worktree a person may inspect after stopping.
type Worktree team.Worktree

// Leftovers is a read-only list of retained worktrees and stashes.
type Leftovers struct {
	Worktrees []Worktree
	Stashes   int
}

// Empty reports whether the stop left no worktree or stash.
func (l Leftovers) Empty() bool { return len(l.Worktrees) == 0 && l.Stashes == 0 }
func readLeftovers(project domainplace.Project, since time.Time, dirs []domainplace.Seen) Leftovers {
	raw := team.GitLeftovers(project, since, dirs)
	out := Leftovers{Stashes: raw.Stashes}
	for _, w := range raw.Worktrees {
		out.Worktrees = append(out.Worktrees, Worktree(w))
	}
	return out
}
