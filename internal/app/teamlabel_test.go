package app

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/web"
)

// TestTeamCommandsFindATeamWhoseLabelIsOnlyItsName: the tab is labelled qw2 and
// nothing else, which is what a person reads (d-a72d). The definition qw is read
// from this machine's launch record, so a team command still finds the definition
// behind the running team (r-e494).
func TestTeamCommandsFindATeamWhoseLabelIsOnlyItsName(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw", "--local")
	writeLaunchRecord(t, "qw2", "qw")
	tabs := `{"tab_id":"t1","label":"qw2"}`
	agents := `{"name":"qw2-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1"}`
	f.runningHerdr(t, tabs, agents)

	got := mustRun(t, "status", "qw2")
	for _, line := range []string{"team definition: qw", "team: qw2", "qw2-lead: working"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("status qw2 is missing %q:\n%s", line, got)
		}
	}
}

// TestStatusListsOnlyTheTeamsInTheRecord: with the label cut down to the team's
// name, a tab hergy never started looks like any other tab. The record is what
// tells them apart, so the person's own tab is not listed as a running team.
func TestStatusListsOnlyTheTeamsInTheRecord(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw", "--local")
	writeLaunchRecord(t, "qw2", "qw")
	tabs := `{"tab_id":"t1","label":"qw2"},{"tab_id":"t9","label":"notes"}`
	f.runningHerdr(t, tabs, "")

	got := mustRun(t, "status")
	if !strings.Contains(got, "team: qw2\n") {
		t.Fatalf("status does not list the team this machine launched:\n%s", got)
	}
	if strings.Contains(got, "notes") {
		t.Fatalf("status lists a tab hergy never started:\n%s", got)
	}
}

// TestATeamWhoseBootFailedIsStillFound: a boot that failed once the tab was made
// leaves a tab and a member, and no launch record. The member hergy started is
// named after the team, so the tab is still hergy's: status finds it and down can
// close it, with the definition falling back to the team's name (r-e494).
func TestATeamWhoseBootFailedIsStillFound(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw", "--local")
	tabs := `{"tab_id":"t1","label":"qw2"}`
	agents := `{"name":"qw2-m","agent":"codex","agent_status":"blocked","pane_id":"p1","tab_id":"t1"}`
	f.runningHerdr(t, tabs, agents)

	// The tab is found (the member names it), and with no record the definition
	// falls back to the team's own name, which is not a definition here: that is
	// the one thing this label change cannot tell apart, and the error names the
	// candidates. `hergy status` with no name lists the team all the same.
	out, stderr, code := run("status", "qw2")
	_ = out
	if code == 0 {
		t.Fatalf("status of a team with no record and no definition of that name succeeded:\n%s", out)
	}
	if !strings.Contains(stderr, "qw2") {
		t.Fatalf("status does not name the team it could not read:\n%s", stderr)
	}
	listed := mustRun(t, "status")
	if !strings.Contains(listed, "team: qw2\n") {
		t.Fatalf("status does not list the half-booted team's tab:\n%s", listed)
	}
}

// TestOlderLabelsStillFindTheirTeams: a team booted before the label was cut
// down is still found, from the label alone, with the definition it carries.
func TestOlderLabelsStillFindTheirTeams(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw", "--local")
	tabs := `{"tab_id":"t1","label":"team:qw2@qw"},{"tab_id":"t2","label":"team:bare"}`
	f.runningHerdr(t, tabs, "")

	got := mustRun(t, "status")
	for _, line := range []string{"team definition: qw", "team: qw2"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("status does not read the older label: missing %q:\n%s", line, got)
		}
	}
}

// writeLaunchRecord puts one launch into the record of the state directory the
// fixture set: it is how a team whose tab is labelled with its name alone is
// recognized, and where its definition is read from.
func writeLaunchRecord(t *testing.T, teamName, definition string) {
	t.Helper()
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(launchlog.Record{
		Command:    launchlog.CommandUp,
		Team:       teamName,
		Definition: definition,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestTheWebUIScreensReadANameOnlyLabel: the running screen and the team list's
// running mark go through the same reader, so they too find a team whose tab is
// labelled with its name alone, and know the definition from the record.
func TestTheWebUIScreensReadANameOnlyLabel(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	mustRun(t, "new", "qw", "--local")
	writeLaunchRecord(t, "qw2", "qw")
	tabs := `{"tab_id":"t1","label":"qw2"}`
	agents := `{"name":"qw2-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}`
	f.runningHerdr(t, tabs, agents)

	p := servePort{env: serveEnv(t)}
	if defs := p.runningDefinitions(); !defs["qw"] {
		t.Errorf("the team list does not mark the definition as running: %v", defs)
	}
	running, err := p.Running(web.LayerLocal)
	if err != nil {
		t.Fatalf("Running: %v", err)
	}
	if len(running.Teams) != 1 {
		t.Fatalf("the running screen shows %d teams, want the one: %+v", len(running.Teams), running.Teams)
	}
	live := running.Teams[0]
	if live.Team != "qw2" || live.Definition != "qw" || live.TabID != "t1" {
		t.Errorf("the running team = %+v", live)
	}
	if !live.Known {
		t.Errorf("the team's definition qw is not known: %+v", live)
	}
}
