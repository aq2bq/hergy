package turns

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
)

func humanID() definition.NodeID { return definition.Human{}.ID() }
func member(name string) definition.NodeID {
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}
func edge(from, to definition.NodeID) definition.Edge { return definition.Edge{From: from, To: to} }

// ring is the 2026-11-05 team: lead talks to the human and spec, spec to lead
// and dev, dev to spec alone (r-bc26 の実例).
func ring() *definition.Graph {
	return definition.NewGraph(
		[]definition.MemberNode{{Name: "lead"}, {Name: "spec"}, {Name: "dev"}},
		[]definition.WorkerNode{{From: "spec", Purpose: "開発", Profile: "ds4f"}},
		[]definition.Edge{
			edge(humanID(), member("lead")), edge(member("lead"), humanID()), edge(member("lead"), member("spec")),
			edge(member("spec"), member("lead")), edge(member("dev"), member("spec")), edge(member("spec"), member("dev")),
		},
	)
}

var at = time.Date(2026, 11, 6, 21, 20, 0, 0, time.UTC)

func TestDestinationFollowsTheEdgesAndTheOneWhoHanded(t *testing.T) {
	g := ring()
	cases := []struct {
		name      string
		a         Agent
		from      string
		hint      string
		wantTo    string
		wantHuman bool
	}{
		{"one edge: dev goes to spec", Agent{Name: "dev"}, "", "", "spec", false},
		{"two edges: spec returns to the one who handed it", Agent{Name: "spec"}, "dev", "", "dev", false},
		{"two edges: spec, handed by lead, returns to lead", Agent{Name: "spec"}, "lead", "", "lead", false},
		{"two edges, nobody handed it: the first turns: names", Agent{Name: "spec"}, "", "", "lead", false},
		{"lead handed by the human goes to the human", Agent{Name: "lead"}, Human, "", Human, true},
		{"lead handed by spec goes to spec", Agent{Name: "lead"}, "spec", "", "spec", false},
		{"a hint names the party", Agent{Name: "lead"}, "spec", Human, Human, true},
		{"a hint outside the edges is ignored", Agent{Name: "dev"}, "", "lead", "spec", false},
		{"a worker returns to the member that hired it", Agent{Name: "dev-scroll", Hirer: "spec"}, "", "", "spec", false},
	}
	for _, c := range cases {
		to, human, _ := Destination(g, c.a, c.from, c.hint)
		if to != c.wantTo || human != c.wantHuman {
			t.Errorf("%s: Destination = %q, %v, want %q, %v", c.name, to, human, c.wantTo, c.wantHuman)
		}
	}
	if to, human, why := Destination(definition.NewGraph([]definition.MemberNode{{Name: "a"}}, nil, nil), Agent{Name: "a"}, "", ""); to != Human || !human || why != WhyNoEdge {
		t.Errorf("a member with no edge: %q, %v, %q, want the human with WhyNoEdge", to, human, why)
	}
}

func TestHintReadsTheLastLine(t *testing.T) {
	cases := map[string]string{
		"結果を書いた。\n→ spec\n":    "spec",
		"結果を書いた。\n-> lead":     "lead",
		"結果を書いた。\n→ 人間\n\n":    Human,
		"結果を書いた。\n→ human":     Human,
		"→ spec と書いたが、そのあとも続く": "",
		"": "",
	}
	for text, want := range cases {
		if got := Hint(text); got != want {
			t.Errorf("Hint(%q) = %q, want %q", text, got, want)
		}
	}
}

func wrote(texts map[string]string) func(string) string {
	return func(name string) string { return texts[name] }
}

func TestStepHandsAStoppedTurnOn(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec"}
	before := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "working", Seq: 2}}
	now := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "done", Seq: 3}}
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"dev": "d6465cd を積んだ。次に続ける。"}), At: at})
	if len(got) != 1 || got[0].Kind != Deliver || got[0].To != "spec" || got[0].From != "dev" || len(got[0].Messages) != 1 {
		t.Fatalf("Step = %+v, want one delivery dev -> spec", got)
	}
	if st.Holder != "spec" || st.From != "dev" || st.Handed["dev"] != 3 {
		t.Errorf("state = %+v, want spec holding it from dev, dev's stop 3 handed", st)
	}
	// The same observation again hands nothing on: one handing per stop.
	if again := Step(st, Input{Graph: ring(), Before: now, Now: now, Wrote: wrote(map[string]string{"dev": "x"}), At: at}); len(again) != 0 {
		t.Errorf("the same stop handed on twice: %+v", again)
	}
}

func TestStepBoxesForAWorkingMemberAndEmptiesTheBoxWhenItStops(t *testing.T) {
	g := ring()
	st := &State{Team: "t", Holder: "spec", From: "lead"}
	// spec is working while dev stops: dev's text waits in spec's box.
	before := []Agent{{Name: "spec", Status: "working", Seq: 5}, {Name: "dev", Status: "working", Seq: 6}}
	now := []Agent{{Name: "spec", Status: "working", Seq: 5}, {Name: "dev", Status: "done", Seq: 7}}
	got := Step(st, Input{Graph: g, Before: before, Now: now, Wrote: wrote(map[string]string{"dev": "push した"}), At: at})
	if len(got) != 1 || got[0].Kind != Box || got[0].To != "spec" {
		t.Fatalf("Step = %+v, want dev's text boxed for spec", got)
	}
	if st.Holder != "spec" || len(st.Pending["spec"]) != 1 {
		t.Errorf("state = %+v, want spec still holding, one message boxed", st)
	}
	// spec stops: its own text goes back to lead (who handed it), and the box
	// empties into spec as one delivery.
	later := []Agent{{Name: "spec", Status: "done", Seq: 8}, {Name: "dev", Status: "done", Seq: 7}}
	got = Step(st, Input{Graph: g, Before: now, Now: later, Wrote: wrote(map[string]string{"spec": "照合した"}), At: at.Add(time.Minute)})
	if len(got) != 2 {
		t.Fatalf("Step = %+v, want spec -> lead and the box -> spec", got)
	}
	if got[0].Kind != Deliver || got[0].To != "lead" || got[0].From != "spec" {
		t.Errorf("first = %+v, want spec's text delivered to lead", got[0])
	}
	if got[1].Kind != Deliver || got[1].To != "spec" || got[1].From != "dev" || got[1].Messages[0].Text != "push した" {
		t.Errorf("second = %+v, want the box emptied into spec", got[1])
	}
	if len(st.Pending) != 0 {
		t.Errorf("the box still holds %+v", st.Pending)
	}
}

func TestStepHandsASilentStopToTheHuman(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec"}
	before := []Agent{{Name: "dev", Status: "working", Seq: 1}}
	now := []Agent{{Name: "dev", Status: "idle", Seq: 2}}
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(nil), At: at})
	if len(got) != 1 || got[0].Kind != ToHuman || got[0].Why != WhySilent {
		t.Fatalf("Step = %+v, want the human to hold a silent stop", got)
	}
	if st.Holder != Human || st.Why != WhySilent || st.From != "dev" {
		t.Errorf("state = %+v", st)
	}
}

func TestStepHandsLeadsAnswerToTheHumanAlongTheEdge(t *testing.T) {
	st := &State{Team: "t", Holder: "lead", From: Human}
	before := []Agent{{Name: "lead", Status: "working", Seq: 1}}
	now := []Agent{{Name: "lead", Status: "done", Seq: 2}}
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"lead": "進捗です。"}), At: at})
	if len(got) != 1 || got[0].Kind != ToHuman || got[0].Why != WhyEdge || len(got[0].Messages) != 1 {
		t.Fatalf("Step = %+v, want the turn to reach the human with lead's text", got)
	}
	if st.Holder != Human || st.Why != "" {
		t.Errorf("state = %+v, want the human holding it with no why", st)
	}
}

func TestStepSeesTheHumanStartATurn(t *testing.T) {
	st := &State{Team: "t", Holder: Human, From: "lead"}
	before := []Agent{{Name: "lead", Status: "idle", Seq: 1}}
	now := []Agent{{Name: "lead", Status: "working", Seq: 2}}
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, At: at})
	if len(got) != 1 || got[0].Kind != Started || got[0].To != "lead" || got[0].From != Human {
		t.Fatalf("Step = %+v, want lead's turn started by the human", got)
	}
	if st.Holder != "lead" || st.From != Human {
		t.Errorf("state = %+v", st)
	}
	// A start hergy itself handed is not a Started: the holder is already set.
	st = &State{Team: "t", Holder: "spec", From: "dev"}
	got = Step(st, Input{Graph: ring(), Before: []Agent{{Name: "spec", Status: "idle", Seq: 1}}, Now: []Agent{{Name: "spec", Status: "working", Seq: 2}}, At: at})
	if len(got) != 0 {
		t.Errorf("hergy's own delivery landing read as a start: %+v", got)
	}
}

func TestStepLeavesABlockedMemberAlone(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec"}
	before := []Agent{{Name: "dev", Status: "working", Seq: 1}}
	now := []Agent{{Name: "dev", Status: "blocked", Seq: 2}}
	if got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"dev": "許可を待つ"}), At: at}); len(got) != 0 {
		t.Errorf("a blocked member (asking the person) was handed on: %+v", got)
	}
	if st.Holder != "dev" {
		t.Errorf("holder = %q, want dev still", st.Holder)
	}
}

func TestStepReturnsAWorkerToItsHirer(t *testing.T) {
	st := &State{Team: "t", Holder: "scroll", From: "spec"}
	before := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "scroll", Hirer: "spec", Status: "working", Seq: 2}}
	now := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "scroll", Hirer: "spec", Status: "done", Seq: 3}}
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"scroll": "⑦ を push した"}), At: at})
	if len(got) != 1 || got[0].Kind != Deliver || got[0].To != "spec" || got[0].From != "scroll" {
		t.Fatalf("Step = %+v, want the worker's text delivered to spec", got)
	}
}

func TestLoadAndSaveKeepTheTurn(t *testing.T) {
	path := Path(t.TempDir(), "demo")
	st, err := Load(path, "demo")
	if err != nil || st.Holder != Human || st.Team != "demo" {
		t.Fatalf("Load of nothing = %+v, %v, want the human holding demo's turn", st, err)
	}
	st.Holder, st.From, st.Since = "lead", Human, at
	st.Pending = map[string][]Message{"spec": {{From: "dev", At: at, Text: "x"}}}
	st.Handed = map[string]int{"dev": 3}
	if err := Save(path, st); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filepath.Join(filepath.Dir(path), "other.json"), "other"); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path, "demo")
	if err != nil || back.Holder != "lead" || back.From != Human || !back.Since.Equal(at) || back.Pending["spec"][0].Text != "x" || back.Handed["dev"] != 3 {
		t.Errorf("Load after Save = %+v, %v", back, err)
	}
}
