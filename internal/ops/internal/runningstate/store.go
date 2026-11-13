package runningstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

// Store writes one file per team. There is no log replay or backfill.
type Store struct{ dir string }

// Open selects this machine's state directory using XDG's absolute-path rule.
func Open() (Store, error) {
	root := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(root) {
		home, err := os.UserHomeDir()
		if err != nil {
			return Store{}, err
		}
		root = filepath.Join(home, ".local", "state")
	}
	return At(filepath.Join(root, "hergy", "teams")), nil
}

// At selects an explicit state directory, principally for tests.
func At(dir string) Store { return Store{dir: dir} }

// Path is the one JSON file for a checked team name.
func (s Store) Path(name domainname.TeamName) string {
	return filepath.Join(s.dir, name.String()+".json")
}

// Begin replaces all facts of a previous run, including receipts and workers.
func (s Store) Begin(name domainname.TeamName, launch Launch) error {
	return s.locked(name, func() error {
		data := snapshot{Team: name.String(), Definition: launch.Definition.String(), Project: launch.Project.String(),
			Layer: launch.Layer, TeamYAML: launch.TeamYAML.String(), Workspace: launch.Workspace, StartedAt: launch.StartedAt,
			Executable: launch.Executable, Lang: launch.Lang, WorkerDefinitions: launch.Workers}
		return s.write(name, data)
	})
}

// Read returns this run, or false when it is absent. A malformed file is an error.
func (s Store) Read(name domainname.TeamName) (State, bool, error) {
	if _, err := domainname.ParseTeamName(name.String()); err != nil {
		return State{}, false, err
	}
	raw, err := os.ReadFile(s.Path(name))
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	var data snapshot
	if err := json.Unmarshal(raw, &data); err != nil {
		return State{}, false, err
	}
	if data.Team != name.String() {
		return State{}, false, fmt.Errorf("%s: team name does not match", s.Path(name))
	}
	if _, err := domainname.ParseDefinitionName(data.Definition); err != nil {
		return State{}, false, err
	}
	for _, path := range []string{data.Project, data.TeamYAML} {
		if !filepath.IsAbs(path) {
			return State{}, false, fmt.Errorf("%s: expected absolute path", path)
		}
		if _, err := domainplace.NewProject(path); err != nil {
			return State{}, false, err
		}
	}
	return State{data: data}, true, nil
}

// List reads current teams in filename order. Lock files are in a subdirectory.
func (s Store) List() ([]State, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]State, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name, err := domainname.ParseTeamName(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		state, exists, err := s.Read(name)
		if err != nil {
			return nil, err
		}
		if exists {
			out = append(out, state)
		}
	}
	return out, nil
}

// Receive replaces only the confirmed members. Missing state is left missing:
// teams started before this store existed must be stopped and started again.
func (s Store) Receive(name domainname.TeamName, receipts []Receipt) error {
	if len(receipts) == 0 {
		return nil
	}
	return s.update(name, func(data *snapshot) {
		for _, receipt := range receipts {
			index := slices.IndexFunc(data.Members, func(member Receipt) bool { return member.Name == receipt.Name })
			if index < 0 {
				data.Members = append(data.Members, receipt)
			} else {
				data.Members[index] = receipt
			}
		}
	})
}

// Hire adds or replaces one worker without changing the team's receipts.
func (s Store) Hire(name domainname.TeamName, worker Worker) error {
	return s.update(name, func(data *snapshot) {
		index := slices.IndexFunc(data.Workers, func(old Worker) bool { return old.Name == worker.Name })
		if index < 0 {
			data.Workers = append(data.Workers, worker)
		} else {
			data.Workers[index] = worker
		}
	})
}

// RemoveWorker ends only this worker's membership.
func (s Store) RemoveWorker(name domainname.TeamName, worker string) error {
	return s.update(name, func(data *snapshot) {
		data.Workers = slices.DeleteFunc(data.Workers, func(old Worker) bool { return old.Name == worker })
	})
}

// End removes the team's state after its panes have been closed.
func (s Store) End(name domainname.TeamName) error {
	return s.locked(name, func() error {
		err := os.Remove(s.Path(name))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	})
}

func (s Store) update(name domainname.TeamName, change func(*snapshot)) error {
	return s.locked(name, func() error {
		state, exists, err := s.Read(name)
		if err != nil || !exists {
			return err
		}
		change(&state.data)
		return s.write(name, state.data)
	})
}

func (s Store) write(name domainname.TeamName, data snapshot) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".team-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.Path(name))
}
