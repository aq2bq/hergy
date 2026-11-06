package web

import (
	"fmt"

	"github.com/aq2bq/hergy/internal/definition"
)

// This file holds the team screen's own words (r-0f8c): one edit as a panel
// sends it. The screen draws these; it builds no sentence that a check would say
// (spec-teams X1).

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
	case "description":
		return definition.Description(o.Text), nil
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
	case "addMember":
		return definition.AddMember(o.Member, o.Role, o.Profile), nil
	case "removeMember":
		return definition.RemoveMember(o.Member), nil
	case "renameMember":
		return definition.RenameMember(o.Member, o.New), nil
	case "moveWorker":
		return definition.MoveWorker(o.From, o.Purpose, o.To), nil
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
// member (spec-teams 1.4.1).
func t2Node(name string) definition.NodeID {
	if name == "human" {
		return (definition.Human{}).ID()
	}
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// t2Turn is one turn of the figure: who hands the turn to whom.
type t2Turn struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// t2TurnsOf is the definition's turns: as written, short names.
func t2TurnsOf(def *definition.Definition) []t2Turn {
	if def == nil || def.Graph == nil {
		return nil
	}
	out := make([]t2Turn, 0, len(def.Graph.Turns))
	for _, edge := range def.Graph.Turns {
		out = append(out, t2Turn{From: edge.From.String(), To: edge.To.String()})
	}
	return out
}
