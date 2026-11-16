package ops

import (
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	observedname "github.com/aq2bq/hergy/internal/ops/internal/name"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/internal/screenlog"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
)

func turnRoster(graph *definition.Graph, run runningstate.State, live ...turns.Agent) func(string) bool {
	names := map[string]bool{}
	if graph != nil {
		for _, member := range graph.Members {
			names[member.Name.String()] = true
		}
	}
	for _, worker := range run.Workers() {
		names[domainname.ShortIn(observedname.Observe(worker.Name), run.Name().Observation()).String()] = true
	}
	for _, agent := range live {
		names[agent.Name] = true
	}
	return func(name string) bool { return names[name] }
}

func readTurnRun(name domainname.TeamName) (runningstate.State, error) {
	store, err := runningstate.Open()
	if err != nil {
		return runningstate.State{}, err
	}
	run, _, err := store.Read(name)
	return run, err
}

func loadTurnWithRoster(path string, name domainname.Seen, roster func(string) bool, at time.Time) (*turns.State, error) {
	st, err := turns.Load(path, name)
	if err != nil {
		return nil, err
	}
	if st.ForgetAbsent(roster, at) {
		if err := turns.Save(path, st); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// Closing a worker establishes only that worker's absence. The watcher owns
// the complete roster; other parties may be live before their hire is recorded.
func forgetStoppedWorker(in StopInput, worker *downWorker) error {
	roster := func(name string) bool { return name != worker.Short.String() }
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	_, err = loadTurnWithRoster(turns.Path(dir, worker.Team.String()), worker.Team, roster, operationTime(in.Now))
	return err
}
