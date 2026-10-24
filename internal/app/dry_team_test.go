package app

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
)

// recordTeamLaunch writes one launch record for a team, so a --dry command that
// no longer reads herdr can find the definition from this project's record.
func TestDryCommandsPlanWithoutHerdr(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	for _, argv := range [][]string{
		{"reprompt", "lead", "--team", "demo", "--dry"},
		{"nudge", "lead", "--team", "demo", "--dry"},
		{"stop", "lead", "--team", "demo", "--dry"},
		{"tell", "lead", "hello", "--team", "demo", "--dry"},
	} {
		got := mustRun(t, argv...)
		if !strings.Contains(got, "demo-lead") {
			t.Errorf("%v does not name demo-lead without herdr:\n%s", argv, got)
		}
	}
}

func recordTeamLaunch(t *testing.T, team string) {
	t.Helper()
	env := serveEnv(t)
	r, err := readDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "qw", team)
	if err != nil {
		t.Fatalf("read qw as %s: %v", team, err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.starts, launchlog.Workspace{}); err != nil {
		t.Fatalf("record %s: %v", team, err)
	}
}
