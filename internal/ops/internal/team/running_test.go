package team

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/teststate"
)

func TestRunningTeamsRequiresStateAndExactTab(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	project := t.TempDir()
	for _, name := range []string{"fresh", "stopped"} {
		if err := teststate.Write(teststate.Team{Name: name, Definition: "definition", Project: project, WorkspaceID: "recorded-space", WorkspaceLabel: "recorded label"}); err != nil {
			t.Fatal(err)
		}
	}
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	states, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	tabs := []Tab{
		{ID: "new", Label: fixtureSeen("fresh"), PaneDirs: seenPaths([]string{filepath.Join(project, "elsewhere")})}, // no pane names or agents
		{ID: "legacy", Label: fixtureSeen("team:fresh@wrong")},
		{ID: "unregistered", Label: fixtureSeen("old"), Agents: fixtureSeenList([]string{"old-lead"}), Panes: fixtureSeenList([]string{"old-lead"})},
		{ID: "duplicate", Label: fixtureSeen("fresh")},
	}
	got := RunningTeams(tabs, states)
	if len(got) != 1 || got[0].Team.String() != "fresh" || got[0].Definition != "definition" || got[0].Project.String() != projectPath(project).String() || got[0].State.Launch().Workspace.ID != "recorded-space" {
		t.Fatalf("running=%+v", got)
	}
	if got := RunningTeams(tabs, nil); len(got) != 0 {
		t.Fatalf("inferred teams without state: %+v", got)
	}
}

func TestFindRunningAndNames(t *testing.T) {
	running := []Running{{Team: fixtureRunningTeam("a"), Definition: "x"}, {Team: fixtureRunningTeam("b"), Definition: "y"}}
	if got, ok := FindRunning(running, "b"); !ok || got.Definition != "y" {
		t.Fatal(got, ok)
	}
	if _, ok := FindRunning(running, "missing"); ok {
		t.Fatal("found missing team")
	}
	if !reflect.DeepEqual(RunningNames(running), []string{"a", "b"}) {
		t.Fatal(RunningNames(running))
	}
}

func TestMemberAgentsAndPanesRemainAvailableForCleanup(t *testing.T) {
	tabs := []Tab{{ID: "team", Label: fixtureSeen("demo"), Agents: fixtureSeenList([]string{"demo-lead"}), Panes: fixtureSeenList([]string{"demo-lead", "mine"})},
		{ID: "other", Label: fixtureSeen("demo-x"), Agents: fixtureSeenList([]string{"demo-x-lead"})}}
	if got := MemberAgents(tabs, fixtureSeen("demo")); !reflect.DeepEqual(fixtureSpellings(got), []string{"demo-lead"}) {
		t.Fatal(got)
	}
	if got := MemberPanes(tabs, fixtureSeen("demo")); !reflect.DeepEqual(fixtureSpellings(got), []string{"demo-lead"}) {
		t.Fatal(got)
	}
}
