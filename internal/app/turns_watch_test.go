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
	f.w = newTurnWatcher("demo", starterGraph(), herdr.New(f.port), filepath.Join(t.TempDir(), "demo.json"), f.out)
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

// TestTurnsHandsWhatAMemberWroteToItsEdge: builder works, writes, stops; the
// watcher prompts lead with what builder wrote, and the state says lead holds
// the turn from builder. The echo of an earlier prompt and the harness's chrome
// are not in it.
func TestTurnsHandsWhatAMemberWroteToItsEdge(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.port.panes["demo-builder"] = "⏺ 準備できています。\n❯ "
	f.w.tick() // both stopped: their panes are remembered as they stand
	f.set("demo-builder", "working", 2)
	f.port.panes["demo-builder"] = "" // herdr cannot read a working agent's scrollback
	f.w.tick()
	f.port.panes["demo-builder"] = "⏺ 準備できています。\n\n❯ 依頼: テストを書いて\n\n⏺ 書いています…\n\n⏺ テストを 3 件書いた。\n  すべて緑。\n\n✻ Worked for 20s\n❯ "
	f.set("demo-builder", "done", 3)
	f.w.tick()
	if len(f.port.prompts) != 1 {
		t.Fatalf("prompts = %q, want one to lead", f.port.prompts)
	}
	target, text, _ := strings.Cut(f.port.prompts[0], "\n")
	if target != "demo-lead" {
		t.Errorf("prompted %q, want demo-lead", target)
	}
	for _, want := range []string{"builder", "⏺ 書いています…\n\n⏺ テストを 3 件書いた。\n  すべて緑。", "herdr agent read demo-builder"} {
		if !strings.Contains(text, want) {
			t.Errorf("the handoff lacks %q:\n%s", want, text)
		}
	}
	for _, bad := range []string{"準備できています", "Worked for", "❯"} {
		if strings.Contains(text, bad) {
			t.Errorf("the handoff carries %q, which builder did not write this turn:\n%s", bad, text)
		}
	}
	if f.w.state.Holder != "lead" || f.w.state.From != "builder" {
		t.Errorf("state = %+v, want lead holding it from builder", f.w.state)
	}
	if !strings.Contains(f.out.String(), "builder → lead") {
		t.Errorf("the watcher's pane does not say builder → lead:\n%s", f.out.String())
	}
	saved, err := turns.Load(f.w.path, "demo")
	if err != nil || saved.Holder != "lead" {
		t.Errorf("the saved state = %+v, %v", saved, err)
	}
	// lead works and stops: its two edges are the human and builder, and the
	// turn came from builder, so it goes back to builder.
	f.set("demo-lead", "working", 4)
	f.w.tick()
	f.port.panes["demo-lead"] = "❯ hergy より: builder のターンが終わり…\n⏺ 読む…\n\n⏺ 通った。次は段 2。\n❯ "
	f.set("demo-lead", "done", 5)
	f.w.tick()
	if len(f.port.prompts) != 2 || !strings.HasPrefix(f.port.prompts[1], "demo-builder\n") {
		t.Fatalf("prompts = %q, want the second to builder", f.port.prompts)
	}
	if f.w.state.Holder != "builder" {
		t.Errorf("holder = %q, want builder", f.w.state.Holder)
	}
}

// TestTurnsLeavesASilentStopAndAnUndeliveredTurnWithTheHuman: a member that
// wrote nothing is not handed on, and a prompt herdr never sees land (after
// one more try) leaves the turn with the human, with why.
func TestTurnsLeavesASilentStopAndAnUndeliveredTurnWithTheHuman(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.set("demo-builder", "idle", 1)
	f.port.panes["demo-builder"] = "⏺ 前の文\n❯ "
	f.w.tick()
	f.set("demo-builder", "working", 2)
	f.w.tick()
	f.set("demo-builder", "idle", 3)
	f.w.tick()
	if len(f.port.prompts) != 0 || f.w.state.Holder != turns.Human || f.w.state.Why != turns.WhySilent {
		t.Fatalf("a silent stop: prompts = %q, state = %+v", f.port.prompts, f.w.state)
	}
	f.port.stall = true
	f.set("demo-builder", "working", 4)
	f.w.tick()
	f.port.panes["demo-builder"] = "⏺ 前の文\n\n⏺ 書いた\n❯ "
	f.set("demo-builder", "done", 5)
	f.w.tick()
	if f.w.state.Holder != turns.Human || f.w.state.Why != turns.WhyUndelivered {
		t.Errorf("an undelivered turn: state = %+v, want the human holding it", f.w.state)
	}
	if !strings.Contains(f.out.String(), "agent_prompt_stalled") {
		t.Errorf("the watcher's pane does not say why:\n%s", f.out.String())
	}
}

// TestTurnsSeesThePersonStartATurn: a member that goes working without hergy's
// delivery was started by the person; the watcher notes it and holds nothing
// against it.
func TestTurnsSeesThePersonStartATurn(t *testing.T) {
	f := newTurnsFixture(t)
	f.set("demo-lead", "idle", 1)
	f.w.tick()
	f.set("demo-lead", "working", 2)
	f.w.tick()
	if f.w.state.Holder != "lead" || f.w.state.From != turns.Human {
		t.Errorf("state = %+v, want lead holding it from the human", f.w.state)
	}
	if !strings.Contains(f.out.String(), "lead") {
		t.Errorf("the watcher's pane says nothing:\n%s", f.out.String())
	}
}
