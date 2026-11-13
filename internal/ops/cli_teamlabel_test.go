package ops

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n" // TestTeamCommandsFindATeamWhoseLabelIsOnlyItsName: the tab is labelled qw2 and
	// nothing else, which is what a person reads (d-a72d). The definition qw is read
	// from this machine's launch record, so a team command still finds the definition
	// behind the running team (r-e494).
	// TestStatusListsOnlyHergeTeams: a running team is one whose members are
	// running. The launch record is history, so a tab a person labelled with the
	// name of a team that has stopped (or with any other name) is not a team, even
	// though the record holds that name (E1).
)

func TestTeamCommandsFindATeamWhoseLabelIsOnlyItsName(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	writeLaunchRecord(t, "qw2", "qw")
	tabs := `{"tab_id":"t1","label":"qw2"}`
	agents := `{"name":"qw2-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1"}`
	f.runningHerdr(t, tabs, agents)
	got := mustRun(t, "status", "qw2")
	for _, line := range []string{"team: qw2 (from qw)", "qw2-lead: working"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("status qw2 is missing %q:\n%s", line, got)
		}
	}
}

func TestStatusListsOnlyHergeTeams(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	writeLaunchRecord(t, "qw2", "qw")

	tabs := `{"tab_id":"t1","label":"qw2"},{"tab_id":"t2","label":"notes"},{"tab_id":"t3","label":"docs"}`
	agents := `{"name":"qw2-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}` + `,{"name":"claude-1","agent":"claude","agent_status":"idle","pane_id":"p3","tab_id":"t3"}`
	f.runningHerdr(t, tabs, agents)
	got := mustRun(t, "status")
	if !strings.Contains(got, "team: qw2 (from qw)\n") {
		t.Fatalf("status does not list the team whose member is running:\n%s", got)
	}
	for _, name := range []string{"notes", "docs"} {
		if strings.Contains(got, "team: "+name+"\n") {
			t.Fatalf("status lists %q, which is not a running team:\n%s", name, got)
		}
	}
}

// TestATeamWhoseBootFailedIsStillFound: a boot that failed once the tab was made
// leaves a tab and a member, and no launch record, so the definition cannot be
// read. The member names the team, so status finds the tab, and down closes the
// panes hergy started for it by name: a definition is only what a prompt is built
// from (r-e494).
func TestATeamWhoseBootFailedIsStillFound(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	mustRun(t, "new", "qw")
	log := f.halfBootedHerdr(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "qw", "team.yaml"), "version: 1\ndescription: \n")
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	out, stderr, code := run("status", "qw2")
	if code == 0 {
		t.Fatalf("status of a team whose definition cannot be read succeeded:\n%s", out)
	}
	if !strings.Contains(stderr, "qw2") || !strings.Contains(stderr, i18n.Tf("status.can_down", "qw2")) {
		t.Fatalf("status does not say what can be done about the team:\n%s", stderr)
	}
	if listed := mustRun(t, "status"); !strings.Contains(listed, "team: qw2 (from qw)\n") {
		t.Fatalf("status does not list the half-booted team's tab:\n%s", listed)
	}

	down := mustRun(t, "down", "qw2")
	for _, line := range []string{
		"team: qw2 (from qw)",
		i18n.T("down.closes_by_name"),
		"qw2-m: closing (pane p-qw2-m)",
		"closed: 1",
	} {
		if !strings.Contains(down, line+"\n") {
			t.Fatalf("down qw2 is missing %q:\n%s", line, down)
		}
	}
	closes := readFile(t, log)
	if !strings.Contains(closes, "pane close p-qw2-m") {
		t.Fatalf("down did not close the pane hergy started:\n%s", closes)
	}

	if strings.Contains(closes, "p-qw2-x-lead") || strings.Contains(down, "qw2-x-lead") {
		t.Fatalf("down qw2 closed another team's member:\n%s\n%s", down, closes)
	}
}

// TestOlderLabelsStillFindTheirTeams: a team booted before the label was cut
// down is still found, from the label alone, with the definition it carries.
func TestOlderLabelsDoNotCreateRunningTeams(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:qw2@qw"},{"tab_id":"t2","label":"qw"}`, "")
	if got := mustRun(t, "status"); !strings.Contains(got, "no teams are running") {
		t.Fatalf("unregistered/legacy labels: %s", got)
	}
}
