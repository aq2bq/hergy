package definition

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Graph is one definition's nodes and edges: who is in the team, what they may
// hire, and who hands the turn to whom. It is read, never written, by every
// projection (the figure, the activity, the lists, a member's prompt, validate).
type Graph struct {
	Human   Human
	Members []MemberNode
	Workers []WorkerNode
	// Turns is the edges a settings file writes in turns:, in the order it
	// writes them. A worker's two edges are not here: workers: makes them
	// (I12c).
	Turns []Edge
	// Edges is every turn: Turns, then the two edges of each worker.
	Edges []Edge
	// nodes is the identity of every node, for the edges' ends and for the
	// checks; it is built once, here.
	nodes map[NodeID]Node
}

// NewGraph builds a graph of the members, the workers and the turns. The edge
// from a worker back to the member that hired it is not written in a settings
// file; it is made here, because a worker always returns to the one that hired
// it (I12c).
func NewGraph(members []MemberNode, workers []WorkerNode, turns []Edge) *Graph {
	g := &Graph{
		Human:   Human{},
		Members: members,
		Workers: workers,
		Turns:   turns,
		Edges:   append([]Edge(nil), turns...),
		nodes:   map[NodeID]Node{},
	}
	g.nodes[g.Human.ID()] = g.Human
	for _, member := range members {
		g.nodes[member.ID()] = member
	}
	for _, worker := range workers {
		g.nodes[worker.ID()] = worker
		if _, ok := g.nodes[memberID(worker.From)]; !ok {
			// The worker's member is not there: Check reports that one fault,
			// and the two edges it would make are left out, so one fault is one
			// problem.
			continue
		}
		// The member hires the worker, and the worker returns to it.
		g.Edges = append(g.Edges, Edge{From: memberID(worker.From), To: worker.ID()})
		g.Edges = append(g.Edges, Edge{From: worker.ID(), To: memberID(worker.From)})
	}
	return g
}

func memberID(name string) NodeID { return NodeID{Kind: KindMember, Name: name} }

// Node is the node with this identity, and whether the graph holds it.
func (g *Graph) Node(id NodeID) (Node, bool) {
	node, ok := g.nodes[id]
	return node, ok
}

// Member is the member node with this name, and whether the graph holds it.
func (g *Graph) Member(name string) (MemberNode, bool) {
	for _, member := range g.Members {
		if member.Name == name {
			return member, true
		}
	}
	return MemberNode{}, false
}

// Out is the edges that leave a node: who it hands the turn to. A member's
// prompt (段 5) and the figure are made from this.
func (g *Graph) Out(id NodeID) []Edge {
	var out []Edge
	for _, edge := range g.Edges {
		if edge.From == id {
			out = append(out, edge)
		}
	}
	return out
}

// ReachesHuman is whether any node hands the turn to the human (I12b)。
func (g *Graph) ReachesHuman() bool {
	for _, edge := range g.Edges {
		if edge.To == g.Human.ID() {
			return true
		}
	}
	return false
}

// Problem is one thing the graph says that cannot be: where it is, and the
// message to show, which comes from the locale (the key and its arguments).
type Problem struct {
	// Field is the team.yaml field the sentence belongs beside: the key a screen
	// places it by, in the one form check.Finding.Field writes (N2). A problem
	// about a node names the node, and one about a line of turns: names turns:.
	Field string
	Key   string // the locale key of the message
	Args  []any
}

// Message is the problem as a person reads it.
func (p Problem) Message() string { return i18n.Tf(p.Key, p.Args...) }

// GraphError is a definition whose graph cannot hold: the file it was read
// from, and every problem, in the order Check reports them. It is one error, so
// a command that reads the definition stops with all of them, the way a file
// that does not read does (I12a, I12b, d-edd4).
type GraphError struct {
	Path     string // the team.yaml the definition was read from
	Problems []Problem
}

// Error is every problem, one line each, with the file once at the front, the
// way the other read errors name it: a command that takes a team name does not
// know which team.yaml that name means.
func (e *GraphError) Error() string {
	return e.Path + ": " + strings.Join(e.Messages(), "\n")
}

// Messages is the sentence of every problem, one line each, without the file:
// hergy validate writes the file itself (breakLine), and would otherwise write
// it twice.
func (e *GraphError) Messages() []string {
	out := make([]string, len(e.Problems))
	for i, problem := range e.Problems {
		out[i] = problem.Message()
	}
	return out
}

// Check is what the graph itself cannot hold: an edge whose end is not a node,
// an edge to itself, one edge written twice, a worker whose hiring member is not
// one, and the turn invariants I12a (every node hands the turn on) and I12b (at
// least one turn reaches the human).
func (g *Graph) Check() []Problem {
	var problems []Problem
	seen := map[Edge]bool{}
	for _, edge := range g.Edges {
		if _, ok := g.nodes[edge.From]; !ok {
			problems = append(problems, Problem{
				Field: "turns",
				Key:   "error.graph_edge_from",
				Args:  []any{edge.String(), edge.From.String()},
			})
		}
		if _, ok := g.nodes[edge.To]; !ok {
			problems = append(problems, Problem{
				Field: "turns",
				Key:   "error.graph_edge_to",
				Args:  []any{edge.String(), edge.To.String()},
			})
		}
		if edge.From == edge.To {
			problems = append(problems, Problem{Field: "turns", Key: "error.graph_self_loop", Args: []any{edge.String()}})
		}
		if seen[edge] {
			problems = append(problems, Problem{Field: "turns", Key: "error.graph_edge_twice", Args: []any{edge.String()}})
			continue
		}
		seen[edge] = true
	}
	for i, worker := range g.Workers {
		if _, ok := g.nodes[memberID(worker.From)]; !ok {
			problems = append(problems, Problem{
				Field: fmt.Sprintf("workers[%d].from", i),
				Key:   "error.graph_worker_from",
				Args:  []any{worker.From},
			})
		}
	}
	problems = append(problems, g.noTurnProblems()...)
	if !g.ReachesHuman() {
		problems = append(problems, Problem{
			Field: "node:human", Key: "error.graph_no_turn_reaches_human",
			Args: []any{g.exampleTarget(g.Human.ID())},
		})
	}
	return problems
}

// exampleTarget is the name a fix's example writes: the first member, or the
// first member other than the one the sentence is about; when the team has no
// other member, the human. A person reading the sentence can copy the example
// and then write the party they mean.
func (g *Graph) exampleTarget(about NodeID) string {
	for _, member := range g.Members {
		if member.ID() != about {
			return member.Name
		}
	}
	return g.Human.ID().String()
}

// noTurnProblems is I12a: every node hands the turn on. What a node without an
// outgoing turn is told comes from its kind (1 節); a worker is never without
// one, because workers: makes its edge back to the member that hired it.
func (g *Graph) noTurnProblems() []Problem {
	var problems []Problem
	for _, id := range g.identity() {
		kind := nodeKinds[id.Kind]
		if kind.noTurnKey == "" || len(g.Out(id)) > 0 {
			continue
		}
		var args []any
		switch id.Kind {
		case KindHuman:
			args = []any{g.exampleTarget(id)}
		case KindMember:
			args = []any{id.String(), id.String(), id.String(), g.exampleTarget(id)}
		case KindWorker:
			// A worker always has its edge back to the member that hired it.
		}
		problems = append(problems, Problem{Field: "node:" + id.String(), Key: kind.noTurnKey, Args: args})
	}
	return problems
}

// identity is every node of the graph: the human first, then the members in the
// order the file writes them, then the workers.
func (g *Graph) identity() []NodeID {
	out := make([]NodeID, 0, 1+len(g.Members)+len(g.Workers))
	out = append(out, g.Human.ID())
	for _, member := range g.Members {
		out = append(out, member.ID())
	}
	for _, worker := range g.Workers {
		out = append(out, worker.ID())
	}
	return out
}

// String is a node's identity as a settings file writes it: the human, a member
// name, or "<kind>:<name>" for another kind (d-11f8).
func (id NodeID) String() string {
	if nodeKinds[id.Kind].byName {
		return id.Name
	}
	return string(id.Kind) + ":" + id.Name
}

// String is the edge as a settings file writes it ("lead -> human").
func (e Edge) String() string { return e.From.String() + " -> " + e.To.String() }
