package app

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/web"
)

// PromptHistory is a read-only view of live membership and recorded bodies.
// A successful record is not a receipt; the sending preamble is not recorded.
func (a *activityRead) PromptHistory(project, definition, place, member, running string) (web.PromptHistory, error) {
	out := web.PromptHistory{Runs: []web.PromptRun{}, Status: "stopped", Reason: "動いているチームがないため、今の文だけを表示します"}
	live := a.hq.Running()
	if !live.Herdr.OK {
		return out, fmt.Errorf("%s", live.Herdr.Text)
	}
	candidates := []web.HQRunningTeam{}
	for _, r := range live.Teams {
		if r.Definition != definition || r.Place != place {
			continue
		}
		if place != "global" && !samePath(r.Project, project) {
			continue
		}
		candidates = append(candidates, r)
		out.Runs = append(out.Runs, web.PromptRun{Team: r.Team, Project: r.Project})
	}
	selected := -1
	for i, r := range candidates {
		if r.Team == running {
			selected = i
		}
	}
	if running == "" && len(candidates) == 1 {
		selected = 0
	}
	if selected < 0 {
		if len(candidates) > 0 {
			out.Status = "choose"
			out.Reason = "実行中のチームを選んでください"
		}
		return out, nil
	}
	team := candidates[selected]
	out.Selected = &out.Runs[selected]
	active := false
	for _, m := range team.Members {
		if m.Name == member && m.Live {
			active = true
		}
	}
	if !active {
		out.Reason = "このメンバーは動いていないため、今の文だけを表示します"
		return out, nil
	}
	cat, err := a.hq.catalogAt(team.Project)
	if err != nil {
		return out, err
	}
	path := cat.TeamYAML(definition, catalog.Layer(place))
	log, err := launchlog.Open()
	if err != nil {
		return out, err
	}
	records, err := log.Read()
	if err != nil {
		return out, err
	}
	record, entry, ok := lastPromptRecord(records, team.Project, path, definition, team.Team, member)
	if !ok {
		out.Status = "missing"
		out.Reason = "このメンバーへ渡した本文を一意に特定できる記録がありません"
		return out, nil
	}
	hash, err := hex.DecodeString(entry.Prompt)
	if err != nil || len(hash) != 32 {
		return out, fmt.Errorf("記録の本文ハッシュが壊れています")
	}
	text, err := log.Text(entry.Prompt)
	if err != nil {
		return out, fmt.Errorf("記録の本文を読めません: %w", err)
	}
	out.Status = "recorded"
	out.Reason = ""
	out.Recorded = &web.RecordedPrompt{Text: string(text), Command: record.Command, Time: record.Time}
	return out, nil
}

// Exact definition paths let legacy project-less records be used safely. Never
// borrow another project's or another definition's same-named member.
func lastPromptRecord(records []launchlog.Record, project, path, definition, team, member string) (launchlog.Record, launchlog.Member, bool) {
	for _, r := range slices.Backward(records) {
		if r.Team != team || r.Definition != definition || !promptCommand(r.Command) {
			continue
		}
		if r.Project != "" && !samePath(r.Project, project) {
			continue
		}
		if !filepath.IsAbs(r.TeamYAML) || !samePath(r.TeamYAML, path) {
			continue
		}
		for _, m := range r.Members {
			if m.Name == team+"-"+member {
				return r, m, true
			}
		}
	}
	return launchlog.Record{}, launchlog.Member{}, false
}
func promptCommand(command string) bool {
	switch command {
	case launchlog.CommandUp, launchlog.CommandApply, launchlog.CommandReprompt, launchlog.CommandRefresh:
		return true
	}
	return false
}
