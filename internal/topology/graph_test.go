package topology

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
)

// memberIDOf is one member's identity, for writing a turn in a test.
func memberIDOf(name string) definition.NodeID {
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// sampleGraph is a small team: a lead, a builder that may hire one worker, and
// the turns the human and the members hand around.
func sampleGraph() *definition.Graph {
	return definition.NewGraph(
		[]definition.MemberNode{
			{Name: "lead", Role: "進行管理", Profile: "planner"},
			{Name: "impl", Role: "設計実装", Profile: "builder"},
		},
		[]definition.WorkerNode{{From: "impl", Purpose: "調査", Profile: "researcher"}},
		[]definition.Edge{
			{From: definition.Human{}.ID(), To: memberIDOf("lead")},
			{From: memberIDOf("lead"), To: memberIDOf("impl")},
			{From: memberIDOf("impl"), To: definition.Human{}.ID()},
		},
	)
}

// graphBox is the box one node is drawn in: the human's, a member's, or a
// worker's under the member that hires it.
func graphBox(f Figure, id definition.NodeID) (Box, bool) {
	switch id.Kind {
	case definition.KindHuman:
		return f.Human, true
	case definition.KindMember:
		for _, member := range f.Members {
			if member.Name == id.Name {
				return member.Box, true
			}
		}
	case definition.KindWorker:
		hiring, purpose, _ := strings.Cut(id.Name, "/")
		for _, member := range f.Members {
			if member.Name != hiring {
				continue
			}
			for _, worker := range member.Workers {
				if worker.Purpose == purpose {
					return worker.Box, true
				}
			}
		}
	}
	return Box{}, false
}

// TestLayoutGraphDrawsEveryNodeAndEdge counts what the figure holds: every node
// of the graph is a box and every edge is an arrow (C7, 段 4).
func TestLayoutGraphDrawsEveryNodeAndEdge(t *testing.T) {
	g := sampleGraph()
	f := LayoutGraph(g)

	if len(f.Members) != len(g.Members) {
		t.Fatalf("members = %d, want %d", len(f.Members), len(g.Members))
	}
	if len(f.Edges) != len(g.Edges) {
		t.Fatalf("arrows = %d, want one per edge (%d)", len(f.Edges), len(g.Edges))
	}
	workers := 0
	for _, member := range f.Members {
		workers += len(member.Workers)
	}
	if workers != len(g.Workers) {
		t.Fatalf("worker boxes = %d, want %d", workers, len(g.Workers))
	}
	if f.Human.H == 0 {
		t.Fatal("the human is not drawn")
	}
	for i, edge := range g.Edges {
		if f.Edges[i].FromNode != edge.From || f.Edges[i].ToNode != edge.To {
			t.Fatalf("arrow %d is %v -> %v, want %v -> %v", i,
				f.Edges[i].FromNode, f.Edges[i].ToNode, edge.From, edge.To)
		}
	}
}

// TestLayoutGraphDrawsNothingButTheGraph holds the figure to the graph: the
// boxes of the rule documents and the band every member reads are not drawn
// (C7, 段 4).
func TestLayoutGraphDrawsNothingButTheGraph(t *testing.T) {
	f := LayoutGraph(sampleGraph())
	if len(f.Docs) != 0 {
		t.Fatalf("document boxes = %d, want none", len(f.Docs))
	}
	if (f.Shared != Box{}) {
		t.Fatalf("the shared band is drawn: %+v", f.Shared)
	}
	for _, member := range f.Members {
		if member.RoleFile != nil {
			t.Fatalf("member %s has a role file box: %+v", member.Name, *member.RoleFile)
		}
	}
}

// TestLayoutGraphNamesTheMemberAndTheWorker checks what each box carries: a
// member's role and profile, a worker's purpose and profile (段 4).
func TestLayoutGraphNamesTheMemberAndTheWorker(t *testing.T) {
	f := LayoutGraph(sampleGraph())
	lead := f.Members[0]
	if lead.Name != "lead" || lead.Role != "進行管理" || lead.Profile != "planner" {
		t.Fatalf("lead box = %+v", lead)
	}
	impl := f.Members[1]
	if len(impl.Workers) != 1 {
		t.Fatalf("impl's workers = %d, want 1", len(impl.Workers))
	}
	if impl.Workers[0].Purpose != "調査" || impl.Workers[0].Profile != "researcher" {
		t.Fatalf("worker box = %+v", impl.Workers[0])
	}
	if impl.Workers[0].Y <= impl.Y+impl.H {
		t.Fatal("the worker is not drawn under the member that hires it")
	}
}

// TestLayoutGraphArrowsMeetTheirBoxes checks where each arrow lands: inside the
// box of the node it comes from and of the node it goes to.
func TestLayoutGraphArrowsMeetTheirBoxes(t *testing.T) {
	g := sampleGraph()
	f := LayoutGraph(g)
	for i, arrow := range f.Edges {
		from, ok := graphBox(f, arrow.FromNode)
		if !ok {
			t.Fatalf("arrow %d: no box for %v", i, arrow.FromNode)
		}
		to, ok := graphBox(f, arrow.ToNode)
		if !ok {
			t.Fatalf("arrow %d: no box for %v", i, arrow.ToNode)
		}
		if arrow.FromY < from.Y || arrow.FromY > from.Y+from.H {
			t.Fatalf("arrow %d leaves at y=%v, outside %+v", i, arrow.FromY, from)
		}
		if arrow.ToY < to.Y || arrow.ToY > to.Y+to.H {
			t.Fatalf("arrow %d lands at y=%v, outside %+v", i, arrow.ToY, to)
		}
	}
}

// TestLayoutGraphWorkerEdgesAreADottedPair checks the two edges a worker makes:
// dotted, side by side between the member's box and the worker's (I12c).
func TestLayoutGraphWorkerEdgesAreADottedPair(t *testing.T) {
	g := sampleGraph()
	f := LayoutGraph(g)
	var pair []Arrow
	for _, arrow := range f.Edges {
		if arrow.FromNode.Kind == definition.KindWorker || arrow.ToNode.Kind == definition.KindWorker {
			if !arrow.Worker {
				t.Fatalf("worker edge %v -> %v is not drawn dotted", arrow.FromNode, arrow.ToNode)
			}
			pair = append(pair, arrow)
		} else if arrow.Worker {
			t.Fatalf("turn %v -> %v is drawn dotted", arrow.FromNode, arrow.ToNode)
		}
	}
	if len(pair) != 2*len(g.Workers) {
		t.Fatalf("dotted arrows = %d, want two per worker (%d)", len(pair), 2*len(g.Workers))
	}
	impl := f.Members[1]
	if got := pair[1].LaneX - pair[0].LaneX; got != workerPairGap {
		t.Fatalf("the pair is %v apart, want %v", got, workerPairGap)
	}
	for _, arrow := range pair {
		if arrow.LaneX <= impl.X || arrow.LaneX >= impl.X+impl.W {
			t.Fatalf("the pair is drawn at x=%v, outside the member's box %+v", arrow.LaneX, impl.Box)
		}
	}
}

// TestLayoutGraphFitsItsBoxesInside checks the figure's size holds everything it
// draws, and that the lanes it leaves for the turns are outside the members.
func TestLayoutGraphFitsItsBoxesInside(t *testing.T) {
	g := sampleGraph()
	f := LayoutGraph(g)
	if f.Width <= f.MemberRight {
		t.Fatalf("width = %v, not wider than the member column (%v)", f.Width, f.MemberRight)
	}
	for _, member := range f.Members {
		if member.X < 0 || member.X+member.W > f.Width {
			t.Fatalf("member %s does not fit: %+v", member.Name, member.Box)
		}
		if member.Y+member.H > f.Height {
			t.Fatalf("member %s does not fit: %+v", member.Name, member.Box)
		}
		for _, worker := range member.Workers {
			if worker.Y+worker.H > f.Height {
				t.Fatalf("worker %s/%s does not fit: %+v", member.Name, worker.Purpose, worker.Box)
			}
		}
	}
	for _, arrow := range f.Edges {
		if arrow.Worker {
			continue
		}
		if arrow.LaneX <= f.MemberRight {
			t.Fatalf("a turn bends at x=%v, inside the members (%v)", arrow.LaneX, f.MemberRight)
		}
	}
}

// TestLayoutGraphDeterministic checks the same graph is laid out in the same
// places twice: the figure is a projection, not a drawing that wanders.
func TestLayoutGraphDeterministic(t *testing.T) {
	g := sampleGraph()
	a, b := LayoutGraph(g), LayoutGraph(g)
	if a.Width != b.Width || a.Height != b.Height || a.Human != b.Human {
		t.Fatalf("figure sizes differ: %+v vs %+v", a, b)
	}
	if len(a.Edges) != len(b.Edges) || len(a.Members) != len(b.Members) {
		t.Fatal("figure lengths differ")
	}
	for i := range a.Edges {
		if a.Edges[i] != b.Edges[i] {
			t.Fatalf("arrow %d differs: %+v vs %+v", i, a.Edges[i], b.Edges[i])
		}
	}
	for i := range a.Members {
		if a.Members[i].Name != b.Members[i].Name || a.Members[i].Box != b.Members[i].Box {
			t.Fatalf("member %d differs", i)
		}
	}
}

// TestLayoutGraphWithoutMembers draws the human alone: a graph with no members
// is still a figure, and there are no arrows.
func TestLayoutGraphWithoutMembers(t *testing.T) {
	f := LayoutGraph(definition.NewGraph(nil, nil, nil))
	if len(f.Members) != 0 || len(f.Edges) != 0 {
		t.Fatalf("an empty graph drew %d members and %d arrows", len(f.Members), len(f.Edges))
	}
	if f.Human.H == 0 || f.Height < f.Human.H {
		t.Fatalf("the human is not drawn: %+v in a figure of height %v", f.Human, f.Height)
	}
}

// TestWorkerArrowWithoutABox leaves the arrow unplaced when the figure holds no
// box for the worker: the edge is named and no line is drawn.
func TestWorkerArrowWithoutABox(t *testing.T) {
	edge := definition.Edge{
		From: memberIDOf("impl"),
		To:   definition.NodeID{Kind: definition.KindWorker, Name: "impl/調査"},
	}
	arrow := workerArrow(edge, nil)
	if !arrow.Worker || arrow.FromNode != edge.From || arrow.ToNode != edge.To {
		t.Fatalf("arrow = %+v, want the edge named and dotted", arrow)
	}
	if arrow.FromY != 0 || arrow.ToY != 0 || arrow.LaneX != 0 {
		t.Fatalf("arrow has a place to meet: %+v", arrow)
	}
}
