package activity

import (
	"testing"
	"time"
)

func TestOfUsesTheTurnOnlyWithAWatcher(t *testing.T) {
	cases := []struct {
		name     string
		statuses []string
		turn     *Turn
		watching bool
		want     State
	}{
		{"permission before human turn", []string{"blocked", "working"}, &Turn{Holder: "human"}, true, WaitingAnswer},
		{"human before working", []string{"idle", "working"}, &Turn{Holder: "human", From: "lead"}, true, WaitingTurn},
		{"working", []string{"working"}, &Turn{Holder: "lead"}, true, Working},
		{"member owns stopped turn", []string{"idle"}, &Turn{Holder: "lead"}, true, Stalled},
		{"human owns stopped turn", []string{"idle"}, &Turn{Holder: "human", From: "lead"}, true, WaitingTurn},
		{"no watcher ignores old human turn", []string{"idle"}, &Turn{Holder: "human"}, false, Stalled},
		{"no state ignores topology", []string{"idle"}, nil, true, Stalled},
		{"no watcher still sees work", []string{"working"}, &Turn{Holder: "human"}, false, Working},
		{"no state still sees permission", []string{"blocked"}, nil, true, WaitingAnswer},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := Input{Team: fixtureSeen("demo"), Graph: graphOf([]string{"lead", "dev"}, [2]string{"lead", "human"}), Turn: c.turn, Watching: c.watching, Now: now}
			for i, status := range c.statuses {
				name := []string{"lead", "dev"}[i]
				in.Agents = append(in.Agents, Agent{Herdr: fixtureSeen("demo-" + name), Node: fixtureSeen(name), Status: status, Seq: i + 1})
			}
			if got := Of(in); got.State != c.want {
				t.Fatalf("state = %v, want %v", got.State, c.want)
			}
		})
	}
}

func TestOfMarksTheTurnSenderInsteadOfTheLastStop(t *testing.T) {
	got := Of(Input{Team: fixtureSeen("demo"), Graph: graphOf([]string{"lead", "dev"}, [2]string{"dev", "human"}),
		Agents:   []Agent{{Herdr: fixtureSeen("demo-lead"), Node: fixtureSeen("lead"), Status: "idle", Seq: 1}, {Herdr: fixtureSeen("demo-dev"), Node: fixtureSeen("dev"), Status: "idle", Seq: 9}},
		Watching: true, Turn: &Turn{Holder: "human", From: "lead", Since: now.Add(-time.Minute)}, Now: now})
	for _, node := range got.Nodes {
		if (node.State == HandedOver) != (node.Name == "lead") {
			t.Fatalf("wrong sender marked: %+v", got.Nodes)
		}
	}
}

func TestOfDoesNotMarkAHumanHandoffWithoutAWatcher(t *testing.T) {
	got := Of(Input{Team: fixtureSeen("demo"), Graph: graphOf([]string{"lead"}, [2]string{"lead", "human"}),
		Agents: []Agent{{Herdr: fixtureSeen("demo-lead"), Node: fixtureSeen("lead"), Status: "idle"}},
		Turn:   &Turn{Holder: "human", From: "lead"}, Watching: false, Now: now})
	if got.State != Stalled || got.Nodes[0].State != Idle {
		t.Fatalf("old handoff was trusted: %+v", got)
	}
}
