package check

import (
	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/source"
)

// TeamInput is what one team definition's check needs.
type TeamInput struct {
	Catalog *catalog.Catalog // reads the two layers
	Name    string           // the definition name (the teams/<name>/ directory)
	Layer   string           // source.LayerLocal or source.LayerGlobal
	Ops     []definition.Op  // applied to the saved team.yaml, in order, before checking
}

// TeamNode is one node of the figure: the human or a member.
type TeamNode struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`           // "human" or "member"
	Role string `json:"role,omitempty"` // the member's role; "" for the human
}

// TeamWorker is one worker of the figure.
type TeamWorker struct {
	From    string `json:"from"`
	Purpose string `json:"purpose"`
	Profile string `json:"profile"`
}

// TeamGraph is the figure's data (spec 1.4): the human and the members as nodes,
// the workers, the turns of the draft, and the turns of the saved file.
type TeamGraph struct {
	Nodes      []TeamNode   `json:"nodes"`
	Workers    []TeamWorker `json:"workers"`
	Turns      [][2]string  `json:"turns"`
	SavedTurns [][2]string  `json:"savedTurns"`
}

// TeamCounts are the numbers a list row shows.
type TeamCounts struct {
	Members int `json:"members"`
	Workers int `json:"workers"`
	Turns   int `json:"turns"`
}

// TeamReport is what one team definition's check found (C2).
type TeamReport struct {
	Problems []Finding  `json:"problems"`
	Graph    TeamGraph  `json:"graph"`
	Counts   TeamCounts `json:"counts"`
}

// Team checks one team definition's team.yaml (C2): it applies Ops to the saved
// text, lays it over the catalog with catalog.With, and calls check.Definition.
//
// The findings are the ones hergy validate prints, so a screen shows no sentence
// of its own (X1, V2). The figure's data comes from the definition read through
// the overlaid text; the saved turns come from the file on disk, before the ops,
// so a screen can draw what the draft took away. An op that names something the
// file does not hold is a finding that stops, and the figure is built from as
// far as the ops got; a definition that does not read still answers with the
// findings and the figure as far as it read.
func Team(in TeamInput) TeamReport {
	cat := in.Catalog
	if cat == nil {
		return TeamReport{}
	}
	path := cat.TeamYAML(in.Name, in.Layer)

	overlay := cat
	var opErr error
	if saved, err := cat.Reader().Read(source.Doc{Path: path}); err == nil {
		var applied string
		applied, opErr = applyTeamOps(cat, path, saved, in.Ops)
		overlay = cat.With(path, applied)
	}

	draft, _ := overlay.Definition(in.Name, in.Layer)
	saved, _ := cat.Definition(in.Name, in.Layer)

	report := TeamReport{
		Problems: Definition(Input{Catalog: overlay, Name: in.Name, Layer: in.Layer}),
		Graph:    teamGraph(draft, saved),
		Counts:   teamCounts(draft),
	}
	if opErr != nil {
		report.Problems = []Finding{{Stops: true, Text: opErr.Error()}}
	}
	return report
}

// applyTeamOps applies every op to the saved text in order, so the check reads
// what a screen's draft stands for. It stops at the first op the file cannot
// hold, and answers the text as far as it got.
func applyTeamOps(cat *catalog.Catalog, path, text string, ops []definition.Op) (string, error) {
	display := cat.Display(path)
	for _, op := range ops {
		out, err := definition.EditYAML([]byte(text), display, op)
		if err != nil {
			return text, err
		}
		text = string(out)
	}
	return text, nil
}

// teamGraph is the figure's data: the draft (the ops applied), and the turns of
// the saved file (the ops not applied), so a screen tells a turn the draft added
// from one it took away.
func teamGraph(draft, saved *definition.Definition) TeamGraph {
	var g TeamGraph
	if draft != nil {
		g.Nodes = append(g.Nodes, TeamNode{ID: "human", Kind: "human"})
		for _, member := range draft.Graph.Members {
			g.Nodes = append(g.Nodes, TeamNode{ID: member.Name, Kind: "member", Role: member.Role})
		}
		for _, worker := range draft.Graph.Workers {
			g.Workers = append(g.Workers, TeamWorker{From: worker.From, Purpose: worker.Purpose, Profile: worker.Profile})
		}
		g.Turns = teamTurnPairs(draft)
	}
	if saved != nil {
		g.SavedTurns = teamTurnPairs(saved)
	}
	return g
}

// teamTurnPairs is one definition's turns: the ends as the short names a settings
// file writes ("human" for the human), which is what a screen draws them by.
func teamTurnPairs(def *definition.Definition) [][2]string {
	out := make([][2]string, 0, len(def.Graph.Turns))
	for _, edge := range def.Graph.Turns {
		out = append(out, [2]string{edge.From.String(), edge.To.String()})
	}
	return out
}

// teamCounts is the numbers a list row shows, from the definition's figure.
func teamCounts(def *definition.Definition) TeamCounts {
	if def == nil {
		return TeamCounts{}
	}
	return TeamCounts{
		Members: len(def.Graph.Members),
		Workers: len(def.Graph.Workers),
		Turns:   len(def.Graph.Turns),
	}
}
