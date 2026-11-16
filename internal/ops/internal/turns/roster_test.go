package turns

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

func roster(names ...string) func(string) bool {
	return func(name string) bool { return slices.Contains(names, name) }
}

func TestForgetAbsentAfterTellAndWorkerStop(t *testing.T) {
	st := &State{Team: fixtureSeen("t"), Holder: Human}
	Step(st, Input{Graph: ring(), Tells: []Tell{{From: "lead", To: "worker", At: at}}, At: at})
	st.ForgetAbsent(roster("lead", "spec", "dev", "worker"), at)
	if len(st.Waits()) != 1 {
		t.Fatalf("registered worker was forgotten: %+v", st)
	}
	st.ForgetAbsent(roster("lead", "spec", "dev"), at.Add(time.Second))
	if len(st.Waits()) != 0 || st.Holder != Human || st.From != "worker" || st.Why != WhyUnanswered {
		t.Fatalf("stopped worker remains: %+v", st)
	}
}

func TestForgetAbsentWhenOldStateIsLoaded(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	st := &State{
		Team: fixtureSeen("t"), Holder: "lead", From: Human, Since: at,
		Opened:  map[string][]Opening{"old": {{By: "lead", At: at}}, "lead": {{By: "old", At: at}, {By: Human, At: at}, {By: "spec", At: at}}},
		Started: map[string]time.Time{"old": at, "lead": at},
		Told:    map[string]Handoff{"old": {To: "lead", At: at}, "lead": {To: "old", At: at}},
		Asked:   map[string]Question{"old": {Seq: 1}, "lead": {Candidates: []string{"old", "spec"}, Waiting: []string{"old", "spec"}}},
		Pending: map[string][]Message{"old": {{From: "lead"}}, "lead": {{From: "old"}, {From: "spec", Text: "keep"}}},
		Rested:  map[string]Rest{"old": {Seq: 1}}, Handed: map[string]int{"old": 1},
		Waiting: map[string]Stop{"old": {Seq: 1}}, Signaled: map[string]time.Time{"old": at},
	}
	path := filepath.Join(t.TempDir(), "t.json")
	if err := Save(path, st); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path, fixtureSeen("t"))
	if err != nil {
		t.Fatal(err)
	}
	loaded.ForgetAbsent(roster("lead", "spec"), at.Add(time.Minute))
	want := &State{
		Team: fixtureSeen("t"), Holder: "lead", From: Human, Since: at,
		Opened:  map[string][]Opening{"lead": {{By: Human, At: at}, {By: "spec", At: at}}},
		Started: map[string]time.Time{"lead": at}, Told: map[string]Handoff{},
		Asked:   map[string]Question{"lead": {Candidates: []string{"spec"}, Waiting: []string{"spec"}}},
		Pending: map[string][]Message{"lead": {{From: "spec", Text: "keep"}}},
		Rested:  map[string]Rest{}, Handed: map[string]int{}, Waiting: map[string]Stop{}, Signaled: map[string]time.Time{},
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("retained state = %+v; want %+v", loaded, want)
	}
}

func TestForgetAbsentKeepsRosterWithoutHerdrObservation(t *testing.T) {
	for _, holder := range []string{Human, "lead", "worker"} {
		st := &State{Holder: holder, From: "lead", Since: at, Opened: map[string][]Opening{"worker": {{By: "lead", At: at}}}}
		st.ForgetAbsent(roster("lead", "worker"), at.Add(time.Minute))
		if st.Holder != holder || st.Since != at || len(st.Waits()) != 1 {
			t.Fatalf("roster member forgotten without a herdr observation: %+v", st)
		}
	}
}

func TestWaitsShowsRepeatedTellOnlyOnce(t *testing.T) {
	st := &State{Holder: Human}
	Step(st, Input{Graph: ring(), Tells: []Tell{{From: "lead", To: "dev", At: at}, {From: "spec", To: "dev", At: at}, {From: "lead", To: "dev", At: at}}, At: at})
	if got, want := st.Waits(), []string{"lead → dev", "spec → dev"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Waits() = %v, want %v", got, want)
	}
	if len(st.Opened["dev"]) != 3 {
		t.Fatal("display deduplication changed the openings")
	}
}
