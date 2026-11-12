package turns

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// TellDir is where one team's tells wait for the watcher:
// <stateDir>/turns/<team>.tells/, one file each, written by hergy tell and
// taken by hergy turns the way signals are (d-8b4e). A tell is how hergy knows
// who opened whose turn.
func TellDir(stateDir, team string) string {
	return filepath.Join(stateDir, "turns", team+".tells")
}

// WriteTell puts one tell in the team's directory, through a temporary file
// and a rename. The name orders them by time.
func WriteTell(dir string, tell Tell) error {
	return writeNote(dir, tell, tell.At, tell.To)
}

// TakeTells reads every tell waiting in the directory, oldest first, and
// removes them. A directory that is not there holds none.
func TakeTells(dir string) ([]Tell, error) {
	return takeNotes[Tell](dir)
}

// writeNote writes one JSON note into dir under a name that sorts by at.
func writeNote(dir string, note any, at time.Time, who string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(note)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".note-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	name := strconv.FormatInt(at.UnixNano(), 10) + "-" + who + ".json"
	return os.Rename(tmp.Name(), filepath.Join(dir, name))
}

// takeNotes reads every JSON note in dir, oldest first by name, and removes
// them. A directory that is not there holds none.
func takeNotes[T any](dir string) ([]T, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && e.Name()[0] != '.' {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []T
	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return out, err
		}
		var note T
		if err := json.Unmarshal(raw, &note); err == nil {
			out = append(out, note)
		}
		if err := os.Remove(path); err != nil {
			return out, err
		}
	}
	return out, nil
}
