package activity

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

// TestMain loads the messages, so the lines a test reads are the ones a person
// sees.
func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// now is the moment every case is judged at.
var now = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

// idOf is one end of a turn, by the name a test writes: "human" is the human.
func idOf(name string) definition.NodeID {
	if name == "human" {
		return definition.NodeID{Kind: definition.KindHuman, Name: "human"}
	}
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// graphOf builds a graph of the members, with the turns a test writes as pairs
// of names.
func graphOf(members []string, turns ...[2]string) *definition.Graph {
	nodes := make([]definition.MemberNode, 0, len(members))
	for _, name := range members {
		nodes = append(nodes, definition.MemberNode{Name: name, Role: name, Profile: "claude"})
	}
	edges := make([]definition.Edge, 0, len(turns))
	for _, turn := range turns {
		edges = append(edges, definition.Edge{From: idOf(turn[0]), To: idOf(turn[1])})
	}
	return definition.NewGraph(nodes, nil, edges)
}

// ago is a watch whose state was first seen that long before now.
func ago(d time.Duration) Watch { return Watch{At: now.Add(-d)} }

// watchedAgo is a watch that was already there when the watching began: the
// length is a lower bound.
func watchedAgo(d time.Duration) Watch { return Watch{At: now.Add(-d), Lower: true} }

// TestOfStates: the four states of 5.2, each from made-up input, with the line,
// the node the line is about, and the length.
func TestOfStates(t *testing.T) {
	twoWay := graphOf([]string{"lead", "spec"},
		[2]string{"human", "lead"}, [2]string{"lead", "human"}, [2]string{"spec", "human"})
	// spec hands the turn to lead, not to the human: a stopped team that does
	// not wait for the human.
	throughLead := graphOf([]string{"lead", "spec"},
		[2]string{"human", "lead"}, [2]string{"lead", "human"}, [2]string{"spec", "lead"})

	cases := []struct {
		name     string
		in       Input
		state    State
		line     string
		last     string
		stalled  time.Duration
		lower    bool
		tail     string
		nodeLead NodeState
		nodeSpec NodeState
	}{
		{
			name: "waiting for an answer",
			in: Input{
				Graph:  twoWay,
				Agents: []Agent{{Herdr: "demo-lead", Node: "lead", Status: "blocked", Seq: 7}, {Herdr: "demo-spec", Node: "spec", Status: "idle", Seq: 2}},
				Tails:  map[string]string{"lead": "許可をください"},
				Seen:   map[string]Watch{"demo-lead": ago(25 * time.Minute)},
			},
			state: WaitingAnswer, line: i18n.Tf("activity.line_stuck", "lead", durationText(25*time.Minute, false)),
			last: "lead", stalled: 25 * time.Minute, tail: "許可をください",
			nodeLead: Blocked, nodeSpec: Idle,
		},
		{
			name: "waiting for the human's turn",
			in: Input{
				Graph:  twoWay,
				Agents: []Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 9}, {Herdr: "demo-spec", Node: "spec", Status: "idle", Seq: 3}},
				Tails:  map[string]string{"lead": "lead の最後のターン"},
				Seen:   map[string]Watch{"demo-lead": ago(12 * time.Minute)},
			},
			state: WaitingTurn, line: i18n.Tf("activity.line_waiting_human", "lead", durationText(12*time.Minute, false)),
			last: "lead", stalled: 12 * time.Minute, tail: "lead の最後のターン",
			nodeLead: HandedOver, nodeSpec: Idle,
		},
		{
			name: "stalled",
			in: Input{
				Graph:  throughLead,
				Agents: []Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 3}, {Herdr: "demo-spec", Node: "spec", Status: "idle", Seq: 8}},
				Tails:  map[string]string{"spec": "spec の最後のターン"},
				Seen:   map[string]Watch{"demo-spec": ago(4 * time.Minute)},
			},
			state: Stalled,
			line:  i18n.Tf("activity.line_team_stuck", durationText(4*time.Minute, false)) + i18n.Tf("activity.line_last_turn", "spec"),
			last:  "spec", stalled: 4 * time.Minute, tail: "spec の最後のターン",
			nodeLead: Idle, nodeSpec: Idle,
		},
		{
			name: "working",
			in: Input{
				Graph:  twoWay,
				Agents: []Agent{{Herdr: "demo-lead", Node: "lead", Status: "working", Seq: 5}, {Herdr: "demo-spec", Node: "spec", Status: "idle", Seq: 1}},
				Seen:   map[string]Watch{"demo-lead": ago(time.Second)},
			},
			state: Working,
			line: i18n.Tf("activity.node_state", "lead", i18n.T("activity.working")) +
				i18n.T("activity.node_separator") +
				i18n.Tf("activity.node_state", "spec", i18n.T("activity.stopped")),
			nodeLead: Active, nodeSpec: Idle,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.in.Team = "demo"
			c.in.Now = now
			got := Of(c.in)
			if got.State != c.state {
				t.Errorf("State = %v, want %v", got.State, c.state)
			}
			if got.Line != c.line {
				t.Errorf("Line = %q, want %q", got.Line, c.line)
			}
			if got.Last != c.last {
				t.Errorf("Last = %q, want %q", got.Last, c.last)
			}
			if got.Stalled != c.stalled || got.Lower != c.lower {
				t.Errorf("Stalled = %v (lower %v), want %v (lower %v)", got.Stalled, got.Lower, c.stalled, c.lower)
			}
			if got.Tail != c.tail {
				t.Errorf("Tail = %q, want %q", got.Tail, c.tail)
			}
			if len(got.Nodes) != 2 {
				t.Fatalf("Nodes = %+v, want the two members", got.Nodes)
			}
			byName := map[string]NodeState{}
			for _, node := range got.Nodes {
				byName[node.Name] = node.State
			}
			if byName["lead"] != c.nodeLead || byName["spec"] != c.nodeSpec {
				t.Errorf("node states = %+v, want lead %v and spec %v", byName, c.nodeLead, c.nodeSpec)
			}
		})
	}
}

// TestOfLowerBound: a state that was already there when the watching began reads
// as a lower bound, so a person does not read "0 分" for a team that has been
// stopped for hours (5.2). The node waiting for the human comes first.
func TestOfLowerBound(t *testing.T) {
	in := Input{
		Team:   "demo",
		Graph:  graphOf([]string{"lead"}, [2]string{"human", "lead"}, [2]string{"lead", "human"}),
		Agents: []Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 4}},
		Seen:   map[string]Watch{"demo-lead": watchedAgo(9 * time.Minute)},
		Now:    now,
	}
	got := Of(in)
	if got.State != WaitingTurn {
		t.Fatalf("State = %v, want the wait for the human's turn", got.State)
	}
	if want := i18n.Tf("activity.line_waiting_human", "lead", durationText(9*time.Minute, true)); got.Line != want {
		t.Errorf("Line = %q, want %q", got.Line, want)
	}
	if !got.Lower || got.Stalled != 9*time.Minute {
		t.Errorf("Stalled = %v (lower %v), want 9m and a lower bound", got.Stalled, got.Lower)
	}
}

// TestOfNoAgents: a team with no live agent is stopped, with no last turn to
// name and no length at all: nothing has been running, so any time would be made
// up (N4). The design names no state for it; it is not working, and nothing
// waits for the human.
func TestOfNoAgents(t *testing.T) {
	got := Of(Input{
		Team:  "demo",
		Graph: graphOf([]string{"lead"}, [2]string{"human", "lead"}, [2]string{"lead", "human"}),
		Now:   now,
	})
	if got.State != Stalled {
		t.Errorf("State = %v, want Stalled", got.State)
	}
	if got.Last != "" {
		t.Errorf("Last = %q, want no last turn", got.Last)
	}
	if got.Stalled != 0 || got.Lower {
		t.Errorf("Stalled = %v (lower %v), want no length", got.Stalled, got.Lower)
	}
	if want := i18n.Tf("activity.line_no_agent", "demo"); got.Line != want {
		t.Errorf("Line = %q, want %q", got.Line, want)
	}
	if len(got.Hired) != 0 {
		t.Errorf("Hired = %+v, want none", got.Hired)
	}
}

// TestOfUnknownLength: an agent the caller handed no watch for has no length the
// judgement can know, and no line says a time it does not have (N4): the line
// says the wait with the words for an unknown length rather than a made-up
// second. There is no lower bound either, because nothing was watched.
func TestOfUnknownLength(t *testing.T) {
	got := Of(Input{
		Team:   "demo",
		Graph:  graphOf([]string{"lead"}, [2]string{"human", "lead"}, [2]string{"lead", "human"}),
		Agents: []Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 4}},
		Now:    now,
	})
	if got.State != WaitingTurn {
		t.Fatalf("State = %v, want the wait for the human's turn", got.State)
	}
	want := i18n.Tf("activity.line_waiting_human", "lead", i18n.T("activity.unknown_length"))
	if got.Line != want {
		t.Errorf("Line = %q, want %q", got.Line, want)
	}
	if got.Stalled != 0 || got.Lower {
		t.Errorf("Stalled = %v (lower %v), want no length", got.Stalled, got.Lower)
	}
}

// TestWatcher: the watcher remembers when each agent's current state was first
// seen (N5), which is the part of 5.2 a screen must not have to keep itself. The
// agents of the first read were already there (a lower bound); an agent that
// appears later, or whose state_change_seq moved, is taken from that call on; an
// agent that is gone is forgotten.
func TestWatcher(t *testing.T) {
	watcher := NewWatcher()
	lead := Agent{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 4}
	spec := Agent{Herdr: "demo-spec", Node: "spec", Status: "idle", Seq: 1}

	first := watcher.Observe([]Agent{lead, spec}, now)
	if got, want := first["demo-lead"], (Watch{At: now, Lower: true}); got != want {
		t.Errorf("the first read = %+v, want %+v", got, want)
	}
	if got, want := first["demo-spec"], (Watch{At: now, Lower: true}); got != want {
		t.Errorf("the second agent of the first read = %+v, want %+v", got, want)
	}

	// Five minutes on, lead is in the same state: the watch it was given holds.
	later := now.Add(5 * time.Minute)
	held := watcher.Observe([]Agent{lead}, later)
	if got, want := held["demo-lead"], (Watch{At: now, Lower: true}); got != want {
		t.Errorf("a state that did not move = %+v, want %+v", got, want)
	}

	// lead moved, spec went away, and a worker appeared.
	moved := lead
	moved.Seq = 9
	worker := Agent{Herdr: "demo-coder", Status: "working", Seq: 1}
	next := watcher.Observe([]Agent{moved, worker}, later)
	if got, want := next["demo-lead"], (Watch{At: later}); got != want {
		t.Errorf("a state that moved = %+v, want %+v (not a lower bound)", got, want)
	}
	if _, ok := next["demo-spec"]; ok {
		t.Errorf("demo-spec is gone and still watched: %+v", next)
	}
	if got, want := next["demo-coder"], (Watch{At: later}); got != want {
		t.Errorf("an agent that appeared = %+v, want %+v", got, want)
	}

	// The one that is gone comes back: it is taken from this call on.
	back := watcher.Observe([]Agent{lead, spec}, later)
	if got, want := back["demo-spec"], (Watch{At: later}); got != want {
		t.Errorf("an agent that came back = %+v, want %+v", got, want)
	}
}

// TestTeamHired: a live agent that is no node of the graph is handed over as one
// of the hired workers, by the herdr name herdr knows it by and in a fixed order,
// so the team screen lists each under the member that hired it (3.2). It stays
// out of the graph's nodes.
func TestTeamHired(t *testing.T) {
	got := Of(Input{
		Team:  "demo",
		Graph: graphOf([]string{"lead"}, [2]string{"human", "lead"}, [2]string{"lead", "human"}),
		Agents: []Agent{
			{Herdr: "demo-lead", Node: "lead", Status: "working", Seq: 1},
			{Herdr: "demo-zoe", Status: "idle", Seq: 2},
			{Herdr: "demo-coder", Status: "working", Seq: 3},
		},
		Now: now,
	})
	if len(got.Hired) != 2 {
		t.Fatalf("Hired = %+v, want the two workers", got.Hired)
	}
	if got.Hired[0].Name != "demo-coder" || got.Hired[1].Name != "demo-zoe" {
		t.Errorf("Hired = %+v, want demo-coder then demo-zoe", got.Hired)
	}
	if got.Hired[0].Herdr != "demo-coder" || got.Hired[0].State != Active || got.Hired[0].Human {
		t.Errorf("hired[0] = %+v, want the working agent by its herdr name", got.Hired[0])
	}
	for _, node := range got.Nodes {
		if node.Name == "demo-coder" || node.Name == "demo-zoe" {
			t.Errorf("a hired worker is among the graph's nodes: %+v", got.Nodes)
		}
	}
}

// TestOfWorkerAgent: an agent that is no node of the graph (a hired worker)
// counts for whether the team works, and stands in the line by its herdr name,
// so a team whose only working agent is a worker does not read as stopped.
func TestOfWorkerAgent(t *testing.T) {
	got := Of(Input{
		Team:  "demo",
		Graph: graphOf([]string{"lead"}, [2]string{"human", "lead"}, [2]string{"lead", "human"}),
		Agents: []Agent{
			{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 1},
			{Herdr: "demo-coder", Status: "working", Seq: 2},
		},
		Now: now,
	})
	if got.State != Working {
		t.Fatalf("State = %v, want Working", got.State)
	}
	want := i18n.Tf("activity.node_state", "demo-coder", i18n.T("activity.working")) +
		i18n.T("activity.node_separator") +
		i18n.Tf("activity.node_state", "lead", i18n.T("activity.stopped"))
	if got.Line != want {
		t.Errorf("Line = %q, want %q", got.Line, want)
	}
}

// TestSort: the teams waiting for a human come first, the one waiting longest
// first, then the stopped ones, then the working ones (5.2).
func TestSort(t *testing.T) {
	teams := []Team{
		{Name: "working", State: Working},
		{Name: "stopped-short", State: Stalled, Stalled: time.Minute},
		{Name: "answer", State: WaitingAnswer, Stalled: 2 * time.Minute},
		{Name: "turn", State: WaitingTurn, Stalled: 30 * time.Minute},
		{Name: "stopped-long", State: Stalled, Stalled: time.Hour},
	}
	Sort(teams)
	want := []string{"turn", "answer", "stopped-long", "stopped-short", "working"}
	for i, name := range want {
		if teams[i].Name != name {
			t.Errorf("teams[%d] = %q, want %q (order: %v)", i, teams[i].Name, name, names(teams))
		}
	}
}

// names is the order of the teams, for a failure message.
func names(teams []Team) []string {
	out := make([]string, 0, len(teams))
	for _, team := range teams {
		out = append(out, team.Name)
	}
	return out
}

// TestLinesReadInJapanese: the sentences a person reads are the ones the design
// writes (5.2), word for word.
func TestLinesReadInJapanese(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Use(i18n.Fallback) })
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	twoWay := graphOf([]string{"lead", "spec"},
		[2]string{"human", "lead"}, [2]string{"lead", "human"}, [2]string{"spec", "human"})
	throughLead := graphOf([]string{"lead", "spec"},
		[2]string{"human", "lead"}, [2]string{"lead", "human"}, [2]string{"spec", "lead"})

	cases := []struct {
		name string
		in   Input
		want string
	}{
		{
			name: "waiting for an answer",
			in: Input{
				Graph:  twoWay,
				Agents: []Agent{{Herdr: "demo-lead", Node: "lead", Status: "blocked", Seq: 2}},
				Seen:   map[string]Watch{"demo-lead": ago(25 * time.Minute)},
			},
			want: "lead が 25 分前から止まっています",
		},
		{
			name: "waiting for the human's turn",
			in: Input{
				Graph:  twoWay,
				Agents: []Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 2}},
				Seen:   map[string]Watch{"demo-lead": ago(12 * time.Minute)},
			},
			want: "lead が 12 分前から人間を待っています",
		},
		{
			name: "stalled",
			in: Input{
				Graph:  throughLead,
				Agents: []Agent{{Herdr: "demo-spec", Node: "spec", Status: "idle", Seq: 2}},
				Seen:   map[string]Watch{"demo-spec": ago(4 * time.Minute)},
			},
			want: "チームが 4 分前から止まっています。最後のターンは spec",
		},
		{
			name: "working",
			in: Input{
				Graph: twoWay,
				Agents: []Agent{
					{Herdr: "demo-lead", Node: "lead", Status: "working", Seq: 2},
					{Herdr: "demo-spec", Node: "spec", Status: "idle", Seq: 1},
				},
			},
			want: "lead が仕事中・spec が止まっています",
		},
		{
			name: "no live agent",
			in:   Input{Graph: twoWay},
			want: "チーム demo のエージェントがいません",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.in.Team, c.in.Now = "demo", now
			if got := Of(c.in).Line; got != c.want {
				t.Errorf("Line = %q, want %q", got, c.want)
			}
		})
	}
}
