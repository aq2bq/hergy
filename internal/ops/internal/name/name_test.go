package name_test

import (
	"encoding"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/name"
)

func TestPlainNames(t *testing.T) {
	parsers := map[string]func(string) error{
		"new team":       func(s string) error { _, e := name.NewTeamName(s); return e },
		"team":           func(s string) error { _, e := name.ParseTeamName(s); return e },
		"new definition": func(s string) error { _, e := name.NewDefinitionName(s); return e },
		"member":         func(s string) error { _, e := name.ParseMemberName(s); return e },
		"worker":         func(s string) error { _, e := name.ParseWorkerName(s); return e },
		"role":           func(s string) error { _, e := name.ParseRoleName(s); return e },
		"profile":        func(s string) error { _, e := name.ParseProfileName(s); return e },
	}
	for kind, parse := range parsers {
		t.Run(kind, func(t *testing.T) {
			for _, valid := range []string{"a", "a-b_c0", strings.Repeat("a", 25)} {
				if err := parse(valid); err != nil {
					t.Errorf("%q: %v", valid, err)
				}
			}
			for _, invalid := range []string{"", "A", "1a", "a b", "a/b", "aあ", "a\n", strings.Repeat("a", 33)} {
				var problem *name.Problem
				if err := parse(invalid); !errors.As(err, &problem) || problem.Value != invalid {
					t.Errorf("%q: %v", invalid, err)
				}
			}
			err := parse(strings.Repeat("a", 32))
			if (err != nil) != strings.HasPrefix(kind, "new ") {
				t.Errorf("32 characters: %v", err)
			}
		})
	}
}

func TestPurpose(t *testing.T) {
	for _, value := range []string{"実装", "two words", " 長い目的 ", strings.Repeat("a", 40)} {
		got, err := name.ParsePurpose(value)
		if err != nil || got.String() != value {
			t.Errorf("%q: %v, %v", value, got, err)
		}
	}
	for _, value := range []string{"", " \t\n", "a,b", "a、b"} {
		if _, err := name.ParsePurpose(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestHerdrCompositionAndResolution(t *testing.T) {
	a, _ := name.ParseTeamName("a")
	ab, _ := name.ParseTeamName("a-b")
	member, _ := name.ParseMemberName("c-d")
	worker, _ := name.ParseWorkerName("build")
	h, err := ab.Member(member)
	if err != nil || h.String() != "a-b-c-d" {
		t.Fatalf("member: %v, %v", h, err)
	}
	w, err := ab.Worker(worker)
	if err != nil || w.String() != "a-b-build" {
		t.Fatalf("worker: %v, %v", w, err)
	}
	watcher := ab.Watcher()
	if watcher.String() != "a-b-turns" {
		t.Fatal(watcher)
	}
	for _, teams := range [][]name.Seen{{fixtureObservation(a.String()), fixtureObservation(ab.String())}, {fixtureObservation(ab.String()), fixtureObservation(a.String())}} {
		team, short, ok := name.Resolve(fixtureObservation(h.String()), teams)
		if !ok || team.String() != ab.String() || short.String() != "c-d" {
			t.Errorf("resolved: %v %q %v", team, short, ok)
		}
	}
	for _, value := range []string{"a", "other-lead", "A-lead"} {
		if _, _, ok := name.Resolve(fixtureObservation(value), []name.Seen{fixtureObservation(a.String()), fixtureObservation(ab.String())}); ok {
			t.Errorf("resolved %q", value)
		}
	}
	long, _ := name.ParseTeamName(strings.Repeat("a", 25))
	six, _ := name.ParseMemberName("aaaaaa")
	seven, _ := name.ParseMemberName("aaaaaaa")
	if _, err := long.Member(six); err != nil {
		t.Fatal(err)
	}
	if _, err := long.Member(seven); err == nil {
		t.Fatal("accepted 33 characters")
	}
	if _, err := a.Member(name.MemberName{}); err == nil {
		t.Fatal("accepted absent member")
	}
	if _, err := (name.TeamName{}).Member(member); err == nil {
		t.Fatal("accepted absent team")
	}
}

func TestSavedStrings(t *testing.T) {
	team, _ := name.ParseTeamName("demo")
	member, _ := name.ParseMemberName("lead")
	herdr, _ := team.Member(member)
	purpose, _ := name.ParsePurpose("two words")
	for _, pair := range []struct {
		value interface {
			encoding.TextMarshaler
			String() string
		}
		into encoding.TextUnmarshaler
		want string
	}{
		{team, new(name.TeamName), "demo"},
		{member, new(name.MemberName), "lead"},
		{herdr, new(name.HerdrName), "demo-lead"},
		{purpose, new(name.Purpose), "two words"},
		{name.DefinitionName{}, new(name.DefinitionName), ""},
		{name.WorkerName{}, new(name.WorkerName), ""},
		{name.RoleName{}, new(name.RoleName), ""},
		{name.ProfileName{}, new(name.ProfileName), ""},
	} {
		if got := pair.value.String(); got != pair.want {
			t.Fatalf("String() = %q, want %q", got, pair.want)
		}
		text, err := pair.value.MarshalText()
		if err != nil || string(text) != pair.want {
			t.Fatalf("text %q: %v", text, err)
		}
		data, err := json.Marshal(pair.value)
		want, _ := json.Marshal(pair.want)
		if err != nil || string(data) != string(want) {
			t.Fatalf("JSON %s: %v", data, err)
		}
		if err := json.Unmarshal(data, pair.into); err != nil {
			t.Fatal(err)
		}
		again, err := json.Marshal(pair.into)
		if err != nil || string(again) != string(data) {
			t.Fatalf("round trip: %s %v", again, err)
		}
	}
}

func TestResolveKeepsUnmatchedObservation(t *testing.T) {
	team, _ := name.ParseTeamName("demo")
	for _, raw := range []string{"other-lead", "human", "", "demo-"} {
		owner, short, ok := name.Resolve(fixtureObservation(raw), []name.Seen{fixtureObservation(team.String())})
		if ok || owner.String() != "" || short.String() != raw {
			t.Errorf("%q: %v %q %v", raw, owner, short, ok)
		}
	}
	// Observed labels retain the old permissive reading; validation happens when
	// assigning names, not when deciding which team's existing pane to display.
	for _, raw := range []string{"demo-1", "demo-" + strings.Repeat("a", 32)} {
		owner, short, ok := name.Resolve(fixtureObservation(raw), []name.Seen{fixtureObservation(team.String())})
		if !ok || owner.String() != team.String() || short.String() != strings.TrimPrefix(raw, "demo-") {
			t.Errorf("%q: %v %q %v", raw, owner, short, ok)
		}
	}
}

func TestExistingDefinitionNamesKeepDirectorySpelling(t *testing.T) {
	for _, raw := range []string{"Upper", strings.Repeat("a", 33), ""} {
		n, err := name.ParseDefinitionName(raw)
		if err != nil || n.String() != raw {
			t.Fatalf("%q: %v %v", raw, n, err)
		}
	}
}

func TestObservedNamesKeepLegacyTargets(t *testing.T) {
	for _, raw := range []string{"Upper-worker", "legacy-1", "a-" + strings.Repeat("b", 40)} {
		var target name.Target = fixtureObservation(raw)
		if target.String() != raw {
			t.Fatal(target)
		}
	}
	for _, raw := range []string{"worker:Upper/1", "worker:team/a/b"} {
		observed := fixtureObservation(raw)
		worker, ok := observed.WorkerTab()
		if !ok || worker.Label() != observed {
			t.Fatalf("lost legacy label %q", raw)
		}
	}
	team := name.TeamFromObservation(fixtureObservation("Upper"))
	if _, ok := team.Model(); ok || team.Observation().String() != "Upper" {
		t.Fatal(team)
	}
	checked := name.TeamFromObservation(fixtureObservation("demo"))
	if model, ok := checked.Model(); !ok || model.String() != "demo" {
		t.Fatal(checked)
	}
	long, _ := name.ParseTeamName(strings.Repeat("a", 32))
	if got := long.Watcher().String(); got != strings.Repeat("a", 32)+"-turns" {
		t.Fatal(got)
	}
}
