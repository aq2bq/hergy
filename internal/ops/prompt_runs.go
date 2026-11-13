package ops

import (
	"fmt"
	"sync"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/ops/internal/catalog"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
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
	value HQRunning
}

const runningHoldFor = 2 * time.Second

// within is the reading held for runningHoldFor, or a fresh one.
func (h *runningHold) within(now time.Time, read func() HQRunning) HQRunning {
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
func (a *Reader) liveRunning() HQRunning {
	return a.runs.within(a.clock(), a.hq.Running)
}

// PromptRuns is every team of one definition that runs, in herdr's order: the
// same filter PromptHistory used when it chose a team itself.
func (a *Reader) PromptRuns(project domainplace.Project, definition, place string) ([]PromptRun, error) {
	live := a.liveRunning()
	if !live.Herdr.OK {
		return nil, fmt.Errorf("%s", live.Herdr.Text)
	}
	return promptRunsOf(live, project, definition, place), nil
}

// promptRunsOf is the filter both readers use: the teams of one definition, in
// the project and place it names.
func promptRunsOf(live HQRunning, project domainplace.Project, definition, place string) []PromptRun {
	out := []PromptRun{}
	for _, r := range live.Teams {
		if r.Definition != definition || r.Place != place {
			continue
		}
		if place != catalog.PlaceGlobal && r.Project.Compare(project) != domainplace.Same {
			continue
		}
		out = append(out, PromptRun{Team: r.Team, Project: r.Project})
	}
	return out
}

// PromptHistory is the recorded body of one member and the live state of the
// team it was asked for. The team is chosen by the web owner now; this reads the
// record and says whether the member is live.
func (a *Reader) PromptHistory(_ domainplace.Project, definition, place, member, running string) (PromptHistory, error) {
	out := PromptHistory{Runs: []PromptRun{}, Status: "missing", Reason: "このメンバーへ渡した本文の記録がありません"}
	live := a.liveRunning()
	if !live.Herdr.OK {
		return out, fmt.Errorf("%s", live.Herdr.Text)
	}
	var team *HQRunningTeam
	for i := range live.Teams {
		if live.Teams[i].Team.String() != running {
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
	store, err := runningstate.Open()
	if err != nil {
		return out, err
	}
	key, err := domainname.ParseTeamName(team.Team.String())
	if err != nil {
		return out, err
	}
	state, exists, err := store.Read(key)
	if err != nil {
		return out, err
	}
	if !exists {
		return out, nil
	}
	cat, err := a.hq.catalogAt(team.Project)
	if err != nil {
		return out, err
	}
	cat = catalogOfRun(cat, state.Launch())
	path := cat.TeamPath(definition, domainplace.Selection(place).Layer())
	entry, ok := lastPromptReceipt(state, team.Project, path, definition, member)
	if !ok {
		out.Status = "missing"
		out.Reason = "このメンバーへ渡した本文を一意に特定できる記録がありません"
		return out, nil
	}
	out.Status = "recorded"
	out.Reason = ""
	out.Recorded = &RecordedPrompt{Text: entry.Prompt, Command: entry.Command, Time: entry.Time.Format(time.RFC3339)}
	return out, nil
}
