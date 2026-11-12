package turns

import (
	"os"
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
		[]definition.MemberNode{{Name: fixtureMemberName("lead")}, {Name: fixtureMemberName("spec")}, {Name: fixtureMemberName("dev")}},
		[]definition.WorkerNode{{From: "spec", Purpose: testPurpose("開発"), Profile: "ds4f"}},
		[]definition.Edge{
			edge(humanID(), member("lead")), edge(member("lead"), humanID()), edge(member("lead"), member("spec")),
			edge(member("spec"), member("lead")), edge(member("dev"), member("spec")), edge(member("spec"), member("dev")),
		},
	)
}

var at = time.Date(2026, 11, 6, 21, 20, 0, 0, time.UTC)

func wrote(texts map[string]string) func(string) string {
	return func(name string) string { return texts[name] }
}

func agents(status map[string]string, seq int) []Agent {
	var out []Agent
	for _, name := range []string{"lead", "spec", "dev"} {
		if s, ok := status[name]; ok {
			out = append(out, Agent{Name: name, Status: s, Seq: seq})
		}
	}
	return out
}

func TestCandidatesNameTheWaitingThenTheEdgesThenTheHirer(t *testing.T) {
	g := ring()
	if got := Candidates(g, Agent{Name: "spec"}, []string{"dev"}); len(got) != 2 || got[0] != "dev" || got[1] != "lead" {
		t.Errorf("spec, dev waiting: %q, want dev then lead (the human is not a candidate name)", got)
	}
	if got := Candidates(g, Agent{Name: "lead"}, nil); len(got) != 1 || got[0] != "spec" {
		t.Errorf("lead: %q, want spec alone", got)
	}
	if got := Candidates(g, Agent{Name: "scroll", Hirer: "spec"}, nil); len(got) != 1 || got[0] != "spec" {
		t.Errorf("a worker: %q, want its hirer", got)
	}
	if got := Candidates(nil, Agent{Name: "a"}, nil); len(got) != 0 {
		t.Errorf("no graph, nobody waiting: %q, want none", got)
	}
}

func TestAnswerNamesOneCandidateTheHumanOrNobody(t *testing.T) {
	cands := []string{"lead", "spec"}
	cases := map[string]string{
		"lead":           "lead",
		"lead に渡します。":    "lead",
		"t-spec":         "spec",
		"人間":             Human,
		"human":          Human,
		"外側へ。":           Human,
		"誰にも":            Nobody,
		"nobody":         Nobody,
		"lead ではなく spec": "",
		"leader に":       "",
		"":               "",
		"渡し先は spec です。lead は待たなくてよい":                           "",
		"hergy より: 次の 1 語だけで答えてください: lead／spec／人間／誰にも\n\nlead": "lead",
	}
	for text, want := range cases {
		if got := Answer(text, fixtureNaming("t"), cands); got != want {
			t.Errorf("Answer(%q) = %q, want %q", text, got, want)
		}
	}
}

// TestATurnTheOutsideOpenedRests: nobody told dev anything; it works and stops
// with text. Its reply is the outside's: nothing is sent, the human holds the
// turn, and the line says so (d-b024). The start prompt's reply and a reply to
// the person's tell are this case (ac-6fe0).
func TestATurnTheOutsideOpenedRests(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"lead": "idle", "spec": "idle", "dev": "idle"}, 1)
	busy := agents(map[string]string{"lead": "working", "spec": "working", "dev": "working"}, 2)
	if got := Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at}); len(got) != 3 || got[0].Kind != Started {
		t.Fatalf("Step = %+v, want three starts by the person", got)
	}
	stopped := agents(map[string]string{"lead": "done", "spec": "done", "dev": "done"}, 3)
	got := Step(st, Input{Graph: ring(), Before: busy, Now: stopped, At: at.Add(time.Minute),
		Wrote: wrote(map[string]string{"lead": "起動しました", "spec": "台帳を見ました。dev に頼む", "dev": "spec に要求を進めてほしい"})})
	if len(got) != 3 {
		t.Fatalf("Step = %+v, want three rested turns", got)
	}
	for _, a := range got {
		if a.Kind != Rested {
			t.Errorf("%+v: want Rested, nothing handed, nothing asked", a)
		}
	}
	if st.Holder != Human || len(st.Asked) != 0 || len(st.Pending) != 0 {
		t.Errorf("state = %+v, want the human holding it and nothing asked", st)
	}
	// The person tells dev; dev answers and stops: still the outside's.
	_ = Step(st, Input{Graph: ring(), Before: stopped, Now: stopped, Tells: []Tell{{To: "dev", At: at.Add(2 * time.Minute)}}, At: at.Add(2 * time.Minute)})
	working := agents(map[string]string{"lead": "done", "spec": "done", "dev": "working"}, 4)
	if got := Step(st, Input{Graph: ring(), Before: stopped, Now: working, At: at.Add(3 * time.Minute)}); len(got) != 0 {
		t.Errorf("the tell landing read as the person typing: %+v", got)
	}
	done := agents(map[string]string{"lead": "done", "spec": "done", "dev": "done"}, 5)
	got = Step(st, Input{Graph: ring(), Before: working, Now: done, At: at.Add(4 * time.Minute), Wrote: wrote(map[string]string{"dev": "了解、待機します"})})
	if len(got) != 1 || got[0].Kind != Rested || got[0].From != "dev" {
		t.Errorf("Step = %+v, want dev's reply to the person to rest", got)
	}
}

// TestAMemberThatToldHandedItsTurnOnItself: lead tells dev; dev works, tells
// spec, stops. hergy records both and does nothing else (d-8b4e).
func TestAMemberThatToldHandedItsTurnOnItself(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"lead": "idle", "spec": "idle", "dev": "idle"}, 1)
	got := Step(st, Input{Graph: ring(), Before: idle, Now: idle, Tells: []Tell{{From: "lead", To: "dev", At: at}}, At: at})
	if len(got) != 1 || got[0].Kind != Told || got[0].From != "lead" || got[0].To != "dev" {
		t.Fatalf("Step = %+v, want lead's tell noted", got)
	}
	if st.Holder != "dev" || st.From != "lead" || len(st.Opened["dev"]) != 1 || st.Waits()[0] != "lead → dev" {
		t.Errorf("state = %+v, want dev holding it from lead, lead waiting", st)
	}
	busy := agents(map[string]string{"lead": "idle", "spec": "idle", "dev": "working"}, 2)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at.Add(time.Second)})
	_ = Step(st, Input{Graph: ring(), Before: busy, Now: busy, Tells: []Tell{{From: "dev", To: "spec", At: at.Add(time.Minute)}}, At: at.Add(time.Minute)})
	stopped := agents(map[string]string{"lead": "idle", "spec": "working", "dev": "done"}, 3)
	got = Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"dev": "spec に渡した"}), At: at.Add(2 * time.Minute)})
	if len(got) != 1 || got[0].Kind != Passed || got[0].From != "dev" {
		t.Fatalf("Step = %+v, want dev's turn passed by itself, nothing asked", got)
	}
	if len(st.Asked) != 0 || len(st.Opened["dev"]) != 0 || st.Holder != "spec" {
		t.Errorf("state = %+v, want nothing asked, dev's opening consumed, spec holding", st)
	}
}

// TestAnUnhandedTurnIsAskedAboutAndHandedAsAnswered: lead tells dev; dev stops
// without a tell. hergy asks dev, naming lead as waiting; dev answers "lead";
// hergy delivers what dev wrote to lead, and lead's reply without a tell is
// asked about in turn (d-80a5, ac-c928).
func TestAnUnhandedTurnIsAskedAboutAndHandedAsAnswered(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"lead": "idle", "spec": "idle", "dev": "idle"}, 1)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: idle, Tells: []Tell{{From: "lead", To: "dev", At: at}}, At: at})
	busy := agents(map[string]string{"lead": "idle", "spec": "idle", "dev": "working"}, 2)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at.Add(time.Second)})
	stopped := agents(map[string]string{"lead": "idle", "spec": "idle", "dev": "done"}, 3)
	got := Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"dev": "d6465cd を積んだ。"}), At: at.Add(time.Minute)})
	if len(got) != 1 || got[0].Kind != Ask || got[0].To != "dev" || got[0].Route != RouteHerdr {
		t.Fatalf("Step = %+v, want dev asked", got)
	}
	if len(got[0].Waiting) != 1 || got[0].Waiting[0] != "lead" || len(got[0].Candidates) != 2 || got[0].Candidates[0] != "lead" || got[0].Candidates[1] != "spec" {
		t.Errorf("ask = %+v, want lead waiting and the candidates lead, spec", got[0])
	}
	if q, ok := st.Asked["dev"]; !ok || q.Text != "d6465cd を積んだ。" || q.Seq != 3 {
		t.Errorf("state = %+v, want the question remembered with dev's text", st)
	}
	// The question lands: not the person's start.
	answering := agents(map[string]string{"lead": "idle", "spec": "idle", "dev": "working"}, 4)
	if got := Step(st, Input{Graph: ring(), Before: stopped, Now: answering, At: at.Add(2 * time.Minute)}); len(got) != 0 {
		t.Errorf("the question landing read as the person typing: %+v", got)
	}
	// dev answers "lead": the text that waited goes to lead, as dev's answer.
	answered := agents(map[string]string{"lead": "idle", "spec": "idle", "dev": "done"}, 5)
	got = Step(st, Input{Graph: ring(), Before: answering, Now: answered, Wrote: wrote(map[string]string{"dev": "lead"}), At: at.Add(3 * time.Minute)})
	if len(got) != 1 || got[0].Kind != Deliver || got[0].To != "lead" || got[0].From != "dev" || got[0].Messages[0].Text != "d6465cd を積んだ。" {
		t.Fatalf("Step = %+v, want dev's text delivered to lead", got)
	}
	if st.Holder != "lead" || st.From != "dev" || len(st.Asked) != 0 || len(st.Opened["lead"]) != 1 || st.Opened["lead"][0].By != "dev" {
		t.Errorf("state = %+v, want lead holding it from dev, dev's opening on lead", st)
	}
	// lead works and stops without a tell: asked, with dev waiting.
	leadBusy := agents(map[string]string{"lead": "working", "spec": "idle", "dev": "done"}, 6)
	_ = Step(st, Input{Graph: ring(), Before: answered, Now: leadBusy, At: at.Add(4 * time.Minute)})
	leadDone := agents(map[string]string{"lead": "done", "spec": "idle", "dev": "done"}, 7)
	got = Step(st, Input{Graph: ring(), Before: leadBusy, Now: leadDone, Wrote: wrote(map[string]string{"lead": "通った。次は段 2。"}), At: at.Add(5 * time.Minute)})
	if len(got) != 1 || got[0].Kind != Ask || got[0].To != "lead" || len(got[0].Waiting) != 1 || got[0].Waiting[0] != "dev" {
		t.Fatalf("Step = %+v, want lead asked with dev waiting", got)
	}
}

// TestAnAnswerForAWorkingMemberIsBoxedAndOpensItsNextTurn: dev answers
// "spec" while spec works; the text waits in spec's box and goes out when spec
// stops, opening spec's turn by dev.
func TestAnAnswerForAWorkingMemberIsBoxedAndOpensItsNextTurn(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: "dev", From: "lead",
		Asked: map[string]Question{"dev": {Seq: 3, At: at, Text: "push した", Candidates: []string{"lead", "spec"}, Waiting: []string{"lead"}}}}
	answering := agents(map[string]string{"spec": "working", "dev": "working"}, 4)
	answered := agents(map[string]string{"spec": "working", "dev": "done"}, 5)
	got := Step(st, Input{Graph: ring(), Before: answering, Now: answered, Wrote: wrote(map[string]string{"dev": "spec へ"}), At: at.Add(time.Minute)})
	if len(got) != 1 || got[0].Kind != Box || got[0].To != "spec" || got[0].Messages[0].Text != "push した" {
		t.Fatalf("Step = %+v, want dev's text boxed for spec", got)
	}
	specDone := agents(map[string]string{"spec": "done", "dev": "done"}, 6)
	got = Step(st, Input{Graph: ring(), Before: answered, Now: specDone, Wrote: wrote(map[string]string{"spec": "照合した"}), At: at.Add(2 * time.Minute)})
	// spec's own turn, which the outside opened, rests; then the box empties.
	if len(got) != 2 || got[0].Kind != Rested || got[0].From != "spec" || got[1].Kind != Deliver || got[1].To != "spec" || got[1].From != "dev" {
		t.Fatalf("Step = %+v, want spec's turn rested and the box emptied into spec", got)
	}
	if len(st.Pending) != 0 || len(st.Opened["spec"]) != 1 || st.Opened["spec"][0].By != "dev" {
		t.Errorf("state = %+v, want the box empty and dev's opening on spec", st)
	}
}

// TestAnswersThatNameTheHumanNobodyOrNoOneLeaveTheTurnWithTheHuman.
func TestAnswersThatNameTheHumanNobodyOrNoOneLeaveTheTurnWithTheHuman(t *testing.T) {
	cases := []struct {
		answer  string
		wantWhy string
		text    bool
	}{
		{"人間", "", true},
		{"誰にも", WhyNobody, false},
		{"lead か spec", WhyUnclear, true},
		{"", WhyUnclear, true},
	}
	for _, c := range cases {
		st := &State{Team: fixtureSeen("t"), Holder: "dev", From: "lead",
			Asked: map[string]Question{"dev": {Seq: 3, At: at, Text: "結果", Candidates: []string{"lead", "spec"}}}}
		before := agents(map[string]string{"dev": "working"}, 4)
		now := agents(map[string]string{"dev": "done"}, 5)
		got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"dev": c.answer}), At: at.Add(time.Minute)})
		if len(got) != 1 || got[0].Kind != ToHuman || got[0].Why != c.wantWhy || got[0].Route != RouteHerdr {
			t.Errorf("answer %q: Step = %+v, want the human holding it with why %q", c.answer, got, c.wantWhy)
			continue
		}
		if (len(got[0].Messages) > 0) != c.text {
			t.Errorf("answer %q: messages = %+v", c.answer, got[0].Messages)
		}
		if st.Holder != Human || st.From != "dev" || st.Why != c.wantWhy || len(st.Asked) != 0 {
			t.Errorf("answer %q: state = %+v", c.answer, st)
		}
	}
}

// TestASilentUnhandedStopGoesToTheHuman: lead told dev; dev stops without a
// line. Nothing to ask about: the human holds it, and the action names lead
// as waiting.
func TestASilentUnhandedStopGoesToTheHuman(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: "dev", From: "lead", Opened: map[string][]Opening{"dev": {{By: "lead", At: at}}}}
	before := agents(map[string]string{"dev": "working"}, 1)
	now := agents(map[string]string{"dev": "idle"}, 2)
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(nil), At: at.Add(time.Minute)})
	if len(got) != 1 || got[0].Kind != ToHuman || got[0].Why != WhySilent || len(got[0].Waiting) != 1 || got[0].Waiting[0] != "lead" {
		t.Fatalf("Step = %+v, want the human to hold a silent stop, lead waiting", got)
	}
	if st.Holder != Human || st.Why != WhySilent || len(st.Asked) != 0 {
		t.Errorf("state = %+v", st)
	}
}

// TestATellDuringATurnOpensTheNextOne: lead tells dev while dev works on a
// turn the person opened. dev's reply rests (the person's), and lead's tell
// opens dev's next turn, which is asked about when it ends unhanded
// (ac-e5d6, the other way round: the person's turn first).
func TestATellDuringATurnOpensTheNextOne(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"lead": "idle", "dev": "idle"}, 1)
	busy := agents(map[string]string{"lead": "idle", "dev": "working"}, 2)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at}) // the person typed
	_ = Step(st, Input{Graph: ring(), Before: busy, Now: busy, Tells: []Tell{{From: "lead", To: "dev", At: at.Add(30 * time.Second)}}, At: at.Add(30 * time.Second)})
	stopped := agents(map[string]string{"lead": "idle", "dev": "done"}, 3)
	got := Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"dev": "人への答え"}), At: at.Add(time.Minute)})
	if len(got) != 1 || got[0].Kind != Rested {
		t.Fatalf("Step = %+v, want the person's turn to rest; lead's tell is for the next", got)
	}
	if len(st.Opened["dev"]) != 1 {
		t.Fatalf("state = %+v, want lead's opening kept for dev's next turn", st)
	}
	again := agents(map[string]string{"lead": "idle", "dev": "working"}, 4)
	_ = Step(st, Input{Graph: ring(), Before: stopped, Now: again, At: at.Add(2 * time.Minute)})
	done := agents(map[string]string{"lead": "idle", "dev": "done"}, 5)
	got = Step(st, Input{Graph: ring(), Before: again, Now: done, Wrote: wrote(map[string]string{"dev": "lead への答え"}), At: at.Add(3 * time.Minute)})
	if len(got) != 1 || got[0].Kind != Ask || got[0].Waiting[0] != "lead" {
		t.Fatalf("Step = %+v, want dev asked with lead waiting", got)
	}
}

// TestThePersonsInterruptionIsTheirsAndTheWaitingMemberIsStillWaiting: lead
// told dev; the person tells dev too before it starts. dev answers both in one
// turn without a tell: the person's part is theirs, and hergy asks dev about
// lead, who still waits (ac-e5d6).
func TestThePersonsInterruptionIsTheirsAndTheWaitingMemberIsStillWaiting(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"lead": "idle", "dev": "idle"}, 1)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: idle, Tells: []Tell{{From: "lead", To: "dev", At: at}, {To: "dev", At: at.Add(time.Second)}}, At: at.Add(time.Second)})
	busy := agents(map[string]string{"lead": "idle", "dev": "working"}, 2)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at.Add(2 * time.Second)})
	stopped := agents(map[string]string{"lead": "idle", "dev": "done"}, 3)
	got := Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"dev": "人へ: はい。lead へ: 結果はこれ"}), At: at.Add(time.Minute)})
	if len(got) != 1 || got[0].Kind != Ask || len(got[0].Waiting) != 1 || got[0].Waiting[0] != "lead" {
		t.Fatalf("Step = %+v, want dev asked about lead, who waits", got)
	}
}

// TestAWorkersUnopenedTurnIsItsHirers: a worker stops with text and no one
// told it anything hergy saw (the hire's prompt did): it is asked, naming its
// hirer (d-3baa).
func TestAWorkersUnopenedTurnIsItsHirers(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	before := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "scroll", Hirer: "spec", Status: "working", Seq: 2}}
	now := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "scroll", Hirer: "spec", Status: "done", Seq: 3}}
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"scroll": "⑦ を push した"}), At: at})
	if len(got) != 1 || got[0].Kind != Ask || got[0].To != "scroll" || got[0].Waiting[0] != "spec" || got[0].Candidates[0] != "spec" {
		t.Fatalf("Step = %+v, want the worker asked, its hirer waiting", got)
	}
}

func TestStepLeavesABlockedMemberAlone(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: "dev", From: "spec", Opened: map[string][]Opening{"dev": {{By: "spec", At: at}}}}
	before := agents(map[string]string{"dev": "working"}, 1)
	now := agents(map[string]string{"dev": "blocked"}, 2)
	if got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"dev": "許可を待つ"}), At: at}); len(got) != 0 {
		t.Errorf("a blocked member (asking the person) was acted on: %+v", got)
	}
	if st.Holder != "dev" || len(st.Opened["dev"]) != 1 {
		t.Errorf("state = %+v, want dev still holding it, spec still waiting", st)
	}
}

func TestStepSeesTheHumanStartATurn(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human, From: "lead"}
	before := agents(map[string]string{"lead": "idle"}, 1)
	now := agents(map[string]string{"lead": "working"}, 2)
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, At: at})
	if len(got) != 1 || got[0].Kind != Started || got[0].To != "lead" || got[0].From != Human {
		t.Fatalf("Step = %+v, want lead's turn started by the human", got)
	}
	if st.Holder != "lead" || st.From != Human || !st.Started["lead"].Equal(at) {
		t.Errorf("state = %+v", st)
	}
}

func TestLoadSaveAndClearKeepAndForgetTheTurn(t *testing.T) {
	dir := t.TempDir()
	path := Path(dir, "demo")
	st, err := Load(path, fixtureSeen("demo"))
	if err != nil || st.Holder != Human || st.Team.String() != "demo" {
		t.Fatalf("Load of nothing = %+v, %v, want the human holding demo's turn", st, err)
	}
	st.Holder, st.From, st.Since = "lead", Human, at
	st.Pending = map[string][]Message{"spec": {{From: "dev", At: at, Text: "x"}}}
	st.Opened = map[string][]Opening{"dev": {{By: "lead", At: at}}}
	st.Asked = map[string]Question{"spec": {Seq: 2, At: at, Text: "y", Candidates: []string{"lead"}}}
	st.Handed = map[string]int{"dev": 3}
	if err := Save(path, st); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path, fixtureSeen("demo"))
	if err != nil || back.Holder != "lead" || back.Pending["spec"][0].Text != "x" || back.Opened["dev"][0].By != "lead" || back.Asked["spec"].Text != "y" || back.Handed["dev"] != 3 {
		t.Errorf("Load after Save = %+v, %v", back, err)
	}
	if err := WriteTell(TellDir(dir, "demo"), Tell{From: "lead", To: "dev", At: at}); err != nil {
		t.Fatal(err)
	}
	if err := Clear(dir, "demo"); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{path, SignalDir(dir, "demo"), TellDir(dir, "demo")} {
		if _, err := os.Stat(gone); err == nil {
			t.Errorf("%s is still there after Clear", gone)
		}
	}
	if _, err := Load(filepath.Join(dir, "turns", "other.json"), fixtureSeen("other")); err != nil {
		t.Fatal(err)
	}
}

func TestTellsAreTakenOldestFirstAndOnce(t *testing.T) {
	dir := TellDir(t.TempDir(), "demo")
	if got, err := TakeTells(dir); err != nil || len(got) != 0 {
		t.Fatalf("TakeTells(no dir) = %v, %v", got, err)
	}
	for _, tell := range []Tell{{From: "dev", To: "lead", At: at.Add(time.Second)}, {To: "dev", At: at}} {
		if err := WriteTell(dir, tell); err != nil {
			t.Fatal(err)
		}
	}
	got, err := TakeTells(dir)
	if err != nil || len(got) != 2 || got[0].From != "" || got[0].To != "dev" || got[1].From != "dev" {
		t.Fatalf("TakeTells = %+v, %v; want the person's first, then dev's", got, err)
	}
	if again, err := TakeTells(dir); err != nil || len(again) != 0 {
		t.Fatalf("TakeTells again = %+v, %v; want none", again, err)
	}
}

// TestATellFoldedIntoTheRunningTurnIsAskedAboutAfterTheGrace: lead tells spec
// while spec works on the start prompt; spec's harness folds the message into
// the running turn and answers both in it (opencode, 2026-11-09 実機). The turn
// rests at first, and when no next turn begins within OpenGrace, spec is
// asked about lead's tell with that turn's text.
func TestATellFoldedIntoTheRunningTurnIsAskedAboutAfterTheGrace(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"lead": "idle", "spec": "idle"}, 1)
	busy := agents(map[string]string{"lead": "idle", "spec": "working"}, 2)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at}) // the start prompt's turn
	_ = Step(st, Input{Graph: ring(), Before: busy, Now: busy, Tells: []Tell{{From: "lead", To: "spec", At: at.Add(time.Minute)}}, At: at.Add(time.Minute)})
	stopped := agents(map[string]string{"lead": "idle", "spec": "done"}, 3)
	got := Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"spec": "起動しました。\nping"}), At: at.Add(3 * time.Minute)})
	if len(got) != 1 || got[0].Kind != Rested {
		t.Fatalf("Step = %+v, want the turn to rest first", got)
	}
	if _, ok := st.Rested["spec"]; !ok || len(st.Opened["spec"]) != 1 {
		t.Fatalf("state = %+v, want the rest remembered and lead's opening kept", st)
	}
	// Still stopped, grace not over: nothing.
	if early := Step(st, Input{Graph: ring(), Before: stopped, Now: stopped, At: at.Add(3*time.Minute + OpenGrace - time.Second)}); len(early) != 0 {
		t.Fatalf("Step = %+v, want nothing before the grace is over", early)
	}
	got = Step(st, Input{Graph: ring(), Before: stopped, Now: stopped, At: at.Add(3*time.Minute + OpenGrace)})
	if len(got) != 1 || got[0].Kind != Ask || got[0].To != "spec" || got[0].Waiting[0] != "lead" || st.Asked["spec"].Text != "起動しました。\nping" {
		t.Fatalf("Step = %+v, state %+v; want spec asked about lead's tell with the turn's text", got, st)
	}
	if len(st.Rested) != 0 {
		t.Errorf("the rest is still remembered: %+v", st.Rested)
	}

	// A member that begins a new turn within the grace is left to it.
	st = &State{Team: fixtureSeen("t"), Holder: Human}
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at})
	_ = Step(st, Input{Graph: ring(), Before: busy, Now: busy, Tells: []Tell{{From: "lead", To: "spec", At: at.Add(time.Minute)}}, At: at.Add(time.Minute)})
	_ = Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"spec": "起動しました"}), At: at.Add(2 * time.Minute)})
	again := agents(map[string]string{"lead": "idle", "spec": "working"}, 4)
	if got := Step(st, Input{Graph: ring(), Before: stopped, Now: again, At: at.Add(2*time.Minute + time.Second)}); len(got) != 0 || len(st.Rested) != 0 {
		t.Fatalf("Step = %+v, state %+v; want the next turn to take lead's opening, nothing asked", got, st)
	}
}

// TestATellToTheHumanEndsTheTurnWithoutAQuestion: lead told dev; dev writes
// for the person, marks it with hergy tell 人間 and stops. Nothing is asked:
// the turn is the human's, and lead waits no more (d-e601, ac-bc44).
func TestATellToTheHumanEndsTheTurnWithoutAQuestion(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"lead": "idle", "dev": "idle"}, 1)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: idle, Tells: []Tell{{From: "lead", To: "dev", At: at}}, At: at})
	busy := agents(map[string]string{"lead": "idle", "dev": "working"}, 2)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at.Add(time.Second)})
	got := Step(st, Input{Graph: ring(), Before: busy, Now: busy, Tells: []Tell{{From: "dev", To: Human, At: at.Add(30 * time.Second)}}, At: at.Add(30 * time.Second)})
	if len(got) != 1 || got[0].Kind != Told || got[0].From != "dev" || got[0].To != Human {
		t.Fatalf("Step = %+v, want the tell noted, dev -> human", got)
	}
	if st.Holder != "dev" || len(st.Opened[Human]) != 0 {
		t.Fatalf("state = %+v, want dev still holding its turn and no turn opened for the human", st)
	}
	stopped := agents(map[string]string{"lead": "idle", "dev": "done"}, 3)
	got = Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"dev": "人への報告"}), At: at.Add(time.Minute)})
	if len(got) != 1 || got[0].Kind != ToHuman || got[0].From != "dev" || got[0].Why != "" || len(got[0].Waiting) != 1 || got[0].Waiting[0] != "lead" {
		t.Fatalf("Step = %+v, want the turn handed to the human from dev, lead released, nothing asked", got)
	}
	if st.Holder != Human || st.From != "dev" || len(st.Asked) != 0 || len(st.Opened["dev"]) != 0 || len(st.Waits()) != 0 {
		t.Fatalf("state = %+v, want the human holding it from dev, nothing asked, nobody waiting", st)
	}
}

// TestATellToTheHumanInThePersonsOwnTurnRests: the person opened dev's turn;
// dev marks its reply with hergy tell 人間 anyway. The reply rests as before,
// and nothing is remembered as an unanswered rest.
func TestATellToTheHumanInThePersonsOwnTurnRests(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"dev": "idle"}, 1)
	busy := agents(map[string]string{"dev": "working"}, 2)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at}) // the person typed
	_ = Step(st, Input{Graph: ring(), Before: busy, Now: busy, Tells: []Tell{{From: "dev", To: Human, At: at.Add(time.Second)}}, At: at.Add(time.Second)})
	stopped := agents(map[string]string{"dev": "done"}, 3)
	got := Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"dev": "人への答え"}), At: at.Add(time.Minute)})
	if len(got) != 1 || got[0].Kind != Rested {
		t.Fatalf("Step = %+v, want the person's turn to rest", got)
	}
	if st.Holder != Human || len(st.Rested) != 0 || len(st.Told) != 0 {
		t.Fatalf("state = %+v, want the human holding it, no rest kept, the tell forgotten", st)
	}
}

// TestATellToTheHumanAnswersTheQuestion: asked who its text is for, dev runs
// hergy tell 人間 instead of answering with a word. The turn is the human's,
// as if it had answered "人間", and lead waits no more.
func TestATellToTheHumanAnswersTheQuestion(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	idle := agents(map[string]string{"lead": "idle", "dev": "idle"}, 1)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: idle, Tells: []Tell{{From: "lead", To: "dev", At: at}}, At: at})
	busy := agents(map[string]string{"lead": "idle", "dev": "working"}, 2)
	_ = Step(st, Input{Graph: ring(), Before: idle, Now: busy, At: at.Add(time.Second)})
	stopped := agents(map[string]string{"lead": "idle", "dev": "done"}, 3)
	got := Step(st, Input{Graph: ring(), Before: busy, Now: stopped, Wrote: wrote(map[string]string{"dev": "報告"}), At: at.Add(time.Minute)})
	if len(got) != 1 || got[0].Kind != Ask {
		t.Fatalf("Step = %+v, want dev asked", got)
	}
	again := agents(map[string]string{"lead": "idle", "dev": "working"}, 4)
	_ = Step(st, Input{Graph: ring(), Before: stopped, Now: again, At: at.Add(2 * time.Minute)})
	_ = Step(st, Input{Graph: ring(), Before: again, Now: again, Tells: []Tell{{From: "dev", To: Human, At: at.Add(2*time.Minute + time.Second)}}, At: at.Add(2*time.Minute + time.Second)})
	done := agents(map[string]string{"lead": "idle", "dev": "done"}, 5)
	got = Step(st, Input{Graph: ring(), Before: again, Now: done, Wrote: wrote(map[string]string{"dev": "人間に渡した"}), At: at.Add(3 * time.Minute)})
	if len(got) != 1 || got[0].Kind != ToHuman || got[0].From != "dev" || got[0].Why != "" || len(got[0].Waiting) != 1 || got[0].Waiting[0] != "lead" {
		t.Fatalf("Step = %+v, want the turn handed to the human, lead released", got)
	}
	if st.Holder != Human || len(st.Asked) != 0 || len(st.Waits()) != 0 {
		t.Fatalf("state = %+v, want the human holding it, nothing asked, nobody waiting", st)
	}
}

func TestIsHumanReadsTheAnswerWords(t *testing.T) {
	for _, word := range []string{"人間", "human", "Human", " 外側 ", "outside"} {
		if !IsHuman(word) {
			t.Errorf("IsHuman(%q) = false, want true", word)
		}
	}
	for _, word := range []string{"lead", "humans", "", "誰にも"} {
		if IsHuman(word) {
			t.Errorf("IsHuman(%q) = true, want false", word)
		}
	}
}
