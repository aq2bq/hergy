package ops

import (
	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"sort"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
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

// LeftoverOwner is whose worktrees a leftover list is about: the team that was
// started, or the worker that was hired, and when hergy recorded that in the
// launch record. Known is false when hergy has no line to read the time from —
// a team started before the record was kept, or a launch whose line could not be
// written — and hergy then cannot say when a worktree came (d-970c).
type LeftoverOwner struct {
	Name  string
	Since time.Time
	Known bool
	// Hired says the time is when the worker was hired, not when a team started.
	Hired bool
}

// teamLeftovers is the owner of a team's leftover list, read from the newest up
// line of that team in that project.
func teamLeftovers(projectRoot domainplace.Project, teamName string) LeftoverOwner {
	since, known := teamStartedAt(projectRoot, teamName)
	return LeftoverOwner{Name: teamName, Since: since, Known: known}
}

// workerLeftovers is the owner of one worker's leftover list, read from the hire
// line that named it.
func workerLeftovers(projectRoot domainplace.Project, teamName, herdrName string) LeftoverOwner {
	since, known := workerHiredAt(projectRoot, teamName, herdrName)
	return LeftoverOwner{Name: herdrName, Since: since, Known: known, Hired: true}
}

// teamStartedAt is when hergy last started this team in this project: the newest
// up line of the launch record for that team and project.
func teamStartedAt(projectRoot domainplace.Project, teamName string) (time.Time, bool) {
	records, ok := availableLaunchRecords()
	if !ok {
		return time.Time{}, false
	}
	var best time.Time
	for _, record := range records {
		if record.Command != launchlog.CommandUp || record.Team.String() != teamName || record.Project.Project().Compare(projectRoot) == domainplace.Different {
			continue
		}
		if at, ok := recordTime(record); ok && at.After(best) {
			best = at
		}
	}
	return best, !best.IsZero()
}

// workerHiredAt is when hergy recorded hiring this worker: the newest hire line
// that named it. A worker hired before hergy wrote hire lines has no line, and
// its team's start is the earliest time it can have been hired.
func workerHiredAt(projectRoot domainplace.Project, teamName, herdrName string) (time.Time, bool) {
	records, ok := availableLaunchRecords()
	if !ok {
		return time.Time{}, false
	}
	var best time.Time
	for _, record := range records {
		if !record.IsHire() || record.Hire == nil || record.Hire.Worker.String() != herdrName {
			continue
		}
		if record.Project.Project().Compare(projectRoot) == domainplace.Different {
			continue
		}
		if at, ok := recordTime(record); ok && at.After(best) {
			best = at
		}
	}
	if best.IsZero() {
		return teamStartedAt(projectRoot, teamName)
	}
	return best, true
}

// recordTime is when one line of the launch record was written. A line whose
// time does not read has no time hergy can use.
func recordTime(record launchlog.Record) (time.Time, bool) {
	at, err := time.Parse(time.RFC3339, record.Time)
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

// readLaunchRecords is this machine's launch record. A record hergy cannot read
// is no record: the list then falls back to where the panes stood.
func availableLaunchRecords() ([]launchlog.Record, bool) {
	log, err := launchlog.Open()
	if err != nil {
		return nil, false
	}
	records, err := log.Read()
	if err != nil {
		return nil, false
	}
	return records, true
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
