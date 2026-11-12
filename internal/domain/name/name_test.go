package name_test

import (
	"encoding"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/domain/name"
)

func TestPlainNames(t *testing.T) {
	parsers := map[string]func(string) error{
		"new team":       func(s string) error { _, e := name.NewTeamName(s); return e },
		"team":           func(s string) error { _, e := name.ParseTeamName(s); return e },
		"new definition": func(s string) error { _, e := name.NewDefinitionName(s); return e },
		"definition":     func(s string) error { _, e := name.ParseDefinitionName(s); return e },
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
	watcher, err := ab.Watcher()
	if err != nil || watcher.String() != "a-b-turns" {
		t.Fatalf("watcher: %v, %v", watcher, err)
	}
	for _, teams := range [][]name.TeamName{{a, ab}, {ab, a}} {
		team, short, ok := name.ResolveHerdr(h.String(), teams)
		if !ok || team != ab || short != "c-d" {
			t.Errorf("resolved: %v %q %v", team, short, ok)
		}
	}
	for _, value := range []string{"a", "other-lead", "A-lead"} {
		if _, _, ok := name.ResolveHerdr(value, []name.TeamName{a, ab}); ok {
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

func TestWorkerTabs(t *testing.T) {
	team, _ := name.ParseTeamName("a-b")
	worker, _ := name.ParseWorkerName("c-d")
	label := team.WorkerTab(worker)
	if label != "worker:a-b/c-d" {
		t.Fatal(label)
	}
	gotTeam, gotWorker, ok := name.ParseWorkerTab(label)
	if !ok || gotTeam != team || gotWorker != worker {
		t.Fatalf("%v %v %v", gotTeam, gotWorker, ok)
	}
	for _, invalid := range []string{"a-b/c-d", "worker:/a", "worker:a/", "worker:a/b/c", "worker:A/b"} {
		if _, _, ok := name.ParseWorkerTab(invalid); ok {
			t.Errorf("accepted %q", invalid)
		}
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

func TestResolveHerdrKeepsUnmatchedObservation(t *testing.T) {
	team, _ := name.ParseTeamName("demo")
	for _, raw := range []string{"other-lead", "human", ""} {
		owner, short, ok := name.ResolveHerdr(raw, []name.TeamName{team})
		if ok || owner.String() != "" || short != raw {
			t.Errorf("%q: %v %q %v", raw, owner, short, ok)
		}
	}
	// Observed labels retain the old permissive reading; validation happens when
	// assigning names, not when deciding which team's existing pane to display.
	for _, raw := range []string{"demo-", "demo-1", "demo-" + strings.Repeat("a", 32)} {
		owner, short, ok := name.ResolveHerdr(raw, []name.TeamName{team})
		if !ok || owner != team || short != strings.TrimPrefix(raw, "demo-") {
			t.Errorf("%q: %v %q %v", raw, owner, short, ok)
		}
	}
}
