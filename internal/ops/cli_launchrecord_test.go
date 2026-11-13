package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
)

// Up keeps launch identity and workspace history without a blob store.
func TestUpRecordsTheLaunch(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	log, records := launchLog(t, f)
	if len(records) != 1 {
		t.Fatalf("up left %d records", len(records))
	}
	rec := records[0]
	if rec.Command != launchlog.CommandUp || rec.Project.String() != f.proj || rec.Team.String() != "demo" || rec.Time == "" {
		t.Fatalf("launch identity=%+v", rec)
	}
	if want := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"); rec.TeamYAML.String() != want {
		t.Fatalf("definition path=%s want=%s", rec.TeamYAML, want)
	}
	if rec.WorkspaceID == "" {
		t.Fatalf("launch lost workspace: %+v", rec)
	}
	if _, err := os.Stat(filepath.Join(log.Dir(), "blobs")); !os.IsNotExist(err) {
		t.Fatalf("up created blobs: %v", err)
	}
}

func TestLaunchHistoryKeepsUpAndReprompt(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	mustRun(t, "reprompt", "lead", "--team", "demo")
	_, records := launchLog(t, f)
	if len(records) != 2 || records[0].Command != launchlog.CommandUp || records[1].Command != launchlog.CommandReprompt {
		t.Fatalf("launch history=%+v", records)
	}
	for _, record := range records {
		if record.Team.String() != "demo" || record.Project.String() != f.proj {
			t.Fatalf("history lost identity: %+v", record)
		}
	}
}

// TestDryAndFailedLaunchesRecordNothing: a plan that changes nothing and a
// launch that failed are both absent from the record: it holds what ran.
func TestDryAndFailedLaunchesRecordNothing(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	mustRun(t, "up", "demo", "--dry")

	if _, records := launchLog(t, f); len(records) != 0 {
		t.Fatalf("up --dry left %d records, want none", len(records))
	}

	f.herdr(t, failingStartHerdr)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	if _, stderr, code := run("up", "demo"); code == 0 {
		t.Fatalf("up succeeded with an agent that does not start:\n%s", stderr)
	}
	if _, records := launchLog(t, f); len(records) != 0 {
		t.Fatalf("a failed up left %d records, want none", len(records))
	}
}

// TestRefreshRefusesAMemberTheTeamHasNoAgentFor: adding a member to a running
// team is the apply's road now (d-36ef), so refresh names hergy apply and changes
// nothing.
func TestRefreshRefusesAMemberTheTeamHasNoAgentFor(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.addingHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "w0:p1")

	_, stderr, code := run("refresh", "builder")
	if code == 0 {
		t.Fatalf("refresh added a member it should have refused:\n%s", stderr)
	}
	if !strings.Contains(stderr, "hergy apply") {
		t.Fatalf("the refusal does not name hergy apply:\n%s", stderr)
	}
	record := readFile(t, log)
	for _, want := range []string{"pane split", "pane rename", "agent start"} {
		if strings.Contains(record, want) {
			t.Fatalf("the refused refresh ran %q:\n%s", want, record)
		}
	}
	if _, records := launchLog(t, f); len(records) != 0 {
		t.Fatalf("the refused refresh left %d records, want none", len(records))
	}
}

// TestRefreshDryRefusesTheSameMember: --dry shows the same refusal, so a plan
// does not promise a pane the real refresh would not make.
func TestRefreshDryRefusesTheSameMember(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.addingHerdr(t)

	_, stderr, code := run("refresh", "builder", "--dry")
	if code == 0 {
		t.Fatalf("refresh --dry did not refuse:\n%s", stderr)
	}
	if !strings.Contains(stderr, "hergy apply") {
		t.Fatalf("the refusal does not name hergy apply:\n%s", stderr)
	}
	if record := readFile(t, log); strings.Contains(record, "pane split") {
		t.Fatalf("refresh --dry made a pane:\n%s", record)
	}
}

// TestALaunchThatCannotBeRecordedStillRuns: the record is written after the team
// is up, so a record that cannot be written is one line on stderr and the
// command a success.
func TestALaunchThatCannotBeRecordedStillRuns(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	writeFile(t, filepath.Join(f.state, "hergy", "launches"), "not a directory\n")

	stdout, stderr, code := run("up", "demo")
	if code != 0 {
		t.Fatalf("up failed because its record could not be written:\n%s", stderr)
	}
	if !strings.Contains(stdout, "booted:") {
		t.Fatalf("up did not boot the team:\n%s", stdout)
	}
	prefix, _, _ := strings.Cut(i18n.T("launch.not_recorded"), "%v")
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], prefix) {
		t.Fatalf("stderr is %q, want one line starting %q", stderr, prefix)
	}
}

// Hire updates current state without appending to launch history.
func TestHireKeepsStateWithoutChangingHistory(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\ndescription: a description for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: dev\n"+
			"workers:\n  - from: lead\n    purpose: research\n    profile: dev\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	mustRun(t, "up", "dev")
	path := filepath.Join(f.state, "hergy", "launches", "log.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mustRun(t, "hire", "researcher", "--purpose", "research",
		"--task", "read the docs", "--team", "dev", "--from", "lead")

	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("hire changed launch history: %s %v", after, err)
	}
	name, _ := domainname.ParseTeamName("dev")
	state, exists := currentTeamState(name)
	if !exists {
		t.Fatal("hire lost current state")
	}
	workers := state.Workers()
	if len(workers) != 1 || workers[0].Name != "dev-researcher" || workers[0].From != "lead" || workers[0].Purpose != "research" || workers[0].Tab != "worker:dev/researcher" {
		t.Fatalf("hired workers=%+v", workers)
	}
}
