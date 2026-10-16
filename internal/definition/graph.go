package definition

// Graph is one definition's nodes and edges: who is in the team, what they may
// hire, and who hands the turn to whom. It is read, never written, by every
// projection (the figure, the activity, the lists, a member's prompt, validate).
type Graph struct {
	Human   Human
	Members []MemberNode
	Workers []WorkerNode
	Edges   []Edge
	// nodes is the identity of every node, for the edges' ends and for the
	// checks; it is built once, here.
	nodes map[NodeID]Node
}

// NewGraph builds a graph of the members and the workers. The edge from a worker
// back to the member that hired it is not written in a settings file; it is made
// here, because a worker always returns to the one that hired it (I12c).
func NewGraph(members []MemberNode, workers []WorkerNode) *Graph {
	g := &Graph{
		Human:   Human{},
		Members: members,
		Workers: workers,
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
		g.addEdge(memberID(worker.From), worker.ID())
		g.addEdge(worker.ID(), memberID(worker.From))
	}
	return g
}

func memberID(name string) NodeID { return NodeID{Kind: KindMember, Name: name} }

// AddEdge writes one turn from the settings file (段 5 reads `turns:`). It is how
// the human and the member-to-member edges get in; an edge to a node that is not
// there is kept, and Check reports it.
func (g *Graph) AddEdge(from, to NodeID) {
	g.addEdge(from, to)
}

func (g *Graph) addEdge(from, to NodeID) {
	g.Edges = append(g.Edges, Edge{From: from, To: to})
}

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
	At   string // where, as a person reads it ("workers: 実装 (from: lead)")
	Key  string // the locale key of the message
	Args []any
}

// Check is what the graph itself cannot hold: an edge whose end is not a node,
// and a worker whose hiring member is not one. The turn invariants (I12a, I12b)
// arrive with `turns:` in 段 5.
func (g *Graph) Check() []Problem {
	var problems []Problem
	seen := map[Edge]bool{}
	for _, edge := range g.Edges {
		if _, ok := g.nodes[edge.From]; !ok {
			problems = append(problems, Problem{
				At:   edge.String(),
				Key:  "error.graph_edge_from",
				Args: []any{edge.String(), edge.From.String()},
			})
		}
		if _, ok := g.nodes[edge.To]; !ok {
			problems = append(problems, Problem{
				At:   edge.String(),
				Key:  "error.graph_edge_to",
				Args: []any{edge.String(), edge.To.String()},
			})
		}
		if seen[edge] {
			problems = append(problems, Problem{At: edge.String(), Key: "error.graph_edge_twice", Args: []any{edge.String()}})
			continue
		}
		seen[edge] = true
	}
	for _, worker := range g.Workers {
		if _, ok := g.nodes[memberID(worker.From)]; !ok {
			problems = append(problems, Problem{
				At:   worker.ID().String(),
				Key:  "error.graph_worker_from",
				Args: []any{worker.From},
			})
		}
	}
	return problems
}

// String is a node's identity as a settings file writes it: the human, a member
// name, or "<kind>:<name>" for another kind (d-11f8).
func (id NodeID) String() string {
	if id.Kind == KindHuman || id.Kind == KindMember {
		return id.Name
	}
	return string(id.Kind) + ":" + id.Name
}

// String is the edge as a settings file writes it ("lead -> human").
func (e Edge) String() string { return e.From.String() + " -> " + e.To.String() }
