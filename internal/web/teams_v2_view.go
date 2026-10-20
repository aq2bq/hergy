package web

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/prompt"
)

// This file is the team-definition screen's data (r-0f8c): the island the page
// is drawn from, the findings mapped to where they are shown, and the ops a
// panel sends. The checks themselves are check.Team and check.Agreement: this
// file builds no sentence of its own (spec-teams P1, X1).

// t2PageData is one tab's page.
type t2PageData struct {
	Lang  string
	Nav   navData
	Tab   string
	Token string
	Data  template.JS
}

// t2Island is everything the screen draws from.
type t2Island struct {
	Project  string            `json:"project"`
	Places   map[string]string `json:"places"`
	Dirs     map[string]string `json:"dirs"`
	Texts    map[string]string `json:"texts"`
	Profiles []string          `json:"profiles"`
	Defs     []t2Def           `json:"defs"`
	Files    []t2File          `json:"files"`
}

// t2Def is one team definition as the list and the two tabs show it.
type t2Def struct {
	Name          string           `json:"name"`
	Place         string           `json:"place"`
	Places        []string         `json:"places"`
	TeamPath      string           `json:"teamPath"`
	AgreePath     string           `json:"agreePath"`
	Hash          string           `json:"hash"`
	AgreeHash     string           `json:"agreeHash"`
	HasAgree      bool             `json:"hasAgree"`
	Broken        bool             `json:"broken"`
	Mission       string           `json:"mission"`
	Team          t2Team           `json:"team"`
	SavedTurns    [][2]string      `json:"savedTurns"`
	Agree         *r2Doc           `json:"agree"`
	Hidden        []t2Hidden       `json:"hidden"`
	Rules         int              `json:"rules"`
	Excluded      int              `json:"excluded"`
	Findings      []t2Finding      `json:"findings"`
	TurnsFindings []t2Finding      `json:"turnsFindings"`
	AgreeReport   *check.DocReport `json:"agreeReport"`
	Prompt        t2Prompt         `json:"prompt"`
}

// t2Team is the saved shape the screen edits: the same fields a team.yaml
// writes, applied through ops.
type t2Team struct {
	Mission string      `json:"mission"`
	Members []t2Member  `json:"members"`
	Workers []t2Worker  `json:"workers"`
	Turns   [][2]string `json:"turns"`
}

type t2Member struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Profile string `json:"profile"`
}

type t2Worker struct {
	From    string `json:"from"`
	Purpose string `json:"purpose"`
	Profile string `json:"profile"`
}

// t2Hidden is one layer whose same name the effective one hides (spec 1.4.5).
type t2Hidden struct {
	Place string `json:"place"`
	Path  string `json:"path"`
	Hash  string `json:"hash"`
}

// t2Finding is one check finding where the screen puts it. Node is the human or
// a member's short name, Worker a 0-based worker index or -1. Field is check's
// field string, which the screen also reads to route a finding (spec 1.7).
type t2Finding struct {
	Sev    string `json:"sev"`
	Field  string `json:"field"`
	Node   string `json:"node"`
	Worker int    `json:"worker"`
	Text   string `json:"text"`
}

// t2Prompt is the prompt's own lines for what a node hands to whom, and what a
// member may hire. The screen never builds these strings (spec X3).
type t2Prompt struct {
	Turns   map[string][]string `json:"turns"`
	Workers map[string][]string `json:"workers"`
}

// t2File is one file the YAML tab can open.
type t2File struct {
	File     string          `json:"file"`
	Place    string          `json:"place"`
	Name     string          `json:"name"`
	Kind     string          `json:"kind"`
	Path     string          `json:"path"`
	Hash     string          `json:"hash"`
	Text     string          `json:"text"`
	Hidden   bool            `json:"hidden"`
	Broken   bool            `json:"broken"`
	Problems []t2YamlProblem `json:"problems"`
}

// t2YamlProblem is one problem the YAML tab shows. Line is 0 for a team.yaml
// finding, which carries a field instead.
type t2YamlProblem struct {
	Sev   string `json:"sev"`
	Field string `json:"field,omitempty"`
	Line  int    `json:"line"`
	Text  string `json:"text"`
}

// t2Op is one edit a panel sends. It is the JSON shape of a definition.Op.
type t2Op struct {
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Member  string `json:"member"`
	Role    string `json:"role"`
	Profile string `json:"profile"`
	From    string `json:"from"`
	To      string `json:"to"`
	Purpose string `json:"purpose"`
	Old     string `json:"old"`
	New     string `json:"new"`
}

// op is the op in the model's words.
func (o t2Op) op() (definition.Op, error) {
	switch o.Kind {
	case "mission":
		return definition.Mission(o.Text), nil
	case "addTurn":
		return definition.AddTurn(t2Node(o.From), t2Node(o.To)), nil
	case "removeTurn":
		return definition.RemoveTurn(t2Node(o.From), t2Node(o.To)), nil
	case "addWorker":
		return definition.AddWorker(o.From, o.Purpose, o.Profile), nil
	case "removeWorker":
		return definition.RemoveWorker(o.From, o.Purpose), nil
	case "workerProfile":
		return definition.WorkerProfile(o.From, o.Purpose, o.Profile), nil
	case "workerPurpose":
		return definition.WorkerPurpose(o.From, o.Old, o.New), nil
	case "memberRole":
		return definition.MemberRole(o.Member, o.Role), nil
	case "memberProfile":
		return definition.MemberProfile(o.Member, o.Profile), nil
	}
	return definition.Op{}, fmt.Errorf("unknown op %q", o.Kind)
}

// t2Ops is a panel's ops in the model's words.
func t2Ops(in []t2Op) ([]definition.Op, error) {
	out := make([]definition.Op, 0, len(in))
	for _, o := range in {
		op, err := o.op()
		if err != nil {
			return nil, err
		}
		out = append(out, op)
	}
	return out, nil
}

// t2Node is a name as a node identity: "human" is the human, anything else a
// member (spec 1.4.1).
func t2Node(name string) definition.NodeID {
	if name == "human" {
		return (definition.Human{}).ID()
	}
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// t2TeamOf is one definition's saved shape.
func t2TeamOf(def *definition.Definition) t2Team {
	out := t2Team{}
	if def == nil || def.Graph == nil {
		return out
	}
	out.Mission = def.Mission
	for _, member := range def.Graph.Members {
		out.Members = append(out.Members, t2Member{Name: member.Name, Role: member.Role, Profile: member.Profile})
	}
	for _, worker := range def.Graph.Workers {
		out.Workers = append(out.Workers, t2Worker{From: worker.From, Purpose: worker.Purpose, Profile: worker.Profile})
	}
	out.Turns = t2TurnsOf(def)
	return out
}

// t2TurnsOf is the definition's turns: as written, short names.
func t2TurnsOf(def *definition.Definition) [][2]string {
	if def == nil || def.Graph == nil {
		return nil
	}
	out := make([][2]string, 0, len(def.Graph.Turns))
	for _, edge := range def.Graph.Turns {
		out = append(out, [2]string{edge.From.String(), edge.To.String()})
	}
	return out
}

// t2PromptOf is the prompt's own lines for the figure's cards.
func t2PromptOf(def *definition.Definition, team string) t2Prompt {
	out := t2Prompt{Turns: map[string][]string{}, Workers: map[string][]string{}}
	if def == nil || def.Graph == nil {
		return out
	}
	for _, edge := range def.Graph.Out((definition.Human{}).ID()) {
		if edge.To.Kind == definition.KindMember {
			out.Turns["human"] = append(out.Turns["human"], edge.To.Name)
		}
	}
	for _, member := range def.Graph.Members {
		for _, turn := range prompt.TurnsOf(def, member.Name, team) {
			if turn.Name == "" {
				out.Turns[member.Name] = append(out.Turns[member.Name], tf("turns.human_item"))
				continue
			}
			out.Turns[member.Name] = append(out.Turns[member.Name], tf("turns.member_item", turn.Name, turn.Role))
		}
		for _, worker := range prompt.WorkersOf(def, member.Name) {
			out.Workers[member.Name] = append(out.Workers[member.Name], tf("workers.allowed_item", worker.Purpose, worker.Profile))
		}
	}
	return out
}

// t2Findings maps check's findings to where the screen shows them: the figure
// column (findings), and the team's own agreement rules (turnsFindings), which
// the agreement panel shows from its own check (spec 1.4.5, 1.7).
func t2Findings(graph check.TeamGraph, problems []check.Finding, agreePath string) (findings, turnsFindings []t2Finding) {
	for _, problem := range problems {
		row := t2Finding{Sev: "warn", Field: problem.Field, Worker: -1, Text: problem.Text}
		if problem.Stops {
			row.Sev = "err"
		}
		switch {
		case strings.HasPrefix(problem.Field, "node:"):
			row.Node = strings.TrimPrefix(problem.Field, "node:")
		case strings.HasPrefix(problem.Field, "members["):
			if i := t2Index(problem.Field); i >= 0 && i+1 < len(graph.Nodes) {
				row.Node = graph.Nodes[i+1].ID
			}
		case strings.HasPrefix(problem.Field, "workers["):
			row.Worker = t2Index(problem.Field)
		case strings.HasPrefix(problem.Field, "rule:") && agreePath != "" && strings.HasPrefix(problem.Field, "rule:"+agreePath+"#"):
			turnsFindings = append(turnsFindings, row)
			continue
		}
		findings = append(findings, row)
	}
	return findings, turnsFindings
}

// t2Index is the number in a field like members[2].role, or -1.
func t2Index(field string) int {
	open := strings.IndexByte(field, '[')
	closeIdx := strings.IndexByte(field, ']')
	if open < 0 || closeIdx <= open {
		return -1
	}
	n, err := strconv.Atoi(field[open+1 : closeIdx])
	if err != nil {
		return -1
	}
	return n
}
