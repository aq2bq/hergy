package app

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/web"
)

// This file is the live half of the prompt reading (⑤ r-ba9b): which teams of
// one definition run, and which recorded body one member received. The prompt's
// text is not built here; the web owner (promptsFor) builds it. The two readers
// share one reading of herdr, held for two seconds, so a check and a comparison
// in the same moment do not read herdr twice.

// runningHold keeps one headquarters reading for a moment (d-002b). The prompt
// column and the check both ask while a person types, and herdr's reading can be
// a command; two seconds keeps the two questions one read without letting a
// start or a stop wait longer than the screens' own ten-second reading.
type runningHold struct {
	mu    sync.Mutex
	at    time.Time
	has   bool
	value web.HQRunning
}

const runningHoldFor = 2 * time.Second

// within is the reading held for runningHoldFor, or a fresh one.
func (h *runningHold) within(now time.Time, read func() web.HQRunning) web.HQRunning {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.has && now.Sub(h.at) < runningHoldFor {
		return h.value
	}
	h.value, h.at, h.has = read(), now, true
	return h.value
}

// liveRunning is the running teams the prompt reader shares with the activity
// screen's reading, held for runningHoldFor.
func (a *activityRead) liveRunning() web.HQRunning {
	return a.runs.within(a.clock(), a.hq.Running)
}

// PromptRuns is every team of one definition that runs, in herdr's order: the
// same filter PromptHistory used when it chose a team itself.
func (a *activityRead) PromptRuns(project, definition, place string) ([]web.PromptRun, error) {
	live := a.liveRunning()
	if !live.Herdr.OK {
		return nil, fmt.Errorf("%s", live.Herdr.Text)
	}
	return promptRunsOf(live, project, definition, place), nil
}

// promptRunsOf is the filter both readers use: the teams of one definition, in
// the project and place it names.
func promptRunsOf(live web.HQRunning, project, definition, place string) []web.PromptRun {
	out := []web.PromptRun{}
	for _, r := range live.Teams {
		if r.Definition != definition || r.Place != place {
			continue
		}
		if place != catalog.PlaceGlobal && !samePath(r.Project, project) {
			continue
		}
		out = append(out, web.PromptRun{Team: r.Team, Project: r.Project})
	}
	return out
}

// PromptHistory is the recorded body of one member and the live state of the
// team it was asked for. The team is chosen by the web owner now; this reads the
// record and says whether the member is live.
func (a *activityRead) PromptHistory(project, definition, place, member, running string) (web.PromptHistory, error) {
	out := web.PromptHistory{Runs: []web.PromptRun{}, Status: "missing", Reason: "このメンバーへ渡した本文の記録がありません"}
	live := a.liveRunning()
	if !live.Herdr.OK {
		return out, fmt.Errorf("%s", live.Herdr.Text)
	}
	var team *web.HQRunningTeam
	for i := range live.Teams {
		if live.Teams[i].Team != running {
			continue
		}
		team = &live.Teams[i]
		break
	}
	if team == nil {
		out.Status = "stopped"
		out.Reason = "動いているチームがないため、今の文だけを表示します"
		return out, nil
	}
	active := false
	for _, m := range team.Members {
		if m.Name == member && m.Live {
			active = true
		}
	}
	if !active {
		out.Status = "stopped"
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
