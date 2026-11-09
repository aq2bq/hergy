package turns

import (
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func wired(names ...string) func(string) bool {
	return func(name string) bool { return slices.Contains(names, name) }
}

// TestASignalEndsTheTurnBeforeHerdrSeesTheStop: the harness speaks while herdr
// still says working; the turn is acted on with the signal's text (dev, whose
// turn spec opened, is asked), and the stop herdr then sees is that turn's,
// not acted on again (d-ef1d, d-5f1a).
func TestASignalEndsTheTurnBeforeHerdrSeesTheStop(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec", Opened: map[string][]Opening{"dev": {{By: "spec", At: at.Add(-time.Minute)}}}}
	working := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "working", Seq: 2}}
	sig := Signal{Member: "dev", At: at, Text: "d6465cd を積んだ。", Source: "claude"}
	got := Step(st, Input{Graph: ring(), Before: working, Now: working, Signals: []Signal{sig}, Wired: wired("dev"), At: at,
		Wrote: wrote(map[string]string{"dev": "ペインの文字は使わない"})})
	if len(got) != 1 || got[0].Kind != Ask || got[0].To != "dev" || st.Asked["dev"].Text != sig.Text {
		t.Fatalf("Step = %+v, state %+v; want dev asked about the signal's text", got, st)
	}
	if got[0].Route != RouteSignal || got[0].Source != "claude" {
		t.Errorf("route = %q %q, want the signal from claude", got[0].Route, got[0].Source)
	}
	if _, ok := st.Signaled["dev"]; !ok {
		t.Fatalf("the signal before the stop is not remembered: %+v", st)
	}
	// herdr sees the stop a second later: nothing more is handed.
	stopped := []Agent{{Name: "spec", Status: "idle", Seq: 3}, {Name: "dev", Status: "done", Seq: 3}}
	if again := Step(st, Input{Graph: ring(), Before: working, Now: stopped, Wired: wired("dev"), At: at.Add(time.Second),
		Wrote: wrote(map[string]string{"dev": "x"})}); len(again) != 0 {
		t.Errorf("the stop the signal already ended was acted on again: %+v", again)
	}
	if st.Handed["dev"] != 3 || len(st.Signaled) != 0 || len(st.Waiting) != 0 {
		t.Errorf("state = %+v, want dev's stop 3 marked handed and nothing waiting", st)
	}
}

// TestASignalAfterThePaneWasHandedIsDropped: herdr saw an unwired member stop
// and the pane was acted on; a signal for the same stop acts twice on nothing.
func TestASignalAfterThePaneWasHandedIsDropped(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec", Opened: map[string][]Opening{"dev": {{By: "spec", At: at.Add(-time.Minute)}}}}
	before := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "working", Seq: 2}}
	now := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "done", Seq: 3}}
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"dev": "積んだ"}), At: at})
	if len(got) != 1 || got[0].Kind != Ask || got[0].Route != RouteHerdr {
		t.Fatalf("Step = %+v, want dev asked from the pane", got)
	}
	late := Step(st, Input{Graph: ring(), Before: now, Now: now, Signals: []Signal{{Member: "dev", At: at, Text: "積んだ", Source: "codex"}}, At: at.Add(2 * time.Second)})
	if len(late) != 1 || late[0].Kind != Dropped || late[0].From != "dev" {
		t.Fatalf("Step = %+v, want the late signal dropped", late)
	}
}

// TestAWiredStopWaitsForItsSignalThenReadsThePane: a wired member's stop is
// not acted on from the pane at once; the signal that comes in time ends it,
// and one that never comes leaves the pane to be read after SignalGrace.
func TestAWiredStopWaitsForItsSignalThenReadsThePane(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec", Opened: map[string][]Opening{"dev": {{By: "spec", At: at.Add(-time.Minute)}}}}
	before := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "working", Seq: 2}}
	now := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "done", Seq: 3}}
	in := Input{Graph: ring(), Before: before, Now: now, Wired: wired("dev"), Wrote: wrote(map[string]string{"dev": "ペインから"}), At: at}
	if got := Step(st, in); len(got) != 0 {
		t.Fatalf("Step = %+v, want the wired stop to wait", got)
	}
	if st.Waiting["dev"].Seq != 3 {
		t.Fatalf("state = %+v, want dev's stop 3 waiting", st)
	}
	// The signal comes two seconds later: handed from it.
	in.Before, in.At = now, at.Add(2*time.Second)
	in.Signals = []Signal{{Member: "dev", At: in.At, Text: "合図から", Source: "opencode"}}
	got := Step(st, in)
	if len(got) != 1 || got[0].Kind != Ask || st.Asked["dev"].Text != "合図から" || got[0].Route != RouteSignal {
		t.Fatalf("Step = %+v, state %+v; want dev asked with the signal's text", got, st)
	}
	if len(st.Waiting) != 0 || st.Handed["dev"] != 3 {
		t.Errorf("state = %+v, want nothing waiting and stop 3 handed", st)
	}
	// Nothing more for that stop, grace or no grace.
	in.Signals, in.At = nil, at.Add(time.Minute)
	if again := Step(st, in); len(again) != 0 {
		t.Errorf("handed again: %+v", again)
	}

	// Without a signal: the pane is read once SignalGrace has passed.
	st = &State{Team: "t", Holder: "dev", From: "spec", Opened: map[string][]Opening{"dev": {{By: "spec", At: at.Add(-time.Minute)}}}}
	in = Input{Graph: ring(), Before: before, Now: now, Wired: wired("dev"), Wrote: wrote(map[string]string{"dev": "ペインから"}), At: at}
	_ = Step(st, in)
	in.Before, in.At = now, at.Add(SignalGrace-time.Second)
	if early := Step(st, in); len(early) != 0 {
		t.Fatalf("Step = %+v, want nothing before the grace is over", early)
	}
	in.At = at.Add(SignalGrace)
	got = Step(st, in)
	if len(got) != 1 || got[0].Kind != Ask || st.Asked["dev"].Text != "ペインから" || got[0].Route != RouteLate {
		t.Fatalf("Step = %+v, state %+v; want dev asked from the pane, late", got, st)
	}
	if len(st.Waiting) != 0 || st.Handed["dev"] != 3 {
		t.Errorf("state = %+v", st)
	}
}

// TestAWaitingStopIsForgottenWhenTheAgentMovesOn: the person typed in the pane
// while the stop waited; that turn is gone and the new one is the person's.
func TestAWaitingStopIsForgottenWhenTheAgentMovesOn(t *testing.T) {
	st := &State{Team: "t", Holder: "spec", From: "dev"}
	before := []Agent{{Name: "dev", Status: "working", Seq: 2}}
	stopped := []Agent{{Name: "dev", Status: "done", Seq: 3}}
	_ = Step(st, Input{Graph: ring(), Before: before, Now: stopped, Wired: wired("dev"), At: at})
	again := []Agent{{Name: "dev", Status: "working", Seq: 4}}
	got := Step(st, Input{Graph: ring(), Before: stopped, Now: again, Wired: wired("dev"), At: at.Add(time.Second)})
	if len(got) != 1 || got[0].Kind != Started || len(st.Waiting) != 0 {
		t.Fatalf("Step = %+v, state %+v; want the person's start and nothing waiting", got, st)
	}
}

func TestSignalsAreTakenOldestFirstAndOnce(t *testing.T) {
	dir := SignalDir(t.TempDir(), "demo")
	if got, err := TakeSignals(dir); err != nil || len(got) != 0 {
		t.Fatalf("TakeSignals(no dir) = %v, %v", got, err)
	}
	for i, sig := range []Signal{
		{Member: "lead", At: at.Add(2 * time.Second), Text: "second", Source: "claude"},
		{Member: "dev", At: at, Text: "first", Source: "codex"},
	} {
		if err := WriteSignal(dir, sig); err != nil {
			t.Fatalf("WriteSignal %d: %v", i, err)
		}
	}
	got, err := TakeSignals(dir)
	if err != nil || len(got) != 2 || got[0].Text != "first" || got[1].Text != "second" || got[1].Source != "claude" {
		t.Fatalf("TakeSignals = %+v, %v; want first then second", got, err)
	}
	if again, err := TakeSignals(dir); err != nil || len(again) != 0 {
		t.Fatalf("TakeSignals again = %+v, %v; want none", again, err)
	}
	if _, err := TakeSignals(filepath.Join(dir, "nope")); err != nil {
		t.Fatal(err)
	}
}

// TestASignalWhileWorkingBeginsTheNextTurn: lead's harness ends a turn while
// herdr still says working (the queued message started the next turn with no
// stop herdr saw). A tell that came during the ended turn opens the next one,
// which is asked about when it ends unhanded (2026-11-09 実機).
func TestASignalWhileWorkingBeginsTheNextTurn(t *testing.T) {
	st := &State{Team: "t", Holder: Human, Started: map[string]time.Time{"lead": at}}
	working := []Agent{{Name: "lead", Status: "working", Seq: 2}, {Name: "dev", Status: "idle", Seq: 1}}
	// dev tells lead during lead's turn; the turn then ends by signal.
	_ = Step(st, Input{Graph: ring(), Before: working, Now: working, Tells: []Tell{{From: "dev", To: "lead", At: at.Add(30 * time.Second)}}, At: at.Add(30 * time.Second)})
	got := Step(st, Input{Graph: ring(), Before: working, Now: working, Wired: wired("lead"), At: at.Add(time.Minute),
		Signals: []Signal{{Member: "lead", At: at.Add(time.Minute), Text: "起動しました", Source: "claude"}}})
	if len(got) != 1 || got[0].Kind != Rested {
		t.Fatalf("Step = %+v, want the start-up reply to rest", got)
	}
	// herdr never saw a stop; the next signal ends the turn dev opened.
	got = Step(st, Input{Graph: ring(), Before: working, Now: working, Wired: wired("lead"), At: at.Add(2 * time.Minute),
		Signals: []Signal{{Member: "lead", At: at.Add(2 * time.Minute), Text: "dev への答え", Source: "claude"}}})
	if len(got) != 1 || got[0].Kind != Ask || got[0].To != "lead" || len(got[0].Waiting) != 1 || got[0].Waiting[0] != "dev" {
		t.Fatalf("Step = %+v, want lead asked with dev waiting", got)
	}
}
