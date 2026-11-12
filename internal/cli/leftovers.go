package cli

import (
	"encoding/json"
	"fmt"
	"io"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/i18n"
)

// downPlan is hergy down --dry --json's answer: the names it would close, and
// what the panes would leave in git (r-b65e).
type downPlan struct {
	Team      string            `json:"team"`
	Members   []domainname.Seen `json:"members"`
	Workers   []domainname.Seen `json:"workers"`
	Leftovers jsonLeftovers     `json:"leftovers"`
}

// writeDownPlan writes the plan as JSON, with empty lists rather than null.
func writeDownPlan(w io.Writer, plan downPlan) error {
	if plan.Members == nil {
		plan.Members = []domainname.Seen{}
	}
	if plan.Workers == nil {
		plan.Workers = []domainname.Seen{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}

// leftoverHeading is the list's first line: it says when the worktrees were
// made, because hergy cannot say which of them the team made (d-970c).
func leftoverHeading(owner ops.LeftoverOwner, dry bool) string {
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
func printLeftovers(w io.Writer, left ops.Leftovers, owner ops.LeftoverOwner, dry bool) {
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

type jsonLeftovers struct {
	Worktrees []jsonWorktree `json:"worktrees"`
	Stashes   int            `json:"stashes"`
}

type jsonWorktree struct {
	Path   domainplace.Project `json:"path"`
	Branch string              `json:"branch"`
	Dirty  bool                `json:"dirty"`
	Ahead  int                 `json:"ahead"`
	Base   string              `json:"base"`
	// PaneCwd says a closed pane's working directory was this worktree (d-970c).
	PaneCwd bool `json:"pane_cwd"`
}

func hqLeftoversOf(left ops.Leftovers) jsonLeftovers {
	out := jsonLeftovers{Worktrees: []jsonWorktree{}, Stashes: left.Stashes}
	for _, one := range left.Worktrees {
		out.Worktrees = append(out.Worktrees, jsonWorktree{
			Path: one.Path, Branch: one.Branch, Dirty: one.Dirty, Ahead: one.Ahead, Base: one.Base,
			PaneCwd: one.PaneCwd,
		})
	}
	return out
}
