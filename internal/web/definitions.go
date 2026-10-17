package web

import (
	"net/http"
	"path/filepath"

	"github.com/aq2bq/hergy/internal/definition"
)

// definitions is the list of the team definitions (設計 3.3). It is also what
// "/" answers (W1), so opening hergy serve lands on the list of what can be
// started.
func (s *Server) definitions(_ *http.Request) (view, error) {
	running, _ := s.runningTeams()
	byDefinition := runningByDefinition(running)
	entries := s.entries()
	data := definitionsData{Crumb: []crumb{{Name: t("web.band.definitions")}}}
	data.Shared = definitionRow{Name: t("web.common.title"), Href: "/agreement"}
	if shared, ok := s.cat.SharedAgreement(); ok {
		data.Shared.Count = s.ruleCount(shared.Path)
	} else {
		// The row and the page agree: there is no shared agreement (N8).
		data.Shared.Empty = true
	}
	for _, entry := range entries {
		row := definitionRow{Name: entry.Name, Href: definitionPath(entry.Name)}
		if entry.Outside {
			row.Path = s.cat.Display(filepath.Dir(filepath.Dir(entry.Path)))
		}
		if entry.UnusedPath != "" {
			row.Unused = s.cat.Display(entry.UnusedPath)
		}
		row.Running = runningLinks(byDefinition[entry.Name])
		if entry.Err != nil {
			row.Broken = true
			row.Mission = []string{s.breakLine(readErrorPath(entry.Err, entry.Path), entry.Err)}
			data.Entries = append(data.Entries, row)
			continue
		}
		def, err := s.readDefinition(entry.Name)
		if err != nil {
			row.Broken = true
			row.Mission = []string{s.breakLine(readErrorPath(err, entry.Path), err)}
			data.Entries = append(data.Entries, row)
			continue
		}
		row.Mission = missionLines(def.Mission)
		row.Members = len(def.Graph.Members)
		row.Workers = len(def.Graph.Workers)
		row.Human = turnsToHuman(def)
		data.Entries = append(data.Entries, row)
	}
	data.Empty = len(data.Entries) == 0
	data.NewCommand = t("web.definitions.new")
	return s.pageBody(view{
		title: t("web.definitions.title"),
		band:  bandDefinitions,
		crumb: data.Crumb,
	}, "definitions.html", data)
}

// missionLines is the mission as the lines the list shows: its first line.
func missionLines(mission string) []string {
	line, _, _ := cutFirstLine(mission)
	if line == "" {
		return nil
	}
	return []string{line}
}

// turnsToHuman is how many turns of a definition reach the human ("人間へ 1").
func turnsToHuman(def *definition.Definition) int {
	return len(def.Graph.Out((definition.Human{}).ID()))
}

// definitionsData is what the list template is given.
type definitionsData struct {
	Crumb      []crumb
	Shared     definitionRow
	Entries    []definitionRow
	Empty      bool
	NewCommand string
}

// definitionRow is one row of the list: one team definition, or the shared
// agreement above them.
type definitionRow struct {
	Name    string
	Href    string
	Mission []string
	Count   int
	Path    string
	Unused  string
	Members int
	Workers int
	Human   int
	Broken  bool
	Empty   bool
	Running []turnEnd
}
