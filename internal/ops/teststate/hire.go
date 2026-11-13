package teststate

import (
	"fmt"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
)

// Worker is a hire in a registered test run.
type Worker runningstate.Worker

// Hire adds a worker with the current generation, including for CLI tests that
// cannot import ops/internal/runningstate directly. It never creates a run.
func Hire(team string, worker Worker) error {
	name, err := domainname.ParseTeamName(team)
	if err != nil {
		return err
	}
	store, err := runningstate.Open()
	if err != nil {
		return err
	}
	state, exists, err := store.Read(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("test run %s does not exist", team)
	}
	return store.Hire(name, state.Generation(), runningstate.Worker(worker))
}
