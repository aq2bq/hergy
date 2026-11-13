// Package runningstate owns the facts of one running team. Launch history is
// independent: ending a run removes its state, and beginning another replaces it.
package runningstate

import (
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
)

// State is an immutable reading. Its slices are never shared with callers.
type State struct{ data snapshot }

// Generation identifies one Begin. Only the state boundary can construct it.
type Generation struct{ value string }

// Generation is the run this reading belongs to, including a run with no receipts.
func (s State) Generation() Generation { return Generation{value: s.data.Generation} }

// Launch supplies the facts fixed when a team's tab is about to be created.
type Launch struct {
	Definition       domainname.DefinitionName
	Project          domainplace.Project
	Layer            domainplace.Selection
	TeamYAML         domainplace.Project
	Workspace        Workspace
	StartedAt        time.Time
	Executable, Lang string
	Workers          []WorkerDefinition
}

// Workspace is the workspace chosen before the team tab is made.
type Workspace struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// WorkerDefinition keeps the startup definition's worker profile fingerprints.
type WorkerDefinition struct {
	From    string `json:"from"`
	Purpose string `json:"purpose"`
	Profile string `json:"profile"`
	Start   string `json:"start"`
	Env     string `json:"env"`
}

// Doc is the full text of a rule document read for a received prompt.
type Doc struct {
	Path  string `json:"path"`
	Layer string `json:"layer"`
	Text  string `json:"text"`
}

// Receipt is one member's latest confirmed body, before a delivery preamble.
// Executable and Lang belong to that receipt: later operations may use other
// prompt-building inputs than the operation that began the team.
type Receipt struct {
	Name       string    `json:"name"`
	Short      string    `json:"short"`
	Prompt     string    `json:"prompt"`
	Command    string    `json:"command"`
	Time       time.Time `json:"time"`
	Docs       []Doc     `json:"docs"`
	Start      string    `json:"start"`
	Env        string    `json:"env"`
	Executable string    `json:"executable"`
	Lang       string    `json:"lang"`
}

// Worker is a worker hired during this run, independently of prompt receipt.
type Worker struct {
	Name    string    `json:"name"`
	From    string    `json:"from"`
	Purpose string    `json:"purpose"`
	Tab     string    `json:"tab"`
	Time    time.Time `json:"time"`
}

type snapshot struct {
	Generation        string                `json:"generation"`
	Team              string                `json:"team"`
	Definition        string                `json:"definition"`
	Project           string                `json:"project"`
	Layer             domainplace.Selection `json:"layer"`
	TeamYAML          string                `json:"team_yaml"`
	Workspace         Workspace             `json:"workspace"`
	StartedAt         time.Time             `json:"started_at"`
	Executable        string                `json:"executable"`
	Lang              string                `json:"lang"`
	Members           []Receipt             `json:"members"`
	WorkerDefinitions []WorkerDefinition    `json:"worker_definitions"`
	Workers           []Worker              `json:"workers"`
}

// Name is the checked team name identifying this run.
func (s State) Name() domainname.TeamName {
	name, _ := domainname.ParseTeamName(s.data.Team)
	return name
}

// Launch returns the immutable startup facts and a detached worker definition.
func (s State) Launch() Launch {
	def, _ := domainname.ParseDefinitionName(s.data.Definition)
	project, _ := domainplace.NewProject(s.data.Project)
	path, _ := domainplace.NewProject(s.data.TeamYAML)
	return Launch{Definition: def, Project: project, Layer: s.data.Layer, TeamYAML: path,
		Workspace: s.data.Workspace, StartedAt: s.data.StartedAt, Executable: s.data.Executable, Lang: s.data.Lang,
		Workers: append([]WorkerDefinition(nil), s.data.WorkerDefinitions...)}
}

// Members returns detached copies of the latest confirmed receipts.
func (s State) Members() []Receipt {
	out := append([]Receipt(nil), s.data.Members...)
	for i := range out {
		out[i].Docs = append([]Doc(nil), out[i].Docs...)
	}
	return out
}

// Workers returns the workers still belonging to this run.
func (s State) Workers() []Worker { return append([]Worker(nil), s.data.Workers...) }
