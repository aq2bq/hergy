package team

import (
	"reflect"
	"testing"
)

// TestRunningTeamsReadsTheLabelTheMembersAndTheRecord: a tab is one of hergy's
// when a live agent in it is named after the tab's label, or when its label still
// has the older "team:<team>@<definition>" (or "team:<team>") shape. A tab a
// person labelled with the name of a team that has stopped is not a team, even
// though the record holds that name; the record is where the definition is read
// from (r-e494, E1).
func TestRunningTeamsReadsTheLabelTheMembersAndTheRecord(t *testing.T) {
	launches := []Launch{
		{Team: "dup", Definition: "first"},
		{Team: "dup", Definition: "second"}, // the newest launch of dup wins
		{Team: "new", Definition: "newdef"},
		{Team: "stopped", Definition: "stopped-def"},
		{Team: "", Definition: "ignored"},
		{Team: "incomplete"},
	}
	tabs := []Tab{
		{ID: "t1", Label: "new", Agents: []string{"new-lead"}},              // the record knows the definition
		{ID: "t2", Label: "team:legacy@old"},                                // the older shape, with the definition
		{ID: "t3", Label: "team:bare"},                                      // the older bare shape
		{ID: "t4", Label: "notes"},                                          // a tab hergy never started
		{ID: "t5", Label: "stopped", Agents: []string{"claude-1"}},          // a person's tab, named like a stopped team
		{ID: "t6", Label: "stray", Agents: []string{"other-1"}},             // another tool's agent
		{ID: "t7", Label: ""},                                               // an unlabelled tab
		{ID: "t8", Label: "team:"},                                          // the older shape with nothing after it
		{ID: "t9", Label: "team:@def"},                                      // the older shape with no team
		{ID: "t10", Label: "dup", Agents: []string{"dup-m"}},                // two launches: the newest definition
		{ID: "t11", Label: "byagent", Agents: []string{"x-1", "byagent-m"}}, // no record; the member names it
		{ID: "t12", Label: "byagent", Agents: []string{"byagent-m"}},        // seen twice: kept once
		{ID: "t13", Label: "new", Agents: []string{"new-lead"}},             // seen twice: kept once
	}
	want := []Running{
		{Team: "new", Definition: "newdef"},
		{Team: "legacy", Definition: "old"},
		{Team: "bare", Definition: "bare"},
		{Team: "dup", Definition: "second"},
		{Team: "byagent", Definition: "byagent"},
	}
	got := RunningTeams(tabs, launches, true)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunningTeams = %+v, want %+v", got, want)
	}
}

// TestRunningTeamsFallsBackToTheRecordWithoutAgents: when herdr's agent list
// cannot be read, there is nothing to tell a team's tab from a person's tab with
// the same name, so a label this machine recorded a launch of is taken as a
// team's rather than losing the team (E1).
func TestRunningTeamsFallsBackToTheRecordWithoutAgents(t *testing.T) {
	launches := []Launch{{Team: "stopped", Definition: "stopped-def"}}
	tabs := []Tab{{ID: "t1", Label: "stopped"}, {ID: "t2", Label: "notes"}}
	want := []Running{{Team: "stopped", Definition: "stopped-def"}}
	if got := RunningTeams(tabs, launches, false); !reflect.DeepEqual(got, want) {
		t.Errorf("RunningTeams without agents = %+v, want %+v", got, want)
	}
	if got := RunningTeams(tabs, launches, true); len(got) != 0 {
		t.Errorf("RunningTeams with agents = %+v, want nothing", got)
	}
}

// TestRunningTeamsWithoutARecordReadsTheOlderLabels: a machine whose record is
// empty (or gone) still finds the teams an older hergy booted, from their
// labels; a label that is only a name cannot be told from any other tab, so it
// is not found.
func TestRunningTeamsWithoutARecordReadsTheOlderLabels(t *testing.T) {
	got := RunningTeams([]Tab{{Label: "team:demo@demo"}, {Label: "team:bare"}, {Label: "demo"}, {Label: "notes"}}, nil, true)
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

// TestMemberAgentsKeepsAnotherTeamOut: the members of one team are the agents in
// that team's own tab. Another team whose name starts the same way ("qw2-x"
// beside "qw2"), and a person's tab carrying the name, keep their agents out of
// it (the audit's D1).
func TestMemberAgentsKeepsAnotherTeamOut(t *testing.T) {
	tabs := []Tab{
		{ID: "t1", Label: "qw2", Agents: []string{"qw2-m"}},
		{ID: "t2", Label: "qw2-x", Agents: []string{"qw2-x-lead"}},
		{ID: "t3", Label: "team:qw2@qw", Agents: []string{"qw2-old"}},
		{ID: "t4", Label: "notes", Agents: []string{"claude-1"}},
	}
	got := MemberAgents(tabs, "qw2")
	want := []string{"qw2-m", "qw2-old"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MemberAgents = %v, want %v", got, want)
	}
	if got := MemberAgents(tabs, "qw2-x"); !reflect.DeepEqual(got, []string{"qw2-x-lead"}) {
		t.Errorf("MemberAgents(qw2-x) = %v", got)
	}
}
