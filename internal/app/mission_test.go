package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// missionTeam writes one local team definition whose team.yaml writes the given
// mission line (empty for a definition that writes none), over the starter
// team's own files, so show, up --dry and validate read a definition. The
// starter files are written the first time only; every later call rewrites
// team.yaml alone.
func missionTeam(t *testing.T, f *fixture, mission string) {
	t.Helper()
	starter := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	if _, err := os.Stat(starter); err != nil {
		mustRun(t, "new", "demo")
	}
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	body := "version: 1\n" + mission +
		"members:\n  - name: lead\n    role: lead\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n"
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
		header := "team definition: demo\n"
		if argv[0] == "up" {
			// A team command names the team; the definition is named only
			// when the two differ.
			header = "team: demo\n"
		}
		if !strings.Contains(got, header) {
			t.Errorf("%v does not carry the header after the mission:\n%s", argv, got)
		}
	}
}

// TestValidateStopsOnABadMission: the mission is required (I15), so a
// definition that writes none, writes an empty one, or writes one over more
// than one line does not read, and hergy validate reports the file broken. One
// line reads, and validate says nothing about the mission.
func TestValidateStopsOnABadMission(t *testing.T) {
	f := newFixture(t)
	initLocales(t)

	cases := []struct{ name, mission, key string }{
		{"no mission", "", "error.mission_missing"},
		{"written empty", "mission: \"\"\n", "error.mission_missing"},
		{"over two lines", "mission: |-\n  first\n  second\n", "error.mission_multiline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			missionTeam(t, f, tc.mission)
			stdout, _, code := run("validate")
			if code == 0 {
				t.Fatalf("validate accepted a definition with %s:\n%s", tc.name, stdout)
			}
			// The detail carries the file, the way hergy validate names a
			// broken file (D2).
			want := i18n.Tf(tc.key, ".hergy/teams/demo/team.yaml")
			if !strings.Contains(stdout, want) {
				t.Errorf("validate does not say %q:\n%s", want, stdout)
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
		i18n.Tf("error.mission_missing", "team.yaml"),
		i18n.Tf("error.mission_multiline", "team.yaml"),
		i18n.T("validate.mission_placeholder"),
	} {
		if strings.Contains(stdout, unwanted) {
			t.Errorf("validate says %q about a mission written in one line:\n%s", unwanted, stdout)
		}
	}
	if show := mustRun(t, "show", "demo"); !strings.HasPrefix(show, "mission: one line is enough\n") {
		t.Errorf("show does not print the mission:\n%s", show)
	}
}

// TestStatusNamesWhyARunningTeamsDefinitionDoesNotRead: a running team whose
// definition is here but does not read is not "in another project": status says
// why the file does not read, with its path (D1). A definition that is not here
// at all keeps the sentence that says so.
func TestStatusNamesWhyARunningTeamsDefinitionDoesNotRead(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	// The definition stops reading: the mission line is gone.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\ndocs: [protocol.yaml]\n"+
			"members:\n  - name: lead\n    role: lead\n    profile: claude-default\n"+
			"  - name: builder\n    role: builder\n    profile: claude-default\n")

	got := mustRun(t, "status")
	if strings.Contains(got, i18n.T("status.definition_elsewhere")) {
		t.Errorf("status says a definition that is here is in another project:\n%s", got)
	}
	want := i18n.Tf("error.mission_missing", ".hergy/teams/demo/team.yaml")
	if !strings.Contains(got, want) {
		t.Errorf("status does not say why the file does not read (%q):\n%s", want, got)
	}
}

// TestStatusSaysWhenTheDefinitionIsNotHere: a running team whose definition is
// gone from this project keeps the sentence that says so (D1's other half).
func TestStatusSaysWhenTheDefinitionIsNotHere(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	if err := os.RemoveAll(filepath.Join(f.proj, ".hergy", "teams", "demo")); err != nil {
		t.Fatal(err)
	}
	got := mustRun(t, "status")
	if !strings.Contains(got, i18n.T("status.definition_elsewhere")) {
		t.Errorf("status does not say the definition is not in this project:\n%s", got)
	}
}

// TestValidateWarnsAboutTheStarterMission: the starter team's mission is an
// example, so a definition that still writes it reads and boots, and hergy
// validate warns that its team has not said what it is for (d-3cae). The
// warning moves no exit code, and rewriting the line clears it.
func TestValidateWarnsAboutTheStarterMission(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	mustRun(t, "new", "demo")

	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("the starter team is a break?\n%s", stdout)
	}
	if want := "  warning: " + i18n.T("validate.mission_placeholder") + "\n"; !strings.Contains(stdout, want) {
		t.Errorf("validate does not warn about the starter mission:\n%s", stdout)
	}

	// A mission of the definition's own: the warning is gone.
	missionTeam(t, f, "mission: hergy をよくする\n")
	stdout, _, code = run("validate")
	if code != 0 {
		t.Fatalf("a written mission is a break?\n%s", stdout)
	}
	if strings.Contains(stdout, i18n.T("validate.mission_placeholder")) {
		t.Errorf("validate warns about the starter mission after it was rewritten:\n%s", stdout)
	}
}
