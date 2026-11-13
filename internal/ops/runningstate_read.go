package ops

import (
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
)

// currentTeamState reads one run without consulting history. Optional metadata
// readers leave names and times unknown when the state cannot be read.
func currentTeamState(name domainname.TeamName) (runningstate.State, bool) {
	store, err := runningstate.Open()
	if err != nil {
		return runningstate.State{}, false
	}
	state, exists, err := store.Read(name)
	return state, exists && err == nil
}

func projectTeamState(project domainplace.Project, raw string) (runningstate.State, bool) {
	name, err := domainname.ParseTeamName(raw)
	if err != nil {
		return runningstate.State{}, false
	}
	state, exists := currentTeamState(name)
	if !exists || state.Launch().Project.Compare(project) != domainplace.Same {
		return runningstate.State{}, false
	}
	return state, true
}
