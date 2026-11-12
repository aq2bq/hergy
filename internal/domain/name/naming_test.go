package name_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/domain/name"
)

func TestNamingKeepsTheRunningAndProspectiveBranches(t *testing.T) {
	member, _ := name.ParseMemberName("lead")
	worker, _ := name.ParseWorkerName("build")
	team, _ := name.ParseTeamName("hotfix")
	definition, _ := name.ParseDefinitionName("Upper")
	running := name.RunningNames(team.Ref())
	prospective := name.DefinitionNames(definition)
	legacy := name.RunningNames(name.TeamFromObservation(fixtureObservation("Upper")))
	for _, tc := range []struct {
		names          name.Naming
		member, worker string
		target         bool
	}{
		{running, "hotfix-lead", "hotfix-build", true},
		{prospective, "Upper-lead", "Upper-build", false},
		{legacy, "Upper-lead", "Upper-build", false},
	} {
		if got := tc.names.Member(member); got != tc.member {
			t.Fatalf("Member = %q, want %q", got, tc.member)
		}
		if got := tc.names.Worker(worker.String()); got != tc.worker {
			t.Fatalf("Worker = %q, want %q", got, tc.worker)
		}
		target, err := tc.names.MemberTarget(member)
		if (err == nil) != tc.target || err == nil && target.String() != tc.member {
			t.Fatalf("MemberTarget = %s, %v", target, err)
		}
		hired, err := tc.names.WorkerTarget(worker)
		if (err == nil) != tc.target || err == nil && hired.String() != tc.worker {
			t.Fatalf("WorkerTarget = %s, %v", hired, err)
		}
	}
	if got := prospective.Worker("<worker>"); got != "Upper-<worker>" {
		t.Fatal(got)
	}
	long, _ := name.ParseDefinitionName(strings.Repeat("x", 33))
	if got := name.DefinitionNames(long).Member(member); got != strings.Repeat("x", 33)+"-lead" {
		t.Fatal(got)
	}
}

func TestObservedWorkerDerivesItsAgentFromOneLabel(t *testing.T) {
	for _, tc := range []struct{ label, team, short, agent string }{
		{"worker:Upper/1", "Upper", "1", "Upper-1"},
		{"worker:team/a/b", "team", "a/b", "team-a/b"},
	} {
		worker, ok := fixtureObservation(tc.label).WorkerTab()
		if !ok || worker.Team().String() != tc.team || worker.Short().String() != tc.short || worker.Agent().String() != tc.agent || worker.Label().String() != tc.label {
			t.Fatalf("WorkerTab(%q) = %+v, %v", tc.label, worker, ok)
		}
	}
	scoped, ok := fixtureObservation("worker:Upper/a/1").WorkerIn(fixtureObservation("Upper/a"))
	if !ok || scoped.Agent().String() != "Upper/a-1" {
		t.Fatalf("scoped worker = %+v, %v", scoped, ok)
	}
}

func TestObservationRecordsPreserveForeignSpellings(t *testing.T) {
	for _, raw := range []string{"", "Upper-1", "worker:Upper/a/b", strings.Repeat("x", 40)} {
		encoded, _ := json.Marshal(raw)
		var seen name.Seen
		if err := json.Unmarshal(encoded, &seen); err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(seen)
		if err != nil || string(got) != string(encoded) {
			t.Fatalf("round trip %q = %s, %v", raw, got, err)
		}
	}
}

func fixtureObservation(raw string) name.Seen {
	var seen name.Seen
	if err := seen.UnmarshalText([]byte(raw)); err != nil {
		panic(err)
	}
	return seen
}
