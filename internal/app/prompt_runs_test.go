package app

import (
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/web"
)

// TestRunningHoldKeepsOneReadingForTwoSeconds: the prompt owner and the check
// share one herdr reading for two seconds (⑤ r-ba9b, d-002b).
func TestRunningHoldKeepsOneReadingForTwoSeconds(t *testing.T) {
	at := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	hold := &runningHold{}
	calls := 0
	read := func() web.HQRunning {
		calls++
		return web.HQRunning{Herdr: web.ActivityNote{OK: true}, Teams: []web.HQRunningTeam{{Team: "core"}}}
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

// TestPromptRunsOfFiltersByDefinitionPlaceAndProject: the runs the prompt owner
// names are the same teams PromptHistory used to choose from.
func TestPromptRunsOfFiltersByDefinitionPlaceAndProject(t *testing.T) {
	live := web.HQRunning{Teams: []web.HQRunningTeam{
		{Team: "core", Definition: "core", Place: "project", Project: "/p/one"},
		{Team: "core-hotfix", Definition: "core", Place: "project", Project: "/p/one"},
		{Team: "other", Definition: "other", Place: "project", Project: "/p/one"},
		{Team: "docs", Definition: "core", Place: "global", Project: "/p/two"},
		{Team: "core-elsewhere", Definition: "core", Place: "project", Project: "/p/two"},
	}}
	if got := promptRunsOf(live, "/p/one", "core", "project"); len(got) != 2 || got[0].Team != "core" || got[1].Team != "core-hotfix" {
		t.Errorf("project runs = %+v", got)
	}
	if got := promptRunsOf(live, "/p/one", "core", "global"); len(got) != 1 || got[0].Team != "docs" {
		t.Errorf("global runs = %+v", got)
	}
	if got := promptRunsOf(live, "/p/three", "core", "project"); len(got) != 0 {
		t.Errorf("another project's runs = %+v", got)
	}
}
