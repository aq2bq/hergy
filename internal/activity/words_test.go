package activity

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestOfNodesStopLastFirst (6 A3): within one state the node whose
// state_change_seq is greatest comes first — it stopped last — and the worked
// example's order is what a person reads in core's member list.
func TestOfNodesStopLastFirst(t *testing.T) {
	got := Of(Input{
		Team:  "core",
		Graph: graphOf([]string{"lead", "impl-a", "impl-b"}, [2]string{"lead", "human"}),
		Agents: []Agent{
			{Herdr: "core-lead", Node: "lead", Status: "idle", Seq: 40},
			{Herdr: "core-impl-a", Node: "impl-a", Status: "idle", Seq: 30},
			{Herdr: "core-impl-b", Node: "impl-b", Status: "idle", Seq: 20},
		},
		Now: now,
	})
	var order []string
	for _, node := range got.Nodes {
		order = append(order, node.Name)
	}
	if want := []string{"lead", "impl-a", "impl-b"}; !slices.Equal(order, want) {
		t.Errorf("nodes = %v, want %v (the last one to stop first)", order, want)
	}
}

// TestHiredStopLastFirst (6 A3): the workers are ordered the way the members are
// — waiting for the human first, then working, then stopped, the last one to stop
// first — not by name.
func TestHiredStopLastFirst(t *testing.T) {
	got := Of(Input{
		Team:  "demo",
		Graph: graphOf([]string{"lead"}, [2]string{"human", "lead"}),
		Agents: []Agent{
			{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 1},
			{Herdr: "demo-zoe", Status: "idle", Seq: 2},
			{Herdr: "demo-coder", Status: "working", Seq: 3},
			{Herdr: "demo-anna", Status: "idle", Seq: 9},
		},
		Now: now,
	})
	var order []string
	for _, node := range got.Hired {
		order = append(order, node.Name)
	}
	if want := []string{"demo-coder", "demo-anna", "demo-zoe"}; !slices.Equal(order, want) {
		t.Errorf("hired = %v, want %v (working first, then the last to stop)", order, want)
	}
}

// TestNodeAt: a node that is stopped or waiting carries the moment its state
// began, and a working node and one with no agent carry none, so the screen has
// no moment to print for them (2.5).
func TestNodeAt(t *testing.T) {
	at := now.Add(-90 * time.Minute)
	got := Of(Input{
		Team:  "demo",
		Graph: graphOf([]string{"lead", "spec", "other"}, [2]string{"human", "lead"}),
		Agents: []Agent{
			{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 3},
			{Herdr: "demo-spec", Node: "spec", Status: "working", Seq: 2},
			{Herdr: "demo-coder", Status: "idle", Seq: 1},
		},
		Seen: map[string]Watch{
			"demo-lead":  ago(90 * time.Minute),
			"demo-spec":  ago(90 * time.Minute),
			"demo-coder": ago(90 * time.Minute),
		},
		Now: now,
	})
	byName := map[string]Node{}
	for _, node := range got.Nodes {
		byName[node.Name] = node
	}
	if got := byName["lead"].At; !got.Equal(at) {
		t.Errorf("the stopped node's moment = %s, want %s", got, at)
	}
	if got := byName["spec"].At; !got.IsZero() {
		t.Errorf("a working node's moment = %s, want none", got)
	}
	if got := byName["other"].At; !got.IsZero() {
		t.Errorf("a node with no agent has a moment: %s", got)
	}
	if len(got.Hired) != 1 || !got.Hired[0].At.Equal(at) {
		t.Errorf("Hired = %+v, want the worker with its own moment", got.Hired)
	}
}

// TestWatcherKeepsItsFile (6 A2): the memory is written where it was told to, and
// a watcher built over that file reads it back: an agent whose seq did not move
// keeps the moment the earlier watcher saw, and one whose seq moved is taken from
// now on.
func TestWatcherKeepsItsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "activity.json")
	first := NewWatcherAt(path)
	lead := Agent{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 4}
	stopped := now.Add(-95 * time.Minute)
	if _, err := os.Stat(path); err == nil {
		t.Error("the file is written before anything was seen")
	}
	first.Observe([]Agent{lead}, stopped)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the memory was not written: %v", err)
	}

	// A serve started again, hours later: the file says when lead stopped.
	again := NewWatcherAt(path)
	later := now.Add(3 * time.Hour)
	seen := again.Observe([]Agent{lead}, later)
	if got, want := seen["demo-lead"], (Watch{At: stopped, Lower: true}); got != want {
		t.Errorf("a state that did not move across a restart = %+v, want %+v", got, want)
	}

	// lead moved: the moment is this read's.
	moved := lead
	moved.Seq = 5
	seen = again.Observe([]Agent{moved}, later)
	if got, want := seen["demo-lead"], (Watch{At: later}); got != want {
		t.Errorf("a state that moved = %+v, want %+v", got, want)
	}

	// An agent that appeared while nobody watched is taken from this read on,
	// not called a lower bound: the earlier serve says which state began when.
	fresh := Agent{Herdr: "demo-new", Status: "idle", Seq: 1}
	seen = again.Observe([]Agent{moved, fresh}, later)
	if got, want := seen["demo-new"], (Watch{At: later}); got != want {
		t.Errorf("an agent that appeared = %+v, want %+v", got, want)
	}
}

// TestWatcherWithoutAFileBeginsLower: a watcher with no file remembers nothing
// across a serve, so the agents of its first read were already in their states
// when the watching began (6 A2).
func TestWatcherWithoutAFileBeginsLower(t *testing.T) {
	seen := NewWatcher().Observe([]Agent{{Herdr: "demo-lead", Node: "lead", Status: "idle", Seq: 1}}, now)
	if got, want := seen["demo-lead"], (Watch{At: now, Lower: true}); got != want {
		t.Errorf("the first read = %+v, want %+v", got, want)
	}
}

// TestWordsReadInJapanese: the words about time are the ones the design writes (6
// A4), word for word, and the states are the ones the API writes (spec-activity
// 3).
func TestWordsReadInJapanese(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Use(i18n.Fallback) })
	if err := i18n.Use("ja"); err != nil {
		t.Fatal(err)
	}
	noon := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"a length", Length(18*time.Minute, false), "18 分"},
		{"a length that is a lower bound", Length(4*time.Hour+5*time.Minute, true), "4 時間 5 分以上"},
		{"today's clock", Clock(noon, noon.Add(-2*time.Hour)), "10:00"},
		{"another day's clock", Clock(noon, noon.Add(-25*time.Hour)), "10/16 11:00"},
		{"a moment it began", SinceAt(noon, noon.Add(-2*time.Hour), false), "10:00 から"},
		{"a moment it was already there", SinceAt(noon, noon.Add(-2*time.Hour), true), "10:00 より前"},
		{"a state", WaitingAnswer.Slug() + "/" + WaitingTurn.Slug() + "/" + Stalled.Slug() + "/" + Working.Slug(), "answer/turn/stalled/working"},
		{"a node's state", Blocked.Slug() + "/" + HandedOver.Slug() + "/" + Active.Slug() + "/" + Idle.Slug() + "/" + Absent.Slug(), "blocked/handed/active/idle/absent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
			}
		})
	}
}
