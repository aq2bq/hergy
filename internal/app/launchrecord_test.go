package app

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
)

// launchLog is the record the fixture's commands write, under the temporary root
// that XDG_STATE_HOME points at.
func launchLog(t *testing.T, f *fixture) (launchlog.Log, []launchlog.Record) {
	t.Helper()
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	records, err := log.Read()
	if err != nil {
		t.Fatal(err)
	}
	return log, records
}

// recordDev records one launch of the dev definition from the files on disk,
// the way up does after a boot, and returns what it appended.
func recordDev(t *testing.T, f *fixture) launchlog.Record {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	r, err := readDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "dev", "")
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	recordLaunch(&stderr, env, r, launchlog.CommandUp, r.starts)
	if stderr.Len() > 0 {
		t.Fatalf("recording dev failed: %s", stderr.String())
	}
	_, records := launchLog(t, f)
	if len(records) == 0 {
		t.Fatal("recording dev wrote no record")
	}
	return records[len(records)-1]
}

// TestUpRecordsTheLaunch: a successful boot leaves one line with what was
// handed out, and every text the line points at is in blobs/ at its full text.
func TestUpRecordsTheLaunch(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	mustRun(t, "up", "demo")

	log, records := launchLog(t, f)
	if len(records) != 1 {
		t.Fatalf("up left %d records, want 1", len(records))
	}
	rec := records[0]
	if rec.Command != launchlog.CommandUp {
		t.Errorf("command = %q, want %q", rec.Command, launchlog.CommandUp)
	}
	if rec.Project != f.proj {
		t.Errorf("project = %q, want %q", rec.Project, f.proj)
	}
	if rec.Layer != source.LayerLocal || rec.Definition != "demo" || rec.Team != "demo" {
		t.Errorf("layer/definition/team = %q/%q/%q, want local/demo/demo", rec.Layer, rec.Definition, rec.Team)
	}
	if want := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"); rec.TeamYAML != want {
		t.Errorf("team_yaml = %q, want %q", rec.TeamYAML, want)
	}
	if len(rec.Hash) != 64 {
		t.Errorf("hash = %q, want the sha256 of the launched team in hex", rec.Hash)
	}
	if rec.Time == "" || rec.Hergy == "" {
		t.Errorf("time/hergy = %q/%q, want the time and which hergy ran", rec.Time, rec.Hergy)
	}

	// The members that were handed a prompt: the whole definition, and each
	// prompt is in blobs/ at its own text.
	wantMembers := map[string]string{"demo-lead": "lead", "demo-builder": "builder"}
	if len(rec.Members) != len(wantMembers) {
		t.Fatalf("members = %v, want both members of the definition", rec.Members)
	}
	prompts := map[string]string{}
	for _, member := range rec.Members {
		short, ok := wantMembers[member.Name]
		if !ok || member.Short != short {
			t.Errorf("member %q with short name %q, want one of %v", member.Name, member.Short, wantMembers)
		}
		text, err := log.Text(member.Prompt)
		if err != nil {
			t.Fatalf("the prompt of %s is not in blobs/: %v", member.Name, err)
		}
		prompts[member.Name] = string(text)
	}
	if prompts["demo-lead"] == "" || prompts["demo-lead"] == prompts["demo-builder"] {
		t.Errorf("the members' prompts are %q and %q, want two texts",
			prompts["demo-lead"], prompts["demo-builder"])
	}

	// The rule files those prompts were built from: the role file and the
	// documents, once each, each at its whole text.
	wantDocs := []string{
		filepath.Join(f.proj, ".hergy", "roles", "builder.yaml"),
		filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"),
		filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml"),
	}
	gotDocs := make([]string, len(rec.Docs))
	for i, doc := range rec.Docs {
		gotDocs[i] = doc.Path
		if doc.Layer != source.LayerLocal {
			t.Errorf("the document %s carries layer %q, want local", doc.Path, doc.Layer)
		}
		text, err := log.Text(doc.SHA256)
		if err != nil {
			t.Fatalf("the text of %s is not in blobs/: %v", doc.Path, err)
		}
		if want := readFile(t, doc.Path); string(text) != want {
			t.Errorf("the record holds another text for %s than the file has:\n%s", doc.Path, text)
		}
	}
	sort.Strings(gotDocs)
	if strings.Join(gotDocs, "\n") != strings.Join(wantDocs, "\n") {
		t.Errorf("the record names the files\n%s\nwant\n%s", strings.Join(gotDocs, "\n"), strings.Join(wantDocs, "\n"))
	}
}

// TestTheRecordFollowsWhatTheCommandHandedOut: up hands a prompt to every member
// and reprompt to the member it was asked for, while the hash is the hash of the
// whole definition either way, so two commands of one team line up.
func TestTheRecordFollowsWhatTheCommandHandedOut(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	mustRun(t, "up", "demo")
	mustRun(t, "reprompt", "lead", "--team", "demo")

	_, records := launchLog(t, f)
	if len(records) != 2 {
		t.Fatalf("up and reprompt left %d records, want 2", len(records))
	}
	if got := len(records[0].Members); got != 2 {
		t.Errorf("up recorded %d members, want both", got)
	}
	last := records[1]
	if last.Command != launchlog.CommandReprompt || len(last.Members) != 1 || last.Members[0].Name != "demo-lead" {
		t.Errorf("reprompt recorded %q with %v, want one prompt to demo-lead", last.Command, last.Members)
	}
	if records[0].Hash != last.Hash {
		t.Errorf("one definition hashed %q at up and %q at reprompt",
			records[0].Hash, last.Hash)
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

	// A boot whose agent does not start fails, and records nothing.
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

// failingStartHerdr is a fake herdr whose agent start fails, so a boot stops
// before it has started anything.
const failingStartHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "agent list")   echo '{"result":{"agents":[]}}' ;;
  "tab list")     echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo@demo"}]}}' ;;
  "tab create")   echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")   echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  "agent start")  echo '{"error":{"code":"start_failed"}}'; exit 1 ;;
  *) echo '{"result":{}}' ;;
esac
`

// TestALaunchThatCannotBeRecordedStillRuns: the record is written after the team
// is up, so a record that cannot be written is one line on stderr and the
// command a success.
func TestALaunchThatCannotBeRecordedStillRuns(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	// State has no room: the path the record would go to is a file.
	blocked := filepath.Join(filepath.Dir(f.state), "blocked")
	writeFile(t, blocked, "not a directory\n")
	t.Setenv("XDG_STATE_HOME", blocked)

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

// TestTheHashFollowsTheProfilesTheMembersRead: the team's hash covers the
// members after their profiles are resolved, so a change to a profile nobody
// reads leaves it alone and a change to one a member reads moves it.
func TestTheHashFollowsTheProfilesTheMembersRead(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: fast\n"+
			"  - name: builder\n    role: builder\n    profile: fast\n"+
			"turns:\n  - human -> lead\n  - lead -> builder\n  - builder -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), profilesYAML("--fast", "--slow"))

	before := recordDev(t, f).Hash

	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), profilesYAML("--fast", "--slower"))
	if got := recordDev(t, f).Hash; got != before {
		t.Errorf("a profile no member reads changed the hash: %q, want %q", got, before)
	}

	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), profilesYAML("--faster", "--slow"))
	if got := recordDev(t, f).Hash; got == before {
		t.Errorf("a profile a member reads left the hash at %q", got)
	}
}

// profilesYAML is one profiles.yaml with a used profile (fast, read by the dev
// definition's lead) and one nobody reads (slow), each with the args given.
func profilesYAML(fast, slow string) string {
	return "version: 1\nprofiles:\n" +
		"  fast:\n    kind: claude\n    args: [" + fast + "]\n" +
		"  slow:\n    kind: codex\n    args: [" + slow + "]\n"
}

// TestTheRecordKeepsARevision: a rule file that gives itself a revision carries
// it into the record, which is what the screens show as the document's name for
// its current shape.
func TestTheRecordKeepsARevision(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "dev", "agreement.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 3\nsections:\n  - rules:\n      - id: P1\n        text: one\n")

	rec := recordDev(t, f)
	for _, doc := range rec.Docs {
		if doc.Path != protocol {
			continue
		}
		if doc.Revision != 3 {
			t.Errorf("the record gives %s revision %d, want 3", protocol, doc.Revision)
		}
		return
	}
	t.Fatalf("the record does not name %s:\n%v", protocol, rec.Docs)
}
