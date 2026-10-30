package app

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
)

// downPlan is hergy down --dry --json's answer: the names it would close, and
// what the panes would leave in git (r-b65e).
type downPlan struct {
	Team      string          `json:"team"`
	Members   []string        `json:"members"`
	Workers   []string        `json:"workers"`
	Leftovers web.HQLeftovers `json:"leftovers"`
}

// writeDownPlan writes the plan as JSON, with empty lists rather than null.
func writeDownPlan(w io.Writer, plan downPlan) error {
	if plan.Members == nil {
		plan.Members = []string{}
	}
	if plan.Workers == nil {
		plan.Workers = []string{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}

// workerNamesOf is the herdr names of one team's workers, by their tabs.
func workerNamesOf(tabs []team.Tab, teamName string) []string {
	var out []string
	for _, tab := range tabs {
		if short, ok := workerOf(tab.Label, teamName); ok {
			out = append(out, teamName+"-"+short)
		}
	}
	sort.Strings(out)
	return out
}

// teamPaneDirs is where the panes of one team stood: every named pane in the
// tab labelled with the team's name and in the worker tabs hergy made for it.
// It is read before the panes close, so the worktrees are known afterwards
// (r-b65e).
func teamPaneDirs(tabs []team.Tab, name string) []string {
	prefix := protocol.WorkerTabPrefix(name)
	var dirs []string
	for _, tab := range tabs {
		if !protocol.NamesTeam(tab.Label, name) && !strings.HasPrefix(tab.Label, prefix) {
			continue
		}
		dirs = append(dirs, tab.PaneDirs...)
	}
	return dirs
}

// workerPaneDirs is where one worker's pane stood, read before it closes.
func workerPaneDirs(tabs []team.Tab, label string) []string {
	for _, tab := range tabs {
		if tab.Label == label {
			return append([]string(nil), tab.PaneDirs...)
		}
	}
	return nil
}

// leftoverOwner is whose worktrees a leftover list is about: the team that was
// started, or the worker that was hired, and when hergy recorded that in the
// launch record. Known is false when hergy has no line to read the time from —
// a team started before the record was kept, or a launch whose line could not be
// written — and hergy then cannot say when a worktree came (d-970c).
type leftoverOwner struct {
	Name  string
	Since time.Time
	Known bool
	// Hired says the time is when the worker was hired, not when a team started.
	Hired bool
}

// teamLeftovers is the owner of a team's leftover list, read from the newest up
// line of that team in that project.
func teamLeftovers(projectRoot, teamName string) leftoverOwner {
	since, known := teamStartedAt(projectRoot, teamName)
	return leftoverOwner{Name: teamName, Since: since, Known: known}
}

// workerLeftovers is the owner of one worker's leftover list, read from the hire
// line that named it.
func workerLeftovers(projectRoot, teamName, herdrName string) leftoverOwner {
	since, known := workerHiredAt(projectRoot, teamName, herdrName)
	return leftoverOwner{Name: herdrName, Since: since, Known: known, Hired: true}
}

// teamStartedAt is when hergy last started this team in this project: the newest
// up line of the launch record for that team and project.
func teamStartedAt(projectRoot, teamName string) (time.Time, bool) {
	records, ok := readLaunchRecords()
	if !ok {
		return time.Time{}, false
	}
	var best time.Time
	for _, record := range records {
		if record.Command != launchlog.CommandUp || record.Team != teamName || !sameProject(record.Project, projectRoot) {
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
func workerHiredAt(projectRoot, teamName, herdrName string) (time.Time, bool) {
	records, ok := readLaunchRecords()
	if !ok {
		return time.Time{}, false
	}
	var best time.Time
	for _, record := range records {
		if !record.IsHire() || record.Hire == nil || record.Hire.Worker != herdrName {
			continue
		}
		if !sameProject(record.Project, projectRoot) {
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

// sameProject reports whether a record's project is the one the team runs in. A
// line written before the project was kept names none, and is not another
// project: the team's name still tells the team apart.
func sameProject(recorded, projectRoot string) bool {
	if recorded == "" || projectRoot == "" {
		return true
	}
	return filepath.Clean(recorded) == filepath.Clean(projectRoot)
}

// readLaunchRecords is this machine's launch record. A record hergy cannot read
// is no record: the list then falls back to where the panes stood.
func readLaunchRecords() ([]launchlog.Record, bool) {
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

// leftoverHeading is the list's first line: it says when the worktrees were
// made, because hergy cannot say which of them the team made (d-970c).
func leftoverHeading(owner leftoverOwner, dry bool) string {
	switch {
	case !owner.Known && dry:
		return i18n.Tf("leftovers.heading_unrecorded_dry", owner.Name)
	case !owner.Known:
		return i18n.Tf("leftovers.heading_unrecorded", owner.Name)
	case owner.Hired && dry:
		return i18n.Tf("leftovers.heading_worker_dry", owner.Name)
	case owner.Hired:
		return i18n.Tf("leftovers.heading_worker", owner.Name)
	case dry:
		return i18n.Tf("leftovers.heading_team_dry", owner.Name)
	}
	return i18n.Tf("leftovers.heading_team", owner.Name)
}

// printLeftovers lists what the closed panes left in git: the worktrees of the
// project that came while the team was running or after the worker was hired,
// the project's stashes, and the commands that remove them. hergy removes
// nothing (d-778e), and says nothing when nothing is left. dry shows the same
// list as a plan.
func printLeftovers(w io.Writer, projectRoot string, dirs []string, owner leftoverOwner, dry bool) {
	since := owner.Since
	if !owner.Known {
		// Without a start line hergy cannot narrow the list by time, so it
		// lists the project's worktrees rather than nothing.
		since = time.Time{}
	}
	left := team.GitLeftovers(projectRoot, since, dirs)
	if left.Empty() {
		return
	}
	fmt.Fprintln(w, leftoverHeading(owner, dry))
	for _, worktree := range left.Worktrees {
		branch := worktree.Branch
		if branch == "" {
			branch = i18n.T("leftovers.no_branch")
		}
		dirty := i18n.T("leftovers.clean")
		if worktree.Dirty {
			dirty = i18n.T("leftovers.dirty")
		}
		ahead := i18n.T("leftovers.ahead_unknown")
		if worktree.Base != "" {
			ahead = i18n.Tf("leftovers.ahead", worktree.Base, worktree.Ahead)
		}
		pane := ""
		if worktree.PaneCwd {
			pane = i18n.T("leftovers.pane_cwd")
		}
		fmt.Fprintf(w, i18n.T("leftovers.worktree"), worktree.Path, branch, dirty, ahead, pane)
		fmt.Fprintf(w, i18n.T("leftovers.remove"), worktree.Path)
		if worktree.Dirty || worktree.Ahead > 0 {
			fmt.Fprint(w, i18n.T("leftovers.check"))
		}
	}
	if left.Stashes > 0 {
		fmt.Fprintf(w, i18n.T("leftovers.stashes"), left.Stashes)
	}
	fmt.Fprint(w, i18n.T("leftovers.note"))
}
