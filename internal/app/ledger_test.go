package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
)

// ledgerProject is a project with one team definition that writes ledger: rules,
// a gy.toml, and the profiles and role documents the definition reads. script,
// when it is not empty, is written as the fake gy HERGY_GY points at; empty
// leaves the fixture's no-gy path, which is a gy that cannot be read.
func ledgerProject(t *testing.T, script string) *fixture {
	t.Helper()
	f := newFixture(t)
	writeFile(t, filepath.Join(f.proj, "gy.toml"), "[scopes.common]\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	for _, role := range []string{"lead", "designer"} {
		writeFile(t, filepath.Join(f.proj, ".hergy", "roles", role+".yaml"),
			"version: 1\nsections:\n  - rules:\n      - id: R1\n        text: one\n")
	}
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: claude-default\n"+
			"  - name: designer\n    role: designer\n    profile: claude-default\n"+
			"turns:\n  - human -> lead\n  - lead -> designer\n  - designer -> lead\n  - lead -> human\n"+
			"ledger:\n  - req approve by lead\n  - question to human by lead\n")
	if script != "" {
		path := filepath.Join(f.cfg, "gy")
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HERGY_GY", path)
	}
	return f
}

// TestValidateWarnsWhenALedgerRuleIsBroken: a write a member of the team made
// that the rule gives to another member is one warning under the definition's
// file line, and it does not make a problem (spec-ledger, d-db1b).
func TestValidateWarnsWhenALedgerRuleIsBroken(t *testing.T) {
	ledgerProject(t, `if [ "$1" = "list" ]; then
  echo '[{"seq":1,"actor":"demo-designer","node":"r-1c42","what":"changed","why":"req approve r-1c42"}]'
  exit 0
fi
`)
	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("validate stopped on a broken ledger rule:\n%s", stdout)
	}
	want := "  " + label("warning") + ": " +
		i18n.Tf("validate.ledger_breach", "r-1c42", "req approve", "demo-designer", "demo", "lead") + "\n"
	if !strings.Contains(stdout, want) {
		t.Errorf("validate does not warn about the write:\nwant %q\ngot:\n%s", want, stdout)
	}
	if !strings.Contains(stdout, "problems: 0\n") || strings.Contains(stdout, "result: broken") {
		t.Errorf("a broken ledger rule changed the problems:\n%s", stdout)
	}
}

// TestValidateIgnoresWritersOutsideTheTeam: the human, another team and the
// definition's own name alone are not this team's writer, so no rule is broken
// (spec-ledger).
func TestValidateIgnoresWritersOutsideTheTeam(t *testing.T) {
	ledgerProject(t, `if [ "$1" = "list" ]; then
  echo '[{"seq":1,"actor":"po","node":"r-1","what":"changed","why":"req approve r-1"},{"seq":2,"actor":"other-lead","node":"r-2","what":"changed","why":"req approve r-2"},{"seq":3,"actor":"demo","node":"r-3","what":"changed","why":"req approve r-3"}]'
  exit 0
fi
`)
	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("validate stopped:\n%s", stdout)
	}
	if strings.Contains(stdout, label("warning")+":") {
		t.Errorf("validate warns about a writer outside the team:\n%s", stdout)
	}
}

// TestValidateSeesTheTeamNameItWasStartedAs: a team started with --as is the
// same definition, so a write by its members is judged, and the warning names
// the team it was started as.
func TestValidateSeesTheTeamNameItWasStartedAs(t *testing.T) {
	f := ledgerProject(t, `if [ "$1" = "list" ]; then
  echo '[{"seq":1,"actor":"demo-2-designer","node":"r-1c42","what":"changed","why":"req approve r-1c42"}]'
  exit 0
fi
`)
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(launchlog.Record{
		Command:  launchlog.CommandUp,
		Team:     "demo-2",
		TeamYAML: filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
	}); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("validate stopped:\n%s", stdout)
	}
	want := i18n.Tf("validate.ledger_breach", "r-1c42", "req approve", "demo-2-designer", "demo-2", "lead")
	if !strings.Contains(stdout, want) {
		t.Errorf("validate does not see the team name it was started as:\nwant %q\ngot:\n%s", want, stdout)
	}
}

// TestValidateSaysTheLedgerWasNotRead: with no gy to read, a definition that
// writes ledger: rules gets one line saying so, not a warning, and the exit code
// does not change (spec-ledger 3).
func TestValidateSaysTheLedgerWasNotRead(t *testing.T) {
	ledgerProject(t, "")
	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("validate stopped when the ledger could not be read:\n%s", stdout)
	}
	want := "  " + i18n.T("label.ledger") + ": " + i18n.T("validate.ledger_unreadable") + "\n"
	if !strings.Contains(stdout, want) {
		t.Errorf("validate does not say the ledger was not read:\nwant %q\ngot:\n%s", want, stdout)
	}
	if strings.Contains(stdout, label("warning")+":") {
		t.Errorf("an unread ledger is printed as a warning:\n%s", stdout)
	}
}

// TestValidateSaysNothingAboutALedgerNoDefinitionWrites: a project whose
// definitions write no ledger: line never starts gy and prints no ledger line,
// so the output of the common case does not change (spec-ledger 5).
func TestValidateSaysNothingAboutALedgerNoDefinitionWrites(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"),
		"version: 1\nsections:\n  - rules:\n      - id: R1\n        text: one\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n  - name: lead\n    role: lead\n    profile: claude-default\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")

	stdout, _, code := run("validate")
	if code != 0 {
		t.Fatalf("validate stopped:\n%s", stdout)
	}
	if strings.Contains(stdout, i18n.T("label.ledger")+":") {
		t.Errorf("validate names a ledger no definition writes:\n%s", stdout)
	}
}

// TestShowPrintsTheLedger: hergy show prints the ledger: rules, so a person
// reads them without opening the file.
func TestShowPrintsTheLedger(t *testing.T) {
	ledgerProject(t, "")
	got := mustRun(t, "show", "demo")
	want := label("ledger") + ": req approve by lead" + i18n.T("list.separator") + "question to human by lead\n"
	if !strings.Contains(got, want) {
		t.Errorf("show does not print the ledger:\nwant %q\ngot:\n%s", want, got)
	}
}

// TestTeamNamesAddsTheNamesTheRecordShows: every name a definition runs under is
// its own name plus the names the launch record shows teams were started as from
// it (--as). A record of another definition, or of no team, adds nothing.
func TestTeamNamesAddsTheNamesTheRecordShows(t *testing.T) {
	f := newFixture(t)
	def := &definition.Definition{
		Name:     "demo",
		TeamYAML: filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []launchlog.Record{
		{Command: launchlog.CommandUp, Team: "demo-2", TeamYAML: def.TeamYAML},
		{Command: launchlog.CommandUp, Team: "demo", TeamYAML: def.TeamYAML}, // its own name: not repeated
		{Command: launchlog.CommandUp, Team: "other", TeamYAML: filepath.Join(f.proj, ".hergy", "teams", "other", "team.yaml")},
		{Command: launchlog.CommandUp, Team: "", TeamYAML: def.TeamYAML},
	} {
		if err := log.Append(record); err != nil {
			t.Fatal(err)
		}
	}
	got := teamNames(def)
	want := []string{"demo", "demo-2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("teamNames = %v, want %v", got, want)
	}
}
