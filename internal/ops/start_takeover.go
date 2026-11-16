package ops

import (
	"slices"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/internal/screenlog"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
	"github.com/aq2bq/hergy/internal/words"
)

// takeOverOlderTeam gives a state to a team an older hergy started. Such a team
// has its tab and its agents in herdr and no state, so no command finds it
// running, and a start under its name is refused because the names are taken:
// the team can neither be used nor started. When the tab labelled with the team
// name holds a live agent for every member of the definition, the start writes
// the state and stops there: no agent is started, ended or sent anything, so the
// members keep their conversations.
//
// What each member was handed is not known, so the state holds no receipt, and
// hergy apply reads every member as one with no record.
func takeOverOlderTeam(in StartInput, r *Resolved) (bool, error) {
	store, err := runningstate.Open()
	if err != nil {
		return false, nil
	}
	if _, exists, err := store.Read(r.ModelTeamName()); err != nil || exists {
		return false, nil
	}
	read := readSession(in.Port)
	if read.Err != nil {
		return false, nil
	}
	tab, found := teamTab(read.Tabs, r.Team.Name.Observation())
	if !found || len(r.Starts) == 0 {
		return false, nil
	}
	live := make([]string, 0, len(tab.Agents))
	for _, agent := range tab.Agents {
		live = append(live, agent.String())
	}
	for _, member := range r.Starts {
		if !slices.Contains(live, member.Name.String()) {
			return false, nil
		}
	}
	workspace := runningstate.Workspace{ID: tab.WorkspaceID}
	for _, space := range read.Workspaces {
		if space.ID == tab.WorkspaceID {
			workspace.Label = space.Label
		}
	}
	if r.generation, err = beginRunningState(store, in.Env, r, workspace, in.Now); err != nil {
		return false, words.Errorf("up.state-unavailable", i18n.Language(), err)
	}
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return false, err
	}
	if _, err := loadTurnWithRoster(turns.Path(dir, r.Team.Name.String()), r.Team.Name.Observation(), turnRoster(r.Team.Definition.Graph, runningstate.State{}), operationTime(in.Now)); err != nil {
		return false, err
	}
	emit(in.Observe, Event{Text: i18n.Tf("up.took_over", r.Team.Name.String(), tab.ID)})
	return true, nil
}
