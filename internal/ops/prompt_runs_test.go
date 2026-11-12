package ops

import (
	"testing"
	"time" // TestRunningHoldKeepsOneReadingForTwoSeconds: the prompt owner and the check
	// share one herdr reading for two seconds (⑤ r-ba9b, d-002b).
	// TestPromptRunsOfFiltersByDefinitionPlaceAndProject: the runs the prompt owner
	// names are the same teams PromptHistory used to choose from.
)

func TestRunningHoldKeepsOneReadingForTwoSeconds(t *testing.T) {
	at := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	hold := &runningHold{}
	calls := 0
	read := func() HQRunning {
		calls++
		return HQRunning{Herdr: ActivityNote{OK: true}, Teams: []HQRunningTeam{{Team: fixtureRunningTeam("core")}}}
	}
	if got := hold.within(at, read); len(got.Teams) != 1 || calls != 1 {
		t.Fatalf("first reading = %+v, calls %d", got, calls)
	}
	hold.within(at.Add(time.Second), read)
	if calls != 1 {
		t.Errorf("a second question within two seconds read herdr again (calls %d)", calls)
	}
	hold.within(at.Add(2*time.Second), read)
	if calls != 2 {
		t.Errorf("two seconds later kept the old reading (calls %d)", calls)
	}
	hold.within(at.Add(3*time.Second), read)
	if calls != 2 {
		t.Errorf("one second after a fresh reading read herdr again (calls %d)", calls)
	}
	hold.within(at.Add(4*time.Second), read)
	if calls != 3 {
		t.Errorf("two seconds after the fresh reading kept it (calls %d)", calls)
	}
}

func TestPromptRunsOfFiltersByDefinitionPlaceAndProject(t *testing.T) {
	live := HQRunning{Teams: []HQRunningTeam{{Team: fixtureRunningTeam("core"), Definition: "core", Place: "project", Project: projectPath("/p/one")},
		{Team: fixtureRunningTeam("core-hotfix"), Definition: "core", Place: "project", Project: projectPath("/p/one")},
		{Team: fixtureRunningTeam("other"), Definition: "other", Place: "project", Project: projectPath("/p/one")},
		{Team: fixtureRunningTeam("docs"), Definition: "core", Place: "global", Project: projectPath("/p/two")},
		{Team: fixtureRunningTeam("core-elsewhere"), Definition: "core", Place: "project", Project: projectPath("/p/two")},
	}}
	if got := promptRunsOf(live, projectPath("/p/one"), "core", "project"); len(got) != 2 || got[0].Team.String() != "core" || got[1].Team.String() != "core-hotfix" {
		t.Errorf("project runs = %+v", got)
	}
	if got := promptRunsOf(live, projectPath("/p/one"), "core", "global"); len(got) != 1 || got[0].Team.String() != "docs" {
		t.Errorf("global runs = %+v", got)
	}
	if got := promptRunsOf(live, projectPath("/p/three"), "core", "project"); len(got) != 0 {
		t.Errorf("another project's runs = %+v", got)
	}
}
