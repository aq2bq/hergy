package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
)

// TestTeamCommandsFindATeamWhoseLabelIsOnlyItsName: the tab is labelled qw2 and
// nothing else, which is what a person reads (d-a72d). The definition qw is read
// from this machine's launch record, so a team command still finds the definition
// behind the running team (r-e494).
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

// TestStatusListsOnlyHergeTeams: a running team is one whose members are
// running. The launch record is history, so a tab a person labelled with the
// name of a team that has stopped (or with any other name) is not a team, even
// though the record holds that name (E1).
func TestStatusListsOnlyHergeTeams(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	writeLaunchRecord(t, "qw2", "qw")
	writeLaunchRecord(t, "docs", "docs-team") // a team that ran once and has stopped
	tabs := `{"tab_id":"t1","label":"qw2"},{"tab_id":"t2","label":"notes"},{"tab_id":"t3","label":"docs"}`
	agents := `{"name":"qw2-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}` +
		`,{"name":"claude-1","agent":"claude","agent_status":"idle","pane_id":"p3","tab_id":"t3"}`
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
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	// status names the team, keeps the reason, and says what can still be done.
	out, stderr, code := run("status", "qw2")
	if code == 0 {
		t.Fatalf("status of a team whose definition cannot be read succeeded:\n%s", out)
	}
	if !strings.Contains(stderr, "qw2") || !strings.Contains(stderr, i18n.Tf("status.can_down", "qw2")) {
		t.Fatalf("status does not say what can be done about the team:\n%s", stderr)
	}
	if listed := mustRun(t, "status"); !strings.Contains(listed, "team: qw2\n") {
		t.Fatalf("status does not list the half-booted team's tab:\n%s", listed)
	}

	// down closes the member's pane, by the name hergy gave it.
	down := mustRun(t, "down", "qw2")
	for _, line := range []string{
		"team: qw2",
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
	// Another team whose name starts the same way has its own tab, and keeps its
	// member: a name alone would have closed it (the audit's D1).
	if strings.Contains(closes, "p-qw2-x-lead") || strings.Contains(down, "qw2-x-lead") {
		t.Fatalf("down qw2 closed another team's member:\n%s\n%s", down, closes)
	}
}

// halfBootedHerdr writes a fake herdr with a tab labelled by the team's name
// alone, one live member in it, and no launch record: what a boot that failed
// once the tab was made leaves behind. It records the calls in <cfg>/herdr.log
// and returns that path.
func (f *fixture) halfBootedHerdr(t *testing.T) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f.herdr(t, `#!/bin/sh
log='`+log+`'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"qw2-m","agent":"codex","agent_status":"blocked","pane_id":"p-qw2-m","tab_id":"t1"},{"name":"qw2-x-lead","agent":"claude","agent_status":"idle","pane_id":"p-qw2-x-lead","tab_id":"t2"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"qw2"},{"tab_id":"t2","label":"qw2-x"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[{"pane_id":"p-qw2-m","label":"qw2-m","tab_id":"t1"},{"pane_id":"p-qw2-x-lead","label":"qw2-x-lead","tab_id":"t2"}]}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	return log
}

// TestOlderLabelsStillFindTheirTeams: a team booted before the label was cut
// down is still found, from the label alone, with the definition it carries.
func TestOlderLabelsStillFindTheirTeams(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	tabs := `{"tab_id":"t1","label":"team:qw2@qw"},{"tab_id":"t2","label":"team:bare"}`
	f.runningHerdr(t, tabs, "")

	got := mustRun(t, "status")
	for _, line := range []string{"team: qw2 (from qw)"} {
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
