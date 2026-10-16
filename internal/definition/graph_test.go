package definition

import (
	"reflect"
	"testing"
)

// TestNewGraphMakesTheWorkerEdges: a settings file writes a member's workers and
// nothing else; the turn from the worker back to the member that hired it is made
// here, because it is always there (I12c).
func TestNewGraphMakesTheWorkerEdges(t *testing.T) {
	g := NewGraph(
		[]MemberNode{{Name: "lead", Role: "lead", Profile: "claude-main"}, {Name: "impl", Role: "impl", Profile: "claude-main"}},
		[]WorkerNode{{From: "lead", Purpose: "調査", Profile: "seeker"}},
	)
	want := []Edge{
		{From: memberID("lead"), To: workerID("lead", "調査")},
		{From: workerID("lead", "調査"), To: memberID("lead")},
	}
	if !reflect.DeepEqual(g.Edges, want) {
		t.Errorf("edges = %v, want %v", g.Edges, want)
	}
	if out := g.Out(memberID("lead")); len(out) != 1 || out[0].To != workerID("lead", "調査") {
		t.Errorf("Out(lead) = %v", out)
	}
	if out := g.Out(workerID("lead", "調査")); len(out) != 1 || out[0].To != memberID("lead") {
		t.Errorf("Out(worker) = %v", out)
	}
	if g.ReachesHuman() {
		t.Error("ReachesHuman = true: 段 3 writes no turn to the human")
	}
}

func TestGraphFindsItsNodes(t *testing.T) {
	g := NewGraph([]MemberNode{{Name: "lead", Role: "lead"}}, nil)
	if member, ok := g.Member("lead"); !ok || member.Role != "lead" {
		t.Errorf("Member(lead) = %+v, %v", member, ok)
	}
	if _, ok := g.Member("nobody"); ok {
		t.Error("Member(nobody) found a member")
	}
	if _, ok := g.Node(g.Human.ID()); !ok {
		t.Error("the human is not a node of the graph")
	}
	if _, ok := g.Node(workerID("lead", "調査")); ok {
		t.Error("a worker that is not in the graph was found")
	}
}

func TestGraphCheck(t *testing.T) {
	t.Run("a sound graph has no problem", func(t *testing.T) {
		g := NewGraph([]MemberNode{{Name: "lead"}}, []WorkerNode{{From: "lead", Purpose: "調査", Profile: "seeker"}})
		if problems := g.Check(); len(problems) != 0 {
			t.Errorf("Check = %+v, want nothing", problems)
		}
	})
	t.Run("a worker whose member is not there", func(t *testing.T) {
		g := NewGraph(nil, []WorkerNode{{From: "lead", Purpose: "調査"}})
		problems := g.Check()
		if len(problems) != 1 || problems[0].Key != "error.graph_worker_from" {
			t.Fatalf("Check = %+v, want the worker's from", problems)
		}
		if got := problems[0].Args; len(got) != 1 || got[0] != "lead" {
			t.Errorf("the problem names %v, want lead", got)
		}
	})
	t.Run("a turn whose ends are not nodes", func(t *testing.T) {
		g := NewGraph([]MemberNode{{Name: "lead"}}, nil)
		g.AddEdge(memberID("lead"), memberID("impl"))
		problems := g.Check()
		if len(problems) != 1 || problems[0].Key != "error.graph_edge_to" {
			t.Fatalf("Check = %+v, want the edge's end", problems)
		}
		if got := problems[0].Args; len(got) != 2 || got[1] != "impl" {
			t.Errorf("the problem names %v, want the end impl", got)
		}
	})
	t.Run("the same turn twice", func(t *testing.T) {
		g := NewGraph([]MemberNode{{Name: "lead"}}, []WorkerNode{{From: "lead", Purpose: "調査"}})
		g.AddEdge(memberID("lead"), memberID("lead"))
		g.AddEdge(memberID("lead"), memberID("lead"))
		problems := g.Check()
		if len(problems) != 1 || problems[0].Key != "error.graph_edge_twice" {
			t.Fatalf("Check = %+v, want the repeated turn", problems)
		}
	})
	t.Run("a turn to the human is a turn like any other", func(t *testing.T) {
		g := NewGraph([]MemberNode{{Name: "lead"}}, nil)
		g.AddEdge(memberID("lead"), g.Human.ID())
		if problems := g.Check(); len(problems) != 0 {
			t.Errorf("Check = %+v, want nothing", problems)
		}
		if !g.ReachesHuman() {
			t.Error("ReachesHuman = false: the turn to the human was written")
		}
	})
}

// TestNodeIDsAreWrittenTheWaySettingsWriteThem: the human by "human", a member by
// its name, another kind as "<kind>:<name>" (d-11f8).
func TestNodeIDsAreWrittenTheWaySettingsWriteThem(t *testing.T) {
	cases := []struct {
		id   NodeID
		want string
	}{
		{Human{}.ID(), "human"},
		{memberID("lead"), "lead"},
		{workerID("lead", "調査"), "worker:lead/調査"},
		{workerID("impl", "試作"), "worker:impl/試作"},
		{NodeID{Kind: NodeKind("team"), Name: "review"}, "team:review"},
	}
	for _, tc := range cases {
		if got := tc.id.String(); got != tc.want {
			t.Errorf("NodeID%+v.String() = %q, want %q", tc.id, got, tc.want)
		}
	}
	if got := (Edge{From: memberID("lead"), To: Human{}.ID()}).String(); got != "lead -> human" {
		t.Errorf("Edge.String() = %q, want lead -> human", got)
	}
}

// workerID is a worker node's identity, so a test does not spell out the kind.
func workerID(from, purpose string) NodeID {
	return WorkerNode{From: from, Purpose: purpose}.ID()
}
