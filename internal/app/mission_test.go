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
// mission line (empty for a definition that writes none), over the starter
// team's own files, so show, up --dry and validate read a definition. The
// starter files are written the first time only; every later call rewrites
// team.yaml alone.
func missionTeam(t *testing.T, f *fixture, mission string) {
	t.Helper()
	starter := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	if _, err := os.Stat(starter); err != nil {
		mustRun(t, "new", "demo")
	}
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	body := "version: 1\n" + mission +
		"docs: [protocol.yaml]\nmembers:\n  - name: lead\n    role: lead\n    profile: claude-default\n"
	writeFile(t, path, body)
}

// TestShowAndUpDryPrintTheMissionFirst: team.yaml's mission: is required (I15),
// and it is the one line that says what the team is for, so show and up --dry
// print it ahead of the rest of the header (r-341e).
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
}

// TestServePortHandsTheMissionToTheScreens: the two screens that show what the
// team is for read it from the same definition the commands read (r-341e): the
// team list's row and the definition screen carry the line team.yaml writes. A
// definition that writes none does not read, so its row carries why instead.
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

	// No mission: the definition does not read, and the row says why.
	missionTeam(t, f, "")
	rows = port.Teams(web.LayerLocal)
	if rows[0].Mission != "" {
		t.Errorf("a definition without a mission carries %q, want none", rows[0].Mission)
	}
	if rows[0].Broken == "" {
		t.Errorf("a definition without a mission carries no reason, want one")
	}
	def, err = port.Definition(web.LayerLocal, "demo")
	if err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if def.Broken == "" {
		t.Errorf("the definition screen does not say why the file does not read")
	}
}

// TestValidateStopsOnABadMission: the mission is required (I15), so a
// definition that writes none, writes an empty one, or writes one over more
// than one line does not read, and hergy validate reports the file broken. One
// line reads, and validate says nothing about the mission.
func TestValidateStopsOnABadMission(t *testing.T) {
	f := newFixture(t)
	initLocales(t)

	cases := []struct{ name, mission, want string }{
		{"no mission", "", i18n.T("error.mission_missing")},
		{"written empty", "mission: \"\"\n", i18n.T("error.mission_missing")},
		{"over two lines", "mission: |-\n  first\n  second\n", i18n.T("error.mission_multiline")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			missionTeam(t, f, tc.mission)
			stdout, _, code := run("validate")
			if code == 0 {
				t.Fatalf("validate accepted a definition with %s:\n%s", tc.name, stdout)
			}
			if !strings.Contains(stdout, tc.want) {
				t.Errorf("validate does not say %q:\n%s", tc.want, stdout)
			}
			if _, _, code := run("show", "demo"); code == 0 {
				t.Errorf("show read a definition with %s", tc.name)
			}
		})
	}

	// One line: the definition reads, validate is quiet, and show prints it.
	missionTeam(t, f, "mission: one line is enough\n")
	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a written mission is a break?\n%s", stdout)
	}
	for _, unwanted := range []string{
		i18n.T("error.mission_missing"), i18n.T("error.mission_multiline"),
	} {
		if strings.Contains(stdout, unwanted) {
			t.Errorf("validate says %q about a mission written in one line:\n%s", unwanted, stdout)
		}
	}
	if show := mustRun(t, "show", "demo"); !strings.HasPrefix(show, "mission: one line is enough\n") {
		t.Errorf("show does not print the mission:\n%s", show)
	}
}
