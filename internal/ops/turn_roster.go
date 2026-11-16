package ops

import (
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	observedname "github.com/aq2bq/hergy/internal/ops/internal/name"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/internal/screenlog"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
	"github.com/aq2bq/hergy/internal/source"
)

func turnRoster(graph *definition.Graph, run runningstate.State) func(string) bool {
	names := map[string]bool{}
	if graph != nil {
		for _, member := range graph.Members {
			names[member.Name.String()] = true
		}
	}
	for _, worker := range run.Workers() {
		names[domainname.ShortIn(observedname.Observe(worker.Name), run.Name().Observation()).String()] = true
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

// A broken definition must not prevent closing a worker. Without a readable
// roster, only that worker is known absent; the other parties are preserved.
func forgetStoppedWorker(in StopInput, worker *downWorker, run runningstate.State) error {
	roster := func(name string) bool { return name != worker.Short.String() }
	launch := run.Launch()
	cat := catalogOfRun(in.Env.Catalog(source.Disk{}), launch)
	if def, err := cat.Definition(launch.Definition.String(), launch.Layer.String()); err == nil && def.TeamYAML.Project().Compare(launch.TeamYAML) == domainplace.Same {
		roster = turnRoster(def.Graph, run)
	}
	dir, err := screenlog.DefaultDir()
	if err != nil {
		return err
	}
	_, err = loadTurnWithRoster(turns.Path(dir, worker.Team.String()), worker.Team, roster, operationTime(in.Now))
	return err
}
