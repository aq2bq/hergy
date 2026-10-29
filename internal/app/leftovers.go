package app

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
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

// printLeftovers lists what the closed panes left in git: the worktrees of the
// project they stood in, the project's stashes, and the commands that remove them.
// hergy removes nothing (d-778e), and says nothing when nothing is left. dry
// shows the same list as a plan.
func printLeftovers(w io.Writer, projectRoot string, dirs []string, dry bool) {
	left := team.GitLeftovers(projectRoot, dirs)
	if left.Empty() {
		return
	}
	if dry {
		fmt.Fprintln(w, i18n.T("leftovers.heading_dry"))
	} else {
		fmt.Fprintln(w, i18n.T("leftovers.heading"))
	}
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
		fmt.Fprintf(w, i18n.T("leftovers.worktree"), worktree.Path, branch, dirty, ahead)
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
