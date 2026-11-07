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
// still says working; the turn is handed from the signal's text, and the stop
// herdr then sees is that turn's, not handed again (d-ef1d, d-5f1a).
func TestASignalEndsTheTurnBeforeHerdrSeesTheStop(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec"}
	working := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "working", Seq: 2}}
	sig := Signal{Member: "dev", At: at, Text: "d6465cd を積んだ。", Source: "claude"}
	got := Step(st, Input{Graph: ring(), Before: working, Now: working, Signals: []Signal{sig}, Wired: wired("dev"), At: at,
		Wrote: wrote(map[string]string{"dev": "ペインの文字は使わない"})})
	if len(got) != 1 || got[0].Kind != Deliver || got[0].To != "spec" || got[0].Messages[0].Text != sig.Text {
		t.Fatalf("Step = %+v, want dev's signal delivered to spec", got)
	}
	if got[0].Route != RouteSignal || got[0].Source != "claude" {
		t.Errorf("route = %q %q, want the signal from claude", got[0].Route, got[0].Source)
	}
	if _, ok := st.Signaled["dev"]; !ok {
		t.Fatalf("the signal before the stop is not remembered: %+v", st)
	}
	// herdr sees the stop a second later: nothing more is handed.
	stopped := []Agent{{Name: "spec", Status: "working", Seq: 3}, {Name: "dev", Status: "done", Seq: 3}}
	if again := Step(st, Input{Graph: ring(), Before: working, Now: stopped, Wired: wired("dev"), At: at.Add(time.Second),
		Wrote: wrote(map[string]string{"dev": "x"})}); len(again) != 0 {
		t.Errorf("the stop the signal already ended was handed again: %+v", again)
	}
	if st.Handed["dev"] != 3 || len(st.Signaled) != 0 || len(st.Waiting) != 0 {
		t.Errorf("state = %+v, want dev's stop 3 marked handed and nothing waiting", st)
	}
}

// TestASignalAfterThePaneWasHandedIsDropped: herdr saw an unwired member stop
// and the pane was handed; a signal for the same stop hands nothing twice.
func TestASignalAfterThePaneWasHandedIsDropped(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec"}
	before := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "working", Seq: 2}}
	now := []Agent{{Name: "spec", Status: "idle", Seq: 1}, {Name: "dev", Status: "done", Seq: 3}}
	got := Step(st, Input{Graph: ring(), Before: before, Now: now, Wrote: wrote(map[string]string{"dev": "積んだ"}), At: at})
	if len(got) != 1 || got[0].Kind != Deliver || got[0].Route != RouteHerdr {
		t.Fatalf("Step = %+v, want one delivery from the pane", got)
	}
	late := Step(st, Input{Graph: ring(), Before: now, Now: now, Signals: []Signal{{Member: "dev", At: at, Text: "積んだ", Source: "codex"}}, At: at.Add(2 * time.Second)})
	if len(late) != 1 || late[0].Kind != Dropped || late[0].From != "dev" {
		t.Fatalf("Step = %+v, want the late signal dropped", late)
	}
}

// TestAWiredStopWaitsForItsSignalThenReadsThePane: a wired member's stop is
// not handed from the pane at once; the signal that comes in time hands it,
// and one that never comes leaves the pane to be read after SignalGrace.
func TestAWiredStopWaitsForItsSignalThenReadsThePane(t *testing.T) {
	st := &State{Team: "t", Holder: "dev", From: "spec"}
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
	if len(got) != 1 || got[0].Kind != Deliver || got[0].Messages[0].Text != "合図から" || got[0].Route != RouteSignal {
		t.Fatalf("Step = %+v, want the signal's text delivered", got)
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
	st = &State{Team: "t", Holder: "dev", From: "spec"}
	in = Input{Graph: ring(), Before: before, Now: now, Wired: wired("dev"), Wrote: wrote(map[string]string{"dev": "ペインから"}), At: at}
	_ = Step(st, in)
	in.Before, in.At = now, at.Add(SignalGrace-time.Second)
	if early := Step(st, in); len(early) != 0 {
		t.Fatalf("Step = %+v, want nothing before the grace is over", early)
	}
	in.At = at.Add(SignalGrace)
	got = Step(st, in)
	if len(got) != 1 || got[0].Kind != Deliver || got[0].Messages[0].Text != "ペインから" || got[0].Route != RouteLate {
		t.Fatalf("Step = %+v, want the pane handed late", got)
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
