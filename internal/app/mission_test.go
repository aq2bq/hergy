package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// missionTeam writes one local team definition whose team.yaml writes the given
// mission line (empty for none), over the starter team's own files, so show,
// up --dry and validate read a definition that reads. The starter files are
// written the first time only; every later call rewrites team.yaml alone.
func missionTeam(t *testing.T, f *fixture, mission string) {
	t.Helper()
	starter := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	if _, err := os.Stat(starter); err != nil {
		mustRun(t, "new", "demo", "--local")
	}
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	body := "version: 1\n" + mission +
		"docs: [protocol.yaml]\nmembers:\n  - name: lead\n    role: lead\n    profile: claude-default\n"
	writeFile(t, path, body)
}

// TestShowAndUpDryPrintTheMissionFirst: team.yaml's mission: is the one line
// that says what the team is for, and show and up --dry print it ahead of the
// rest of the header (r-341e). A definition without a mission prints no line
// for it, and the header is otherwise the header it was.
func TestShowAndUpDryPrintTheMissionFirst(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	missionTeam(t, f, "mission: hergy をよくする\n")

	for _, argv := range [][]string{{"show", "demo"}, {"up", "demo", "--dry"}} {
		got := mustRun(t, argv...)
		want := "mission: hergy をよくする\n"
		if argv[0] == "up" {
			// The plan names what it is before the team's own lines.
			want = "mode: boot\n" + want
		}
		if !strings.HasPrefix(got, want) {
			t.Errorf("%v does not open with the mission:\n%s", argv, got)
		}
		if !strings.Contains(got, "team definition: demo\n") {
			t.Errorf("%v does not carry the header after the mission:\n%s", argv, got)
		}
	}

	// No mission, no line: the header opens with the definition again.
	missionTeam(t, f, "")
	if got := mustRun(t, "show", "demo"); !strings.HasPrefix(got, "team definition: demo\n") {
		t.Errorf("a definition without a mission opens with something else:\n%s", got)
	}
}

// TestServePortHandsTheMissionToTheScreens: the two screens that show what the
// team is for read it from the same definition the commands read (r-341e): the
// team list's row and the definition screen carry the line team.yaml writes, and
// a definition without one carries none rather than a sentence of its own — the
// screens say "no mission is written" themselves.
func TestServePortHandsTheMissionToTheScreens(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	missionTeam(t, f, "mission: hergy をよくする\n")
	port := servePort{env: serveEnv(t)}

	rows := port.Teams(web.LayerLocal)
	if len(rows) != 1 {
		t.Fatalf("the team list has %d rows, want the one definition", len(rows))
	}
	if rows[0].Mission != "hergy をよくする" {
		t.Errorf("the team list row carries the mission %q, want the one written", rows[0].Mission)
	}
	def, err := port.Definition(web.LayerLocal, "demo")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if def.Mission != "hergy をよくする" {
		t.Errorf("the definition screen carries the mission %q, want the one written", def.Mission)
	}

	missionTeam(t, f, "")
	rows = port.Teams(web.LayerLocal)
	if rows[0].Mission != "" {
		t.Errorf("a definition without a mission carries %q, want none", rows[0].Mission)
	}
	def, err = port.Definition(web.LayerLocal, "demo")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if def.Mission != "" {
		t.Errorf("a definition without a mission carries %q, want none", def.Mission)
	}
}

// TestValidateWarnsAboutTheMission: validate says when a team definition writes
// no mission, and when it writes one over more than one line, and neither moves
// the exit code: a team whose mission is missing or long boots the same way a
// team over the provisional size does (r-341e, n-9eea).
func TestValidateWarnsAboutTheMission(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	missionTeam(t, f, "")

	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a missing mission is a break?\n%s", got)
	}
	if want := "  warning: " + i18n.T("validate.mission_missing") + "\n"; !strings.Contains(got, want) {
		t.Errorf("validate does not warn about the missing mission:\n%s", got)
	}

	// Over more than one line: validate says so, and keeps the lines as written
	// rather than joining them.
	missionTeam(t, f, "mission: |-\n  first\n  second\n")
	got, _, code = run("validate")
	if code != 0 {
		t.Fatalf("a mission over two lines is a break?\n%s", got)
	}
	if want := "  warning: " + i18n.T("validate.mission_multiline") + "\n"; !strings.Contains(got, want) {
		t.Errorf("validate does not say the mission is over one line:\n%s", got)
	}
	if strings.Contains(got, i18n.T("validate.mission_missing")) {
		t.Errorf("validate warns about a missing mission when one is written:\n%s", got)
	}
	// What the commands show is the first line, not the paragraph.
	if show := mustRun(t, "show", "demo"); !strings.HasPrefix(show, "mission: first\n") {
		t.Errorf("show does not print the first line of the mission:\n%s", show)
	}

	// One line: no warning at all.
	missionTeam(t, f, "mission: one line is enough\n")
	got, _, code = run("validate")
	if code != 0 {
		t.Fatalf("a written mission is a break?\n%s", got)
	}
	for _, unwanted := range []string{
		i18n.T("validate.mission_missing"), i18n.T("validate.mission_multiline"),
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("validate warns about a mission that is written in one line:\n%s", got)
		}
	}
}
