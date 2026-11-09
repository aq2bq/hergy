package app

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/herdr"
	"github.com/aq2bq/hergy/internal/protocol"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/turns"
)

// turnsPort is a fake herdr for the watcher: the panes' text by herdr name,
// the prompts it was asked to send, and whether a prompt lands.
type turnsPort struct {
	panes   map[string]string
	prompts []string // "<target>\n<text>"
	stall   bool     // every prompt answers agent_prompt_stalled
}

func (p *turnsPort) Verify(string) error { return nil }

func (p *turnsPort) JSON(args ...string) (map[string]any, error) {
	if len(args) >= 4 && args[0] == "agent" && args[1] == "prompt" {
		if p.stall {
			_, err := protocol.Result(args, `{"error":{"code":"agent_prompt_stalled"}}`, "", 1)
			return nil, err
		}
		p.prompts = append(p.prompts, args[2]+"\n"+args[3])
		return map[string]any{}, nil
	}
	return nil, protocol.Failure(args, "", "unexpected: "+strings.Join(args, " "), 1)
}

func (p *turnsPort) Text(args ...string) (string, error) {
	if len(args) >= 3 && args[0] == "agent" && args[1] == "read" {
		return p.panes[args[2]], nil
	}
	return "", protocol.Failure(args, "", "unexpected: "+strings.Join(args, " "), 1)
}

func humanNodeID() definition.NodeID { return definition.Human{}.ID() }
func memberNodeID(name string) definition.NodeID {
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// starterGraph is hergy new's shape: human -> lead, lead -> builder,
// builder -> lead, lead -> human.
func starterGraph() *definition.Graph {
	e := func(a, b definition.NodeID) definition.Edge { return definition.Edge{From: a, To: b} }
	return definition.NewGraph([]definition.MemberNode{{Name: "lead"}, {Name: "builder"}}, nil, []definition.Edge{
		e(humanNodeID(), memberNodeID("lead")), e(memberNodeID("lead"), memberNodeID("builder")),
		e(memberNodeID("builder"), memberNodeID("lead")), e(memberNodeID("lead"), humanNodeID()),
	})
}

type turnsFixture struct {
	port   *turnsPort
	w      *turnWatcher
	out    *bytes.Buffer
	agents []protocol.Agent
	now    time.Time
}

func newTurnsFixture(t *testing.T) *turnsFixture {
	t.Helper()
	inJapanese(t)
	f := &turnsFixture{port: &turnsPort{panes: map[string]string{}}, out: &bytes.Buffer{}, now: time.Date(2026, 11, 6, 21, 20, 0, 0, time.UTC)}
	dir := t.TempDir()
	f.w = newTurnWatcher("demo", starterGraph(), herdr.New(f.port), filepath.Join(dir, "demo.json"), f.out)
	f.w.tells = turns.TellDir(dir, "demo")
	f.w.signals = turns.SignalDir(dir, "demo")
	f.w.read = func() ([]team.Tab, []protocol.Agent, bool, error) {
		tabs := []team.Tab{{ID: "t1", Label: "demo", Agents: []string{"demo-lead", "demo-builder"}}}
		return tabs, f.agents, true, nil
	}
	f.w.clock = func() time.Time { return f.now }
	if err := f.w.load(); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *turnsFixture) set(name, status string, seq int) {
	for i := range f.agents {
		if f.agents[i].Name == name {
			f.agents[i].Status, f.agents[i].Seq = status, seq
			return
		}
	}
	f.agents = append(f.agents, protocol.Agent{Name: name, Status: status, Seq: seq, TabID: "t1"})
}

// tell records a hergy tell the way the command does, from a member or the
// person ("").
func (f *turnsFixture) tell(t *testing.T, from, to string) {
	t.Helper()
	if err := turns.WriteTell(f.w.tells, turns.Tell{From: from, To: to, At: f.now}); err != nil {
		t.Fatal(err)
	}
}

// later moves the clock and ticks.
func (f *turnsFixture) later(d time.Duration) {
	f.now = f.now.Add(d)
	f.w.tick()
}

// TestTurnsRestsATurnThePersonOpened: nobody told builder anything; it works
// (the person typed) and stops with text. Nothing is sent, the human holds the
// turn, and the strip says the reply was the person's (d-b024, ac-6fe0).
func TestTurnsRestsATurnThePersonOpened(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.port.panes["demo-builder"] = "⏺ 準備できています。\n❯ "
	f.w.tick()
	f.set("demo-builder", "working", 2)
	f.port.panes["demo-builder"] = ""
	f.later(time.Second)
	f.port.panes["demo-builder"] = "⏺ 準備できています。\n\n❯ 起動の文\n\n⏺ 起動しました。lead に要求を進めてほしい。\n❯ "
	f.set("demo-builder", "done", 3)
	f.later(time.Minute)
	if len(f.port.prompts) != 0 {
		t.Fatalf("prompts = %q, want none: the reply is the person's", f.port.prompts)
	}
	if f.w.state.Holder != turns.Human || f.w.state.From != "builder" || len(f.w.state.Asked) != 0 {
		t.Errorf("state = %+v, want the human holding it from builder, nothing asked", f.w.state)
	}
	if !strings.Contains(f.out.String(), "builder は人間に答えて止まりました") {
		t.Errorf("the strip does not say the reply rested:\n%s", f.out.String())
	}
}

// TestTurnsDoesNotAskAMemberThatToldTheHuman: lead tells builder; builder
// writes for the person, runs hergy tell 人間 and stops. No question is sent,
// the strip says the turn went to the human, and nobody waits (d-e601, ac-bc44).
func TestTurnsDoesNotAskAMemberThatToldTheHuman(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.port.panes["demo-builder"] = "⏺ 準備できています。\n❯ "
	f.w.tick()
	f.tell(t, "lead", "builder")
	f.later(time.Second)
	f.set("demo-builder", "working", 2)
	f.port.panes["demo-builder"] = ""
	f.later(time.Second)
	f.tell(t, "builder", turns.Human)
	f.later(time.Second)
	if !strings.Contains(f.out.String(), "builder → 人間: hergy tell で渡しました") {
		t.Errorf("the strip does not note the tell to the human:\n%s", f.out.String())
	}
	f.port.panes["demo-builder"] = "⏺ 準備できています。\n\n❯ lead より: 報告して\n\n⏺ 人へ: 終わりました。\n❯ "
	f.set("demo-builder", "done", 3)
	f.later(time.Minute)
	if len(f.port.prompts) != 0 {
		t.Fatalf("prompts = %q, want none: builder said its text is the person's", f.port.prompts)
	}
	if f.w.state.Holder != turns.Human || f.w.state.From != "builder" || len(f.w.state.Asked) != 0 || len(f.w.state.Waits()) != 0 {
		t.Errorf("state = %+v, want the human holding it from builder, nothing asked, nobody waiting", f.w.state)
	}
	if !strings.Contains(f.out.String(), "builder → 人間: ターンは人間へ渡りました") {
		t.Errorf("the strip does not say the turn went to the human:\n%s", f.out.String())
	}
}

// TestTurnsAsksAnUnhandedTurnAndDeliversAsAnswered: lead tells builder;
// builder works, writes, stops without a tell. The watcher asks builder who
// it is for, naming lead as waiting; builder answers "lead"; the watcher
// delivers what builder wrote to lead, says it was as answered, and lead
// holds the turn from builder (d-80a5, ac-c928).
func TestTurnsAsksAnUnhandedTurnAndDeliversAsAnswered(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.port.panes["demo-builder"] = "⏺ 準備できています。\n❯ "
	f.w.tick()
	f.tell(t, "lead", "builder")
	f.later(time.Second)
	if !strings.Contains(f.out.String(), "lead → builder: hergy tell で渡しました") {
		t.Errorf("the strip does not note the tell:\n%s", f.out.String())
	}
	f.set("demo-builder", "working", 2)
	f.port.panes["demo-builder"] = ""
	f.later(time.Second)
	if len(f.port.prompts) != 0 {
		t.Fatalf("the tell landing was acted on: %q", f.port.prompts)
	}
	f.port.panes["demo-builder"] = "⏺ 準備できています。\n\n❯ lead より: テストを書いて\n\n⏺ 書いています…\n\n⏺ テストを 3 件書いた。\n  すべて緑。\n\n✻ Worked for 20s\n❯ "
	f.set("demo-builder", "done", 3)
	f.later(time.Minute)
	if len(f.port.prompts) != 1 {
		t.Fatalf("prompts = %q, want the question to builder", f.port.prompts)
	}
	target, text, _ := strings.Cut(f.port.prompts[0], "\n")
	if target != "demo-builder" {
		t.Errorf("asked %q, want demo-builder", target)
	}
	for _, want := range []string{"hergy tell", "lead が答えを待っています。", "誰に渡しますか", "lead／人間／誰にも"} {
		if !strings.Contains(text, want) {
			t.Errorf("the question lacks %q:\n%s", want, text)
		}
	}
	if !strings.Contains(f.out.String(), "builder は誰にも渡さずに止まりました。lead が答えを待っています。渡し先を聞いています［herdr の遷移］") {
		t.Errorf("the strip does not say builder is asked:\n%s", f.out.String())
	}
	if q, ok := f.w.state.Asked["builder"]; !ok || !strings.Contains(q.Text, "テストを 3 件書いた。") {
		t.Fatalf("state = %+v, want the question remembered with builder's text", f.w.state)
	}
	// The question lands; builder answers with one word.
	f.set("demo-builder", "working", 4)
	f.port.panes["demo-builder"] = ""
	f.later(time.Second)
	f.port.panes["demo-builder"] = "⏺ テストを 3 件書いた。\n  すべて緑。\n\n✻ Worked for 20s\n❯ hergy より: あなたは書いて止まりましたが…: lead／人間／誰にも\n\n⏺ lead\n❯ "
	f.set("demo-builder", "done", 5)
	f.later(time.Second)
	if len(f.port.prompts) != 2 {
		t.Fatalf("prompts = %q, want the delivery to lead", f.port.prompts)
	}
	target, text, _ = strings.Cut(f.port.prompts[1], "\n")
	if target != "demo-lead" {
		t.Errorf("delivered to %q, want demo-lead", target)
	}
	for _, want := range []string{"builder", "⏺ 書いています…\n\n⏺ テストを 3 件書いた。\n  すべて緑。", "herdr agent read demo-builder", "hergy tell"} {
		if !strings.Contains(text, want) {
			t.Errorf("the handoff lacks %q:\n%s", want, text)
		}
	}
	for _, bad := range []string{"準備できています", "Worked for", "❯", "誰に渡しますか"} {
		if strings.Contains(text, bad) {
			t.Errorf("the handoff carries %q, which builder did not write that turn:\n%s", bad, text)
		}
	}
	if !strings.Contains(f.out.String(), "builder → lead: 渡しました（") || !strings.Contains(f.out.String(), "［本人の答え］") {
		t.Errorf("the strip does not say the delivery was as answered:\n%s", f.out.String())
	}
	if f.w.state.Holder != "lead" || f.w.state.From != "builder" || len(f.w.state.Asked) != 0 {
		t.Errorf("state = %+v, want lead holding it from builder", f.w.state)
	}
	saved, err := turns.Load(f.w.path, "demo")
	if err != nil || saved.Holder != "lead" || len(saved.Opened["lead"]) != 1 {
		t.Errorf("the saved state = %+v, %v; want lead holding it, opened by builder", saved, err)
	}
}

// TestTurnsLeavesAMemberThatToldAlone: lead tells builder; builder tells lead
// back during its turn and stops. Nothing is asked, nothing is sent: the tell
// did it (d-8b4e).
func TestTurnsLeavesAMemberThatToldAlone(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.w.tick()
	f.tell(t, "lead", "builder")
	f.later(time.Second)
	f.set("demo-builder", "working", 2)
	f.later(time.Second)
	f.tell(t, "builder", "lead")
	f.later(time.Second)
	f.port.panes["demo-builder"] = "⏺ lead に渡した。\n❯ "
	f.set("demo-builder", "done", 3)
	f.later(time.Second)
	if len(f.port.prompts) != 0 || len(f.w.state.Asked) != 0 {
		t.Fatalf("prompts = %q, state = %+v; want nothing asked", f.port.prompts, f.w.state)
	}
	if f.w.state.Holder != "lead" || f.w.state.From != "builder" {
		t.Errorf("state = %+v, want lead holding it from builder", f.w.state)
	}
}

// TestTurnsAnswersThatNameNobodyOrNoOneLeaveTheTurnWithTheHuman.
func TestTurnsAnswersThatNameNobodyOrNoOneLeaveTheTurnWithTheHuman(t *testing.T) {
	for answer, want := range map[string]struct{ why, line string }{
		"誰にも":       {turns.WhyNobody, "builder の答えは「誰にも」"},
		"lead か 人間": {turns.WhyUnclear, "builder の答えから渡し先が決まりません"},
	} {
		f := newTurnsFixture(t)
		f.w.state.Asked = map[string]turns.Question{"builder": {Seq: 3, At: f.now, Text: "結果", Candidates: []string{"lead"}, Waiting: []string{"lead"}}}
		f.set("demo-lead", "idle", 1)
		f.set("demo-builder", "working", 4)
		f.w.tick()
		f.port.panes["demo-builder"] = "⏺ " + answer + "\n❯ "
		f.set("demo-builder", "done", 5)
		f.later(time.Second)
		if len(f.port.prompts) != 0 || f.w.state.Holder != turns.Human || f.w.state.Why != want.why {
			t.Errorf("answer %q: prompts = %q, state = %+v", answer, f.port.prompts, f.w.state)
		}
		if !strings.Contains(f.out.String(), want.line) {
			t.Errorf("answer %q: the strip lacks %q:\n%s", answer, want.line, f.out.String())
		}
	}
}

// TestTurnsLeavesASilentUnaskableAndUndeliveredTurnWithTheHuman: a member
// that wrote nothing while lead waited is not asked; one that cannot be asked
// (the prompt never lands) and one whose answer's delivery never lands leave
// the turn with the human, with why.
func TestTurnsLeavesASilentUnaskableAndUndeliveredTurnWithTheHuman(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.port.panes["demo-builder"] = "⏺ 前の文\n❯ "
	f.w.tick()
	f.tell(t, "lead", "builder")
	f.set("demo-builder", "working", 2)
	f.later(time.Second)
	f.set("demo-builder", "idle", 3)
	f.later(time.Second)
	if len(f.port.prompts) != 0 || f.w.state.Holder != turns.Human || f.w.state.Why != turns.WhySilent {
		t.Fatalf("a silent stop: prompts = %q, state = %+v", f.port.prompts, f.w.state)
	}
	if !strings.Contains(f.out.String(), "builder は 1 行も書かずに止まりました。lead が答えを待っています。") {
		t.Errorf("the strip does not name lead as waiting:\n%s", f.out.String())
	}
	// Asked, but the question never lands.
	f.port.stall = true
	f.tell(t, "lead", "builder")
	f.set("demo-builder", "working", 4)
	f.later(time.Second)
	f.port.panes["demo-builder"] = "⏺ 前の文\n\n⏺ 書いた\n❯ "
	f.set("demo-builder", "done", 5)
	f.later(time.Second)
	if f.w.state.Holder != turns.Human || f.w.state.Why != turns.WhyUnanswered || len(f.w.state.Asked) != 0 {
		t.Errorf("an unaskable member: state = %+v, want the human holding it", f.w.state)
	}
	if !strings.Contains(f.out.String(), "builder に聞けませんでした（") || !strings.Contains(f.out.String(), "agent_prompt_stalled") {
		t.Errorf("the strip does not say why:\n%s", f.out.String())
	}
	// Answered "lead", but the delivery never lands.
	f.port.stall = false
	f.tell(t, "lead", "builder")
	f.set("demo-builder", "working", 6)
	f.later(time.Second)
	f.port.panes["demo-builder"] = "⏺ 書いた\n\n⏺ また書いた\n❯ "
	f.set("demo-builder", "done", 7)
	f.later(time.Second)
	if len(f.w.state.Asked) != 1 {
		t.Fatalf("state = %+v, want builder asked", f.w.state)
	}
	f.port.stall = true
	f.set("demo-builder", "working", 8)
	f.later(time.Second)
	f.port.panes["demo-builder"] = "⏺ また書いた\n\n⏺ lead\n❯ "
	f.set("demo-builder", "done", 9)
	f.later(time.Second)
	if f.w.state.Holder != turns.Human || f.w.state.Why != turns.WhyUndelivered {
		t.Errorf("an undelivered answer: state = %+v, want the human holding it", f.w.state)
	}
}

// TestTurnsSeesThePersonStartATurn: a member that goes working without a tell
// or a delivery was started by the person; the watcher notes it and holds
// nothing against it.
func TestTurnsSeesThePersonStartATurn(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.w.tick()
	f.set("demo-lead", "working", 2)
	f.w.tick()
	if f.w.state.Holder != "lead" || f.w.state.From != turns.Human {
		t.Errorf("state = %+v, want lead holding it from the human", f.w.state)
	}
	if !strings.Contains(f.out.String(), "人間が lead のターンを始めました") {
		t.Errorf("the watcher's pane says nothing:\n%s", f.out.String())
	}
}

// TestTurnsActsOnTheSignalBeforeHerdrSeesTheStop: builder's harness says the
// turn ended while herdr still says working; the watcher asks builder at once
// with the signal's text, says it came as a signal, and the stop herdr then
// sees acts on nothing again (r-378e, d-ef1d, d-5f1a).
func TestTurnsActsOnTheSignalBeforeHerdrSeesTheStop(t *testing.T) {
	f := newTurnsFixture(t)
	f.w.wired["builder"] = true
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.w.tick()
	f.tell(t, "lead", "builder")
	f.set("demo-builder", "working", 2)
	f.later(time.Second)
	if err := turns.WriteSignal(f.w.signals, turns.Signal{Member: "builder", At: f.now, Text: "テストを 3 件書いた。\nすべて緑。", Source: "claude"}); err != nil {
		t.Fatal(err)
	}
	f.later(time.Second)
	if len(f.port.prompts) != 1 || !strings.HasPrefix(f.port.prompts[0], "demo-builder\n") {
		t.Fatalf("prompts = %q, want the question to builder from the signal", f.port.prompts)
	}
	if f.w.state.Asked["builder"].Text != "テストを 3 件書いた。\nすべて緑。" {
		t.Errorf("state = %+v, want the signal's text waiting", f.w.state)
	}
	if !strings.Contains(f.out.String(), "渡し先を聞いています［合図: claude］") {
		t.Errorf("the strip does not say the signal ended it:\n%s", f.out.String())
	}
	// herdr sees the stop a second later: the pane is not read, nothing more is sent.
	f.port.panes["demo-builder"] = "⏺ ペインには別の文字\n❯ "
	f.set("demo-builder", "done", 3)
	f.later(time.Second)
	if len(f.port.prompts) != 1 {
		t.Errorf("the stop the signal ended was acted on again: %q", f.port.prompts)
	}
	if f.w.state.Handed["builder"] != 3 {
		t.Errorf("state = %+v, want builder's stop 3 marked", f.w.state)
	}
}

// TestTurnsWaitsForAWiredMembersSignalThenReadsThePane: a wired member's stop
// is not acted on from the pane at once; with no signal in SignalGrace the pane
// is read and the line says so.
func TestTurnsWaitsForAWiredMembersSignalThenReadsThePane(t *testing.T) {
	f := newTurnsFixture(t)
	f.w.wired["builder"] = true
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.port.panes["demo-builder"] = "❯ "
	f.w.tick()
	f.tell(t, "lead", "builder")
	f.set("demo-builder", "working", 2)
	f.port.panes["demo-builder"] = ""
	f.later(time.Second)
	f.port.panes["demo-builder"] = "⏺ ペインから読んだ本文\n❯ "
	f.set("demo-builder", "done", 3)
	f.later(time.Second)
	if len(f.port.prompts) != 0 {
		t.Fatalf("a wired member's stop was acted on before its signal could come: %q", f.port.prompts)
	}
	f.later(turns.SignalGrace)
	if len(f.port.prompts) != 1 || !strings.Contains(f.w.state.Asked["builder"].Text, "ペインから読んだ本文") {
		t.Fatalf("prompts = %q, state = %+v; want builder asked with the pane's text after the grace", f.port.prompts, f.w.state)
	}
	if !strings.Contains(f.out.String(), "［合図が 5 秒来ないので herdr の遷移］") {
		t.Errorf("the watcher's pane does not say the signal never came:\n%s", f.out.String())
	}
}
