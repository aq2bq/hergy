package team

import (
	"reflect"
	"testing"
)

// TestRunningTeamsReadsTheLabelAndTheRecord: a tab is one of hergy's when this
// machine's launch record holds its team, or when its label still has the older
// "team:<team>@<definition>" (or "team:<team>") shape. The definition comes from
// the newest record; the older label is the fallback (r-e494).
func TestRunningTeamsReadsTheLabelAndTheRecord(t *testing.T) {
	launches := []Launch{
		{Team: "dup", Definition: "first"},
		{Team: "dup", Definition: "second"}, // the newest launch of dup wins
		{Team: "new", Definition: "newdef"},
		{Team: "", Definition: "ignored"},
		{Team: "incomplete"},
	}
	tabs := []Tab{
		{Label: "new"},                                // the record knows it
		{Label: "team:legacy@old"},                    // the older shape, with the definition
		{Label: "team:bare"},                          // the older bare shape
		{Label: "notes"},                              // a tab hergy never started
		{Label: "stray", Agents: []string{"other-1"}}, // another tool's agent
		{Label: ""},                                   // an unlabelled tab
		{Label: "team:"},                              // the older shape with nothing after it
		{Label: "team:@def"},                          // the older shape with no team
		{Label: "dup"},                                // two launches: the newest definition
		{Label: "byagent", Agents: []string{"x-1", "byagent-m"}}, // no record; the member names it
		{Label: "byagent", Agents: []string{"byagent-m"}},        // seen twice: kept once
		{Label: "new"}, // seen twice: kept once
	}
	want := []Running{
		{Team: "new", Definition: "newdef"},
		{Team: "legacy", Definition: "old"},
		{Team: "bare", Definition: "bare"},
		{Team: "dup", Definition: "second"},
		{Team: "byagent", Definition: "byagent"},
	}
	got := RunningTeams(tabs, launches)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunningTeams = %+v, want %+v", got, want)
	}
}

// TestRunningTeamsPrefersTheRecordOverAnOlderLabel: a team booted from another
// definition under the same name is what the record says, not what an old label
// still carries.
func TestRunningTeamsPrefersTheRecordOverAnOlderLabel(t *testing.T) {
	got := RunningTeams(
		[]Tab{{Label: "team:qw2@elsewhere"}},
		[]Launch{{Team: "qw2", Definition: "qw"}},
	)
	want := []Running{{Team: "qw2", Definition: "qw"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunningTeams = %+v, want %+v", got, want)
	}
}

// TestRunningTeamsWithoutARecordReadsTheOlderLabels: a machine whose record is
// empty (or gone) still finds the teams an older hergy booted, from their
// labels; a label that is only a name cannot be told from any other tab, so it
// is not found.
func TestRunningTeamsWithoutARecordReadsTheOlderLabels(t *testing.T) {
	got := RunningTeams([]Tab{{Label: "team:demo@demo"}, {Label: "team:bare"}, {Label: "demo"}, {Label: "notes"}}, nil)
	want := []Running{
		{Team: "demo", Definition: "demo"},
		{Team: "bare", Definition: "bare"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunningTeams = %+v, want %+v", got, want)
	}
}

func TestFindRunningAndNames(t *testing.T) {
	running := []Running{{Team: "a", Definition: "x"}, {Team: "b", Definition: "y"}}
	if got, ok := FindRunning(running, "b"); !ok || got.Definition != "y" {
		t.Errorf("FindRunning(b) = %+v, %v", got, ok)
	}
	if _, ok := FindRunning(running, "c"); ok {
		t.Error("FindRunning(c) found a team")
	}
	if got := RunningNames(running); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("RunningNames = %v", got)
	}
}
