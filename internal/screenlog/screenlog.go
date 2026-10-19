// Package screenlog remembers, for each member, what screen hergy status last
// saw it with and when that screen was first seen. status reads it to tell a
// member that is working and whose screen has not changed, so a person looking
// at a quiet team can tell a long turn from a stall.
//
// The file lives under $XDG_STATE_HOME/hergy, next to the launch record: like a
// launch, a screen hergy has seen is a fact about this machine. It holds no
// screen text — only the hash — and one entry per member, keyed by the name
// herdr knows it by.
package screenlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// fileName is the one file of this record.
const fileName = "screens.json"

// Entry is what hergy remembers about one member between status runs: the hash
// of its screen, and the time that screen was first seen (not the time it was
// last confirmed), so the difference from now is how long it has stood.
type Entry struct {
	Hash string    `json:"hash"`
	At   time.Time `json:"at"`
}

// Hash is the hash of one screen, in lowercase hex. It is the one place the hash
// is made, so the launcher that reads the screen and a test that checks it agree
// without repeating the recipe.
func Hash(screen string) string {
	sum := sha256.Sum256([]byte(screen))
	return hex.EncodeToString(sum[:])
}

// Log is one machine's record of member screens.
type Log struct{ dir string }

// DefaultDir is where this machine's record lives: $XDG_STATE_HOME/hergy, or
// ~/.local/state/hergy when XDG_STATE_HOME is unset or not absolute. State is
// not settings, so it is not put under the config directory, and it is not put
// in the project: a screen hergy saw is a fact about this machine, shared by
// every project's teams.
func DefaultDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "hergy"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "hergy"), nil
}

// Open is this machine's record.
func Open() (Log, error) {
	dir, err := DefaultDir()
	if err != nil {
		return Log{}, err
	}
	return At(dir), nil
}

// At is the record in one directory: the tests use it, and so does a caller that
// knows where the record belongs better than this package does.
func At(dir string) Log { return Log{dir: dir} }

// Read returns every entry, keyed by member name. A record that is not there yet
// is an empty map, not an error: no status has been run on this machine.
func (l Log) Read() (map[string]Entry, error) {
	text, err := os.ReadFile(filepath.Join(l.dir, fileName))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries := map[string]Entry{}
	if len(text) == 0 {
		return entries, nil
	}
	if err := json.Unmarshal(text, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// Write replaces the whole record. A temp file and a rename, so a reader sees
// either the old file or the new one, never half of one. encoding/json sorts the
// keys, so the same entries always write the same bytes.
func (l Log) Write(entries map[string]Entry) error {
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	text, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(l.dir, fileName+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(text); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(l.dir, fileName))
}
