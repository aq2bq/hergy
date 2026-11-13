// Package teststate creates explicit run registrations for operation, CLI and
// API tests. It never reconstructs state from launch history or herdr output.
package teststate

import (
	"path/filepath"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
)

// Receipt is a confirmed body supplied explicitly by a test.
type Receipt = runningstate.Receipt

// Doc is the rule text a test says its member received.
type Doc = runningstate.Doc

// WorkerDefinition is one startup worker profile supplied by a test.
type WorkerDefinition = runningstate.WorkerDefinition

// Team describes one run independently of its live herdr fixture.
type Team struct {
	Name, Definition, Project, Layer, TeamYAML string
	WorkspaceID, WorkspaceLabel                string
	StartedAt                                  time.Time
	Executable, Lang                           string
	Members                                    []Receipt
	WorkerDefinitions                          []WorkerDefinition
}

// Write begins a run in the test's XDG_STATE_HOME. Empty Layer means local;
// empty TeamYAML means the named definition in the project's .hergy directory.
func Write(in Team) error {
	name, err := domainname.ParseTeamName(in.Name)
	if err != nil {
		return err
	}
	def, err := domainname.ParseDefinitionName(in.Definition)
	if err != nil {
		return err
	}
	project, err := domainplace.NewProject(in.Project)
	if err != nil {
		return err
	}
	if in.Layer == "" {
		in.Layer = "local"
	}
	if in.TeamYAML == "" {
		in.TeamYAML = filepath.Join(in.Project, ".hergy", "teams", in.Definition, "team.yaml")
	}
	path, err := domainplace.NewProject(in.TeamYAML)
	if err != nil {
		return err
	}
	if in.StartedAt.IsZero() {
		in.StartedAt = time.Unix(1, 0).UTC()
	}
	store, err := runningstate.Open()
	if err != nil {
		return err
	}
	if in.Executable == "" {
		in.Executable = "hergy"
	}
	if in.Lang == "" {
		in.Lang = "en"
	}
	generation, err := store.Begin(name, runningstate.Launch{Definition: def, Project: project, Layer: domainplace.Selection(in.Layer), TeamYAML: path,
		Workspace: runningstate.Workspace{ID: in.WorkspaceID, Label: in.WorkspaceLabel}, StartedAt: in.StartedAt, Executable: in.Executable, Lang: in.Lang, Workers: in.WorkerDefinitions})
	if err != nil {
		return err
	}
	return store.Receive(name, generation, in.Members)
}
