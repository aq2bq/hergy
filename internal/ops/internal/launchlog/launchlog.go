// Package launchlog stores append-only launch history. It records the project,
// team, definition path, time and workspace; current run state lives elsewhere.
package launchlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
)

const (
	// logName is the one line per launch, appended and never rewritten.
	logName = "log.jsonl"
)

// Command is the operation that wrote a launch history entry.
const (
	CommandUp       = "up"
	CommandRefresh  = "refresh"
	CommandReprompt = "reprompt"
	CommandApply    = "apply"
)

// Record is the identity and workspace of a past launch. Current members,
// prompts and workers belong to runningstate rather than launch history.
type Record struct {
	Time        string           `json:"time"`
	Command     string           `json:"command"`
	Project     domainplace.Seen `json:"project"`
	Team        domainname.Seen  `json:"team"`
	TeamYAML    domainplace.Seen `json:"team_yaml"`
	Workspace   string           `json:"workspace,omitempty"`
	WorkspaceID string           `json:"workspace_id,omitempty"`
}

// StartedInWorkspace reports a team start with both workspace identifiers.
// Later prompt operations create no tab and cannot replace its workspace.
func (r Record) StartedInWorkspace() bool {
	return r.Command == CommandUp && r.Workspace != "" && r.WorkspaceID != ""
}

// Workspace is one herdr workspace as a launch names it: the label a person
// reads, and the id it had when the team was started there.
type Workspace struct {
	Label string
	ID    string
}

// Log is one machine's record of launches.
type Log struct{ dir string }

// DefaultDir is where this machine's record lives: $XDG_STATE_HOME/hergy/launches,
// or ~/.local/state/hergy/launches when XDG_STATE_HOME is unset or not absolute.
// State is not settings, so it is not put under the config directory, and it is
// not put in the project: a launch is a fact about this machine, and the global
// layer's team definitions have no project of their own.
func DefaultDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "hergy", "launches"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "hergy", "launches"), nil
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

// Dir is where this record's files go.
func (l Log) Dir() string { return l.dir }

// Append adds one launch with one write. Existing lines and legacy blobs are
// left untouched: this file is history.
func (l Log) Append(rec Record) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(l.dir, logName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Read returns every launch, oldest first, ignoring legacy hire entries.
// A record that is not there yet is an empty list, not an error: nothing has
// been launched on this machine.
func (l Log) Read() ([]Record, error) {
	text, err := os.ReadFile(filepath.Join(l.dir, logName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Record
	for i, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		if line == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", logName, i+1, err)
		}
		// Old versions wrote hires into this file. They are not launch history,
		// even when their team, definition or workspace fields are populated.
		if rec.Command == "hire" {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

// Latest is the newest workspace-bearing up of a definition, and whether one
// exists. A team running under another name (--as) belongs to the definition it
// was launched from.
//
// The definition is named by the team.yaml its launch read: this machine's record
// holds every project's launches, and two projects can hold a definition of the
// same name, so the layer and the name alone would hand back another project's
// launch (M1).
func (l Log) Latest(teamYAML domainplace.Project) (Record, bool, error) {
	records, err := l.Read()
	if err != nil {
		return Record{}, false, err
	}
	for _, record := range slices.Backward(records) {
		if !record.StartedInWorkspace() {
			continue
		}
		if record.TeamYAML.Project().Compare(teamYAML) == domainplace.Same || (record.TeamYAML.Project().IsZero() && teamYAML.IsZero()) {
			return record, true, nil
		}
	}
	return Record{}, false, nil
}
