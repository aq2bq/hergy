package ops

import (
	"strings"
	"testing" // recordTeamLaunch writes one launch record for a team, so a --dry command that
	// no longer reads herdr can find the definition from this project's record.
)

func TestDryCommandsPlanWithoutHerdr(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	for _, argv := range [][]string{{"reprompt", "lead", "--team", "demo", "--dry"}, {"nudge", "lead", "--team", "demo", "--dry"}, {"stop", "lead", "--team", "demo", "--dry"}, {"tell", "lead", "hello", "--team", "demo", "--dry"}} {
		got := mustRun(t, argv...)
		if !strings.Contains(got, "demo-lead") {
			t.Errorf("%v does not name demo-lead without herdr:\n%s", argv, got)
		}
	}
}
