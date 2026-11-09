package turns

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Path is where one team's turn stands: <stateDir>/turns/<team>.json, beside
// activity.json and the launch record. stateDir is hergy's state directory
// ($XDG_STATE_HOME/hergy or ~/.local/state/hergy), which the caller resolves.
func Path(stateDir, team string) string {
	return filepath.Join(stateDir, "turns", team+".json")
}

// Load reads one team's turn. A file that is not there is a team whose turn
// hergy has not seen yet: the human holds it, and nothing waits.
func Load(path, team string) (*State, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{Team: team, Holder: Human}, nil
	}
	if err != nil {
		return nil, err
	}
	st := &State{}
	if err := json.Unmarshal(raw, st); err != nil {
		return nil, err
	}
	return st, nil
}

// Save writes the turn, whole, through a temporary file and a rename, so a
// reader never sees half of it.
func Save(path string, st *State) error {
	raw, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".turns-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Clear forgets one team's turn: the state file, the signals and the tells
// that waited. hergy down calls it once the team's panes are closed, so the
// next team of that name starts with the human holding the turn.
func Clear(stateDir, team string) error {
	var first error
	for _, path := range []string{Path(stateDir, team), SignalDir(stateDir, team), TellDir(stateDir, team)} {
		if err := os.RemoveAll(path); err != nil && first == nil {
			first = err
		}
	}
	return first
}
