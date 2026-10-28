package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/source"
)

// passingHerdr is a fake herdr that answers every command the CLI may need, so
// a boot, a refresh or a down runs to the end.
const passingHerdr = `#!/bin/sh
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
  "pane list")    echo '{"result":{"panes":[]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// liveHerdr is a fake herdr that reports the demo team's two members as live, so
// a down has panes to close.
const liveHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1"},{"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2"}]}}' ;;
  "agent get")
    case "$3" in
      demo-lead)   echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1"}}}' ;;
      demo-builder) echo '{"result":{"agent":{"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2"}}}' ;;
      *) echo '{"error":{"code":"agent_not_found"}}' ;;
    esac
    ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo@demo"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// statusHerdr is a fake herdr that reports the demo team's two members and one
// hired worker, with states a status run reports: one working, one blocked, and
// the worker idle. Its team tab uses the older label form, "team:demo", so a
// team command also proves it reads a tab with no team definition name.
const statusHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1"},{"name":"demo-builder","agent":"claude","agent_status":"blocked","pane_id":"p2","tab_id":"t1"},{"name":"demo-w1","agent":"claude","agent_status":"idle","pane_id":"p3","tab_id":"tw"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo"},{"tab_id":"tw","label":"worker:demo/w1"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// absentHerdr is a fake herdr with only demo-lead live and idle, so the other
// member reports as not present.
const absentHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// noPaneHerdr is a fake herdr where the demo team's members have no live agent
// and no pane is named after them.
const noPaneHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent get") echo '{"error":{"code":"agent_not_found"}}' ;;
  "pane list") echo '{"result":{"panes":[]}}' ;;
  "tab list")  echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo@demo"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// fixture is one temporary project with a global config directory outside the
// home directory, so paths inside the project go relative and paths under the
// config directory stay absolute.
type fixture struct {
	proj  string
	cfg   string
	state string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{proj: filepath.Join(root, "proj"), cfg: filepath.Join(root, "cfg")}
	f.state = filepath.Join(root, "state")
	for _, dir := range []string{f.proj, f.cfg} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", f.proj, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	t.Chdir(f.proj)
	t.Setenv("XDG_CONFIG_HOME", f.cfg)
	// The launch record is state: it is kept out of the config directory, and a
	// test writes it inside the temporary root like everything else.
	t.Setenv("XDG_STATE_HOME", f.state)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("HERDR_ENV", "")
	t.Setenv("HERDR_PANE_ID", "")
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "no-herdr"))
	// gy is reached the same way: a test that wants the questions writes a fake
	// gy and points HERGY_GY at it (段 6).
	t.Setenv("HERGY_GY", filepath.Join(f.cfg, "no-gy"))
	return f
}

// herdr writes a fake herdr and points HERGY_HERDR at it.
func (f *fixture) herdr(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(f.cfg, "herdr")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_HERDR", path)
}

// statefulHerdr writes a fake herdr that remembers the agents it started and the
// tabs it created, so a second boot under the same names collides in preflight
// the way real herdr would, and a team command can find the team's tab. It
// records every call it gets in <cfg>/herdr.log and returns that path, so a test
// can check a tab label or a closed pane.
func (f *fixture) statefulHerdr(t *testing.T) string {
	t.Helper()
	state := filepath.Join(f.cfg, "agents.txt")
	tabs := filepath.Join(f.cfg, "tabs.txt")
	log := filepath.Join(f.cfg, "herdr.log")
	for _, path := range []string{state, tabs} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
state='` + state + `'
tabs='` + tabs + `'
log='` + log + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "agent list")
    printf '{"result":{"agents":['
    first=1
    while read -r tab n; do
      [ -z "$n" ] && continue
      if [ $first -eq 1 ]; then first=0; else printf ','; fi
      printf '{"name":"%s","agent":"claude","agent_status":"idle","pane_id":"p-%s","tab_id":"%s"}' "$n" "$n" "$tab"
    done < "$state"
    printf ']}}'
    ;;
  "agent start")
    name="$3"
    if grep -q " $name$" "$state"; then
      echo '{"error":{"code":"name_in_use"}}'
      exit 1
    fi
    tab=$(tail -n 1 "$tabs" | cut -d' ' -f1)
    printf '%s %s\n' "$tab" "$name" >> "$state"
    echo '{"result":{}}'
    ;;
  "agent get")
    name="$3"
    case "$3" in
      p-*) name="${3#p-}" ;;
    esac
    tab=$(awk -v n="$name" '$2 == n { print $1; exit }' "$state")
    if [ -n "$tab" ]; then
      printf '{"result":{"agent":{"name":"%s","agent":"claude","agent_status":"idle","pane_id":"p-%s","tab_id":"%s"}}}\n' "$name" "$name" "$tab"
    else
      echo '{"error":{"code":"agent_not_found"}}'
    fi
    ;;
  "tab list")
    printf '{"result":{"tabs":['
    first=1
    while read -r id label; do
      [ -z "$id" ] && continue
      if [ $first -eq 1 ]; then first=0; else printf ','; fi
      printf '{"tab_id":"%s","label":"%s"}' "$id" "$label"
    done < "$tabs"
    printf ']}}'
    ;;
  "pane list") echo '{"result":{"panes":[]}}' ;;
  "tab create")
    label=""
    prev=""
    for a in "$@"; do
      if [ "$prev" = "--label" ]; then label="$a"; fi
      prev="$a"
    done
    n=$(wc -l < "$tabs" | tr -d ' ')
    n=$((n + 1))
    printf 't%s %s\n' "$n" "$label" >> "$tabs"
    printf '{"result":{"tab":{"tab_id":"t%s"},"root_pane":{"pane_id":"p1"}}}' "$n"
    ;;
  "pane split") echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return log
}

// runningHerdr writes a fake herdr that reports the given team tabs and live
// agents, so a team command can find the team definition behind a team name.
// tabs and agents are JSON object fragments (no brackets). It also answers the
// calls a status or a down needs.
func (f *fixture) runningHerdr(t *testing.T, tabs, agents string) {
	t.Helper()
	script := `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list")     echo '{"result":{"tabs":[` + tabs + `]}}' ;;
  "agent list")   echo '{"result":{"agents":[` + agents + `]}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "tab create")   echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")   echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
}

// refreshDeadHerdr writes a fake herdr where the demo team's lead has no live
// agent, but the pane hergy named after it is still there at its shell. It
// records every call in <cfg>/herdr.log and returns that path.
func (f *fixture) refreshDeadHerdr(t *testing.T) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
	script := `#!/bin/sh
log='` + log + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent get")         echo '{"error":{"code":"agent_not_found"}}' ;;
  "pane list")         echo '{"result":{"panes":[{"pane_id":"w1:p9","label":"demo-lead"}]}}' ;;
  "pane process-info") echo '{"result":{"process_info":{"foreground_process_group_id":900,"shell_pid":900,"pane_id":"w1:p9"}}}' ;;
  "tab list")          echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo@demo"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return log
}

// addingHerdr writes a fake herdr where the demo team is running with demo-lead
// at p1, and demo-builder has no live agent and no pane named after it: a refresh
// of builder adds it to the team's tab. It records every call in <cfg>/herdr.log
// and returns that path.
func (f *fixture) addingHerdr(t *testing.T) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
	script := `#!/bin/sh
log='` + log + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent get")
    case "$3" in
      demo-lead) echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}}}' ;;
      *) echo '{"error":{"code":"agent_not_found"}}' ;;
    esac
    ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo@demo"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[{"pane_id":"p1","label":"demo-lead","tab_id":"t1"}]}}' ;;
  "pane split") echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return log
}

// readFile returns a file's contents, failing the test when it is missing.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// writeFile writes a file, creating its directories.
func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// run invokes one command line and returns what a person would see.
func run(argv ...string) (stdout, stderr string, code int) {
	var out, errBuf bytes.Buffer
	code = Run(argv, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// mustRun runs a command that is expected to succeed.
func mustRun(t *testing.T, argv ...string) string {
	t.Helper()
	stdout, stderr, code := run(argv...)
	if code != 0 {
		t.Fatalf("%v: exit %d\nstderr:\n%s", argv, code, stderr)
	}
	return stdout
}

func wantLines(t *testing.T, got string, want ...string) {
	t.Helper()
	wantText := strings.Join(want, "\n") + "\n"
	if got != wantText {
		t.Fatalf("output differs\n got:\n%s\nwant:\n%s", got, wantText)
	}
}

func TestNewShape(t *testing.T) {
	f := newFixture(t)
	got := mustRun(t, "new", "demo")
	wantLines(t, got,
		"team definition: demo",
		"dir: .hergy/teams/demo",
		"file: .hergy/teams/demo/team.yaml",
		"file: .hergy/teams/demo/agreement.yaml",
		"role document: .hergy/roles/lead.yaml",
		"role document: .hergy/roles/builder.yaml",
		"profile: claude-default",
		"file: .hergy/profiles.yaml",
		"config: "+filepath.Join(f.cfg, "hergy", "config.yaml"),
		"next: up demo --dry",
	)
}

// TestNewCreatesTheGlobalConfigWhenMissing: a first new writes the global
// config.yaml (and the hergy/ directory) with version: 1 and the run's language,
// and reports the file as one more output line.
func TestNewCreatesTheGlobalConfigWhenMissing(t *testing.T) {
	f := newFixture(t)
	got := mustRun(t, "new", "demo")
	path := filepath.Join(f.cfg, "hergy", "config.yaml")
	wantLines(t, readFile(t, path), "version: 1", "lang: en")
	if !strings.Contains(got, "config: "+path+"\n") {
		t.Fatalf("new does not report the config.yaml it made:\n%s", got)
	}
}

// TestNewConfigLanguageFollowsTheRun: --lang decides the lang the created
// config.yaml names; without --lang the default, en, is used.
func TestNewConfigLanguageFollowsTheRun(t *testing.T) {
	t.Run("--lang ja", func(t *testing.T) {
		f := newFixture(t)
		mustRun(t, "new", "demo", "--lang", "ja")
		wantLines(t, readFile(t, filepath.Join(f.cfg, "hergy", "config.yaml")), "version: 1", "lang: ja")
	})
	t.Run("no --lang", func(t *testing.T) {
		f := newFixture(t)
		mustRun(t, "new", "demo")
		wantLines(t, readFile(t, filepath.Join(f.cfg, "hergy", "config.yaml")), "version: 1", "lang: en")
	})
}

// TestNewLeavesAnExistingConfigAlone: when config.yaml is already there, new
// neither rewrites it nor reports it, whatever --lang says.
func TestNewLeavesAnExistingConfigAlone(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.cfg, "hergy", "config.yaml")
	writeFile(t, path, "version: 1\nlang: ja\n")
	got := mustRun(t, "new", "demo", "--lang", "en")
	if after := readFile(t, path); after != "version: 1\nlang: ja\n" {
		t.Fatalf("new rewrote an existing config.yaml:\n%s", after)
	}
	if strings.Contains(got, "config:") {
		t.Fatalf("new reports a config.yaml it did not make:\n%s", got)
	}
}

// TestLsAfterNewUsesTheCreatedLanguage: once new has written lang: ja, a later
// command with no --lang reads Japanese.
func TestLsAfterNewUsesTheCreatedLanguage(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo", "--lang", "ja")
	got := mustRun(t, "ls")
	if !strings.Contains(got, "チーム定義: demo\n") {
		t.Fatalf("ls after a Japanese new is not Japanese:\n%s", got)
	}
	if !strings.Contains(got, "  パス: .hergy/teams/demo/team.yaml\n") {
		t.Fatalf("ls after a Japanese new is not Japanese:\n%s", got)
	}
}

// TestValidateAcceptsTheCreatedConfig: the config.yaml new writes is one
// validate reads without complaint.
func TestValidateAcceptsTheCreatedConfig(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	path := filepath.Join(f.cfg, "hergy", "config.yaml")
	got := mustRun(t, "validate")
	if !strings.Contains(got, "file: "+path+"\n  result: ok\n") {
		t.Fatalf("validate does not accept the created config.yaml:\n%s", got)
	}
	if !strings.Contains(got, "problems: 0\n") {
		t.Fatalf("validate found a problem in the created config.yaml:\n%s", got)
	}
}

// TestValidateSaysALeftoverStyleOnce: the leftover-style warning names one file
// and one section, and a file several definitions read is one line to delete, so
// validate says it once for the whole run rather than once per definition
// (d-f4f6).
func TestValidateSaysALeftoverStyleOnce(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	mustRun(t, "new", "demo2")
	// The starter role document keeps one style: line — the shape this release
	// takes out — and both definitions read it, because a role document is
	// looked up by the role's name.
	path := filepath.Join(".hergy", "roles", "lead.yaml")
	text := readFile(t, path)
	writeFile(t, path, strings.Replace(text, "sections:\n  - rules:", "sections:\n  - style: ids\n    rules:", 1))

	got := mustRun(t, "validate")
	if n := strings.Count(got, "file: .hergy/teams/"); n != 2 {
		t.Fatalf("validate read %d definitions, want the 2 that read the role document:\n%s", n, got)
	}
	want := i18n.Tf("rules.style_gone", ".hergy/roles/lead.yaml", "sections[0]")
	want = strings.TrimPrefix(want, ".hergy/roles/lead.yaml: ")
	if n := strings.Count(got, want); n != 1 {
		t.Errorf("validate says the leftover-style sentence %d times, want once:\n%s", n, got)
	}
	if !strings.Contains(got, "problems: 0\n") {
		t.Errorf("a leftover style: line is not a problem:\n%s", got)
	}
}

// TestNewWritesTheProfileTheStarterTeamNames: the starter team's members name a
// profile, so new writes that profile into the layer's profiles.yaml when the
// layer cannot already resolve it (I7). The name says what it picks rather than
// the kind it runs, so validate's name warning has nothing to say about it.
func TestNewWritesTheProfileTheStarterTeamNames(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	wantLines(t, readFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml")),
		"version: 1", "profiles:", "  claude-default:", "    kind: claude")
	if got := readFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")); !strings.Contains(got, "profile: claude-default") {
		t.Errorf("the starter team.yaml does not name the profile:\n%s", got)
	}
	if got := mustRun(t, "validate"); !strings.Contains(got, "problems: 0\n") {
		t.Fatalf("validate found a problem in the definition new wrote:\n%s", got)
	}
}

// TestNewKeepsAnExistingProfile: a layer that already resolves the name keeps its
// own profiles.yaml as it is, so a starter team is not a reason to write over a
// file that holds more than the starter profile.
func TestNewKeepsAnExistingProfile(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.proj, ".hergy", "profiles.yaml")
	writeFile(t, path, "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    args: [--model, opus]\n")
	mustRun(t, "new", "demo")
	if got := readFile(t, path); !strings.Contains(got, "--model") {
		t.Errorf("new rewrote a profiles.yaml that already names the profile:\n%s", got)
	}
}

// TestValidateWarnsAboutAProfileNamedAfterItsKind: a profile named after its
// kind looks like a choice of kind, where it is a choice of what the engine runs
// (I7), so validate says so under the file that holds the profile (d-a477). It
// is a warning: validate still finds no problem.
func TestValidateWarnsAboutAProfileNamedAfterItsKind(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n  claude:\n    kind: claude\n")
	got := mustRun(t, "validate")
	warning := "  warning: " + i18n.Tf("validate.profile_named_after_kind", "claude") + "\n"
	if want := "file: .hergy/profiles.yaml\n  result: ok\n" + warning; !strings.Contains(got, want) {
		t.Fatalf("validate does not warn under the profiles.yaml:\n%s", got)
	}
	if strings.Contains(got, i18n.Tf("validate.profile_named_after_kind", "claude-default")) {
		t.Errorf("validate warns about a name that is not its kind:\n%s", got)
	}
	if !strings.Contains(got, "problems: 0\n") {
		t.Errorf("the warning moved the exit code:\n%s", got)
	}
}

// TestValidateWarnsAboutThePlacesHergyNoLongerReads: 段 4 moved a team's
// agreement to agreement.yaml and a role document to the layer's roles/. A file
// left in one of the old places is not read, so validate names it and where it
// goes now, once per file, and moves no exit code (spec/32 D1). Each of the old
// places is tried on its own, so a fix that forgets one is still a warning here.
func TestValidateWarnsAboutThePlacesHergyNoLongerReads(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		args  []any
		write func(t *testing.T, f *fixture)
	}{
		{
			name: "a protocol.yaml that was not renamed",
			key:  "validate.old_team_agreement",
			args: []any{".hergy/teams/demo/protocol.yaml", ".hergy/teams/demo/agreement.yaml"},
			write: func(t *testing.T, f *fixture) {
				writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml"), "version: 1\nsections: []\n")
			},
		},
		{
			name: "a role document left in the definition's folder",
			key:  "validate.old_team_role",
			args: []any{".hergy/teams/demo/roles/keep.yaml", ".hergy/roles/keep.yaml"},
			write: func(t *testing.T, f *fixture) {
				writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "roles", "keep.yaml"), "version: 1\nsections: []\n")
			},
		},
		{
			name: "a common/ directory",
			key:  "validate.old_common",
			args: []any{".hergy/common"},
			write: func(t *testing.T, f *fixture) {
				writeFile(t, filepath.Join(f.proj, ".hergy", "common", "old.yaml"), "version: 1\nsections: []\n")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "demo")
			tc.write(t, f)
			got := mustRun(t, "validate")
			want := i18n.Tf(tc.key, tc.args...)
			if !strings.Contains(got, "warning: "+want+"\n") {
				t.Fatalf("validate does not warn about %s:\n%s", tc.name, got)
			}
			if !strings.Contains(got, "problems: 0\n") {
				t.Errorf("the warning about %s moved the exit code:\n%s", tc.name, got)
			}
		})
	}
}

// TestValidateNamesEveryPlaceItLookedForARoleDocument: a missing role document
// names every place it was looked for, in the one order a name is looked up
// (spec/32 D2). A person moving a definition over sees both, rather than one
// place that may already hold the file.
func TestValidateNamesEveryPlaceItLookedForARoleDocument(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n  - name: lead\n    role: absent\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n")

	stdout, _, code := run("validate")
	if code == 0 {
		t.Fatalf("validate accepted a missing role document:\n%s", stdout)
	}
	places := strings.Join([]string{
		".hergy/roles/absent.yaml",
		filepath.Join(f.cfg, "hergy", "roles", "absent.yaml"),
	}, i18n.T("list.separator"))
	want := i18n.Tf("error.no_role_doc", "lead", "absent", places)
	if !strings.Contains(stdout, want) {
		t.Errorf("validate does not name every place it looked (%q):\n%s", want, stdout)
	}
}

// TestValidateNamesTheFileAndMemberForATemplateMissingADoor: a prompt template
// that drops a door stops (I4), and the sentence names the team.yaml it read and,
// when it is a member's own prompt:, the member (spec/32 D2).
func TestValidateNamesTheFileAndMemberForATemplateMissingADoor(t *testing.T) {
	const member = "members:\n  - name: lead\n    role: lead\n    profile: claude-default\n" +
		"turns:\n  - human -> lead\n  - lead -> human\n"
	cases := []struct {
		name, team string
		whereKey   string
		whereArgs  []any
	}{
		{
			name:     "the team's prompt:",
			team:     "version: 1\nmission: a mission for the test\nprompt: \"%{role_doc}\"\n" + member,
			whereKey: "prompt.template_team",
		},
		{
			name:      "a member's prompt:",
			team:      "version: 1\nmission: a mission for the test\n" + strings.Replace(member, "    profile: claude-default\n", "    profile: claude-default\n    prompt: \"%{role_doc}\"\n", 1),
			whereKey:  "prompt.template_member",
			whereArgs: []any{"lead"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
			writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"), "version: 1\nsections:\n  - rules:\n      - id: L1\n        text: one\n")
			writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"), tc.team)

			stdout, _, code := run("validate")
			if code == 0 {
				t.Fatalf("validate accepted a template with a door missing:\n%s", stdout)
			}
			where := i18n.Tf(tc.whereKey, tc.whereArgs...)
			want := i18n.Tf("error.prompt_missing_part", ".hergy/teams/demo/team.yaml", where, "docs")
			if !strings.Contains(stdout, want) {
				t.Errorf("validate does not name the file and %s (%q):\n%s", tc.name, want, stdout)
			}
		})
	}
}

// TestValidatePrintsOnePromptBreakAtATime: a definition whose two members have no
// role document stops, and hergy validate prints the first stop alone with a
// problems count of one, as it always has, while a check hands over one stop for
// each member so a screen shows them all at once (N1).
func TestValidatePrintsOnePromptBreakAtATime(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"), "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: absent\n    profile: claude-default\n"+
			"  - name: spec\n    role: gone\n    profile: claude-default\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n  - spec -> human\n")

	got, _, code := run("validate")
	if code == 0 {
		t.Fatalf("validate accepted two members with no role document:\n%s", got)
	}
	if count := strings.Count(got, "  detail: "); count != 1 {
		t.Errorf("validate printed %d details, want the first stop alone:\n%s", count, got)
	}
	if !strings.Contains(got, "problems: 1\n") {
		t.Errorf("validate does not count one problem, as it always has:\n%s", got)
	}
	if !strings.Contains(got, "the role absent") {
		t.Errorf("validate does not print the first member's stop:\n%s", got)
	}
	if strings.Contains(got, "the role gone") {
		t.Errorf("validate printed the second member's stop too:\n%s", got)
	}
}

// TestUpStopsOnAGraphTheDefinitionCannotHold: a definition whose turns cannot
// hold stops hergy up the way a file that does not read does, with every
// sentence hergy validate prints, rather than booting a team whose turns the
// definition does not say (I12a, I12b, 段 5, d-edd4).
func TestUpStopsOnAGraphTheDefinitionCannotHold(t *testing.T) {
	f := newFixture(t)
	// Two members and no turns: each of them, and the human, hands the turn to
	// nobody, and no turn reaches the human.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: dev\n"+
			"  - name: builder\n    role: builder\n    profile: dev\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")

	want := []string{
		"the human hands the turn to nobody",
		"lead hands the turn to nobody",
		"builder hands the turn to nobody",
		"no turn reaches the human",
	}
	stdout, stderr, code := run("up", "demo", "--dry")
	if code == 0 {
		t.Fatal("up --dry succeeded with a graph the definition cannot hold")
	}
	if stdout != "" {
		t.Errorf("up --dry wrote to stdout:\n%s", stdout)
	}
	for _, sentence := range want {
		if !strings.Contains(stderr, sentence) {
			t.Errorf("up --dry does not say %q:\n%s", sentence, stderr)
		}
	}
	// The file the definition was read from is named once, up in front (D2):
	// a command that takes a team name does not know which team.yaml that is.
	teamYAML := filepath.Join(".hergy", "teams", "demo", "team.yaml")
	if got := strings.Count(stderr, teamYAML); got != 1 {
		t.Errorf("up --dry names %s %d times, want it once at the front:\n%s", teamYAML, got, stderr)
	}

	// hergy validate prints the same sentences, one detail line each, and the
	// file once on a line (not twice: breakLine writes it).
	out, _, code := run("validate")
	if code == 0 {
		t.Fatal("validate found no problem in a graph the definition cannot hold")
	}
	for _, sentence := range want {
		if !strings.Contains(out, sentence) {
			t.Errorf("validate does not say %q:\n%s", sentence, out)
		}
	}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, want[0]) && strings.Count(line, teamYAML) != 1 {
			t.Errorf("validate's detail line names %s %d times, want once:\n%s", teamYAML, strings.Count(line, teamYAML), line)
		}
	}

	// The commands that build a prompt stop the same way: each reads the
	// definition before it touches herdr (d-edd4).
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:demo@demo"}`, "")
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	for _, argv := range [][]string{
		{"refresh", "lead", "--team", "demo"},
		{"reprompt", "lead", "--team", "demo"},
		{"hire", "researcher", "--purpose", "build", "--task", "read", "--team", "demo", "--dry"},
	} {
		_, stderr, code := run(argv...)
		if code == 0 {
			t.Errorf("%v succeeded with a graph the definition cannot hold", argv)
			continue
		}
		if !strings.Contains(stderr, want[0]) {
			t.Errorf("%v does not stop with the graph's sentences:\n%s", argv, stderr)
		}
	}
}

func TestLsShape(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	got := mustRun(t, "ls")
	wantLines(t, got,
		"team definition: demo",
		"  path: .hergy/teams/demo/team.yaml",
	)
}

// TestNoOutputNamesTheLayer: the layer is hergy's own word and a person reads
// where a definition is by its path, so no command names the layer (I11, 段 7).
func TestNoOutputNamesTheLayer(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	for _, argv := range [][]string{
		{"ls"},
		{"show", "demo"},
		{"up", "demo", "--dry"},
		{"new", "fresh"},
	} {
		out := mustRun(t, argv...)
		for _, word := range []string{"layer", "層"} {
			if strings.Contains(out, word) {
				t.Errorf("%v names the layer (%q):\n%s", argv, word, out)
			}
		}
	}
}

func TestValidateShape(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	got := mustRun(t, "validate")
	wantLines(t, got,
		"file: "+filepath.Join(f.cfg, "hergy", "config.yaml"),
		"  result: ok",
		"file: .hergy/profiles.yaml",
		"  result: ok",
		"file: .hergy/teams/demo/team.yaml",
		"  result: ok",
		"  warning: "+i18n.T("validate.mission_placeholder"),
		"file: .hergy/roles/builder.yaml",
		"  result: ok",
		"file: .hergy/roles/lead.yaml",
		"  result: ok",
		"checked: 5",
		"problems: 0",
		"rules 6 in 3 files",
		i18n.T("validate.size_heading"),
		"  team .hergy/teams/demo: 198 characters (the provisional budget is 3000)",
		"  role .hergy/roles/builder.yaml: 236 characters (the provisional budget is 3000)",
		"  role .hergy/roles/lead.yaml: 174 characters (the provisional budget is 3000)",
		"This budget is provisional, for cutting team and role definitions; it does not stop hergy up.",
	)
}

// TestSharedAgreementIsNotReadAndStops: the agreement every team used to share
// (agreement.yaml beside the teams, in either place) does not reach a member's
// prompt any more; while one is left, the commands that build a prompt and
// validate stop on it, name the path and say where the rules go (d-31ad,
// r-7de2, ac-b7b6).
func TestSharedAgreementIsNotReadAndStops(t *testing.T) {
	shared := "version: 1\ntitle: 共通\nsections:\n  - heading: 台帳\n    rules:\n      - id: S1\n        text: 全員が読む。\n"
	for _, place := range []string{"project", "global"} {
		t.Run(place, func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "demo")
			path := filepath.Join(f.proj, ".hergy", "agreement.yaml")
			shown := ".hergy/agreement.yaml"
			if place == "global" {
				path = filepath.Join(f.cfg, "hergy", "agreement.yaml")
				shown = path
			}
			writeFile(t, path, shared)

			// The projection does not read it: the rule stands in no member's
			// prompt, even though the file is there.
			cat := catalog.New(filepath.Join(f.cfg, "hergy"), filepath.Join(f.proj, ".hergy"), f.proj, source.Disk{}, func(path string) string { return path })
			r, err := readDefinition(cat, f.proj, "demo", "")
			if err != nil {
				t.Fatalf("the definition does not read: %v", err)
			}
			for _, start := range r.starts {
				if strings.Contains(start.Prompt, "S1") || strings.Contains(start.Prompt, "全員が読む") {
					t.Errorf("%s's prompt carries the shared agreement:\n%s", start.Name, start.Prompt)
				}
			}

			// The commands that build a prompt stop, and say where the rules go.
			for _, argv := range [][]string{
				{"up", "demo", "--dry"},
				{"up", "demo"},
				{"hire", "w", "--purpose", "p", "--task", "t", "--team", "demo", "--dry"},
				{"reprompt", "lead", "--team", "demo", "--dry"},
			} {
				_, stderr, code := run(argv...)
				if code == 0 {
					t.Errorf("%v succeeded with the shared agreement left", argv)
					continue
				}
				for _, want := range []string{shown, "teams/<definition>/agreement.yaml", "roles/<role>.yaml"} {
					if !strings.Contains(stderr, want) {
						t.Errorf("%v does not name %q:\n%s", argv, want, stderr)
					}
				}
			}

			// The placeholders in the sentence are the language's own (T13 R2):
			// Japanese reads <チーム定義名> and <ロール名>, not the English words.
			_, stderr, code := run("up", "demo", "--dry", "--lang", "ja")
			if code == 0 {
				t.Fatal("up --dry --lang ja did not stop on the shared agreement")
			}
			for _, want := range []string{"<チーム定義名>", "<ロール名>"} {
				if !strings.Contains(stderr, want) {
					t.Errorf("the Japanese sentence does not read %q:\n%s", want, stderr)
				}
			}
			if strings.Contains(stderr, "<definition>") || strings.Contains(stderr, "<role>") {
				t.Errorf("the Japanese sentence carries an English placeholder:\n%s", stderr)
			}

			// validate names the file and stops with exit 1.
			stdout, _, code := run("validate")
			if code == 0 {
				t.Fatal("validate did not stop on the shared agreement")
			}
			if !strings.Contains(stdout, "file: "+shown+"\n") || !strings.Contains(stdout, "problems: 1\n") {
				t.Errorf("validate does not name the file and count it:\n%s", stdout)
			}
			if strings.Contains(stdout, "S1") {
				t.Errorf("validate counts a rule of the shared agreement:\n%s", stdout)
			}

			// Moved where it belongs, the same rule reaches the member.
			writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml"), shared)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if got := mustRun(t, "up", "demo", "--dry"); !strings.Contains(got, "S1") {
				t.Errorf("the rule moved to the team's agreement does not reach the prompt:\n%s", got)
			}
		})
	}
}

func TestValidateShapeReportsABrokenFile(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	if err := os.WriteFile(path, []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _, code := run("validate")
	if code == 0 {
		t.Fatalf("validate succeeded on a broken file:\n%s", got)
	}
	if !strings.Contains(got, "file: .hergy/teams/demo/team.yaml\n  result: broken\n") {
		t.Fatalf("validate does not report the break in the one shape:\n%s", got)
	}
	if !strings.Contains(got, "checked: 5\nproblems: 1\n") {
		t.Fatalf("validate summary is not in the one shape:\n%s", got)
	}
}

func TestShowShape(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	got := mustRun(t, "show", "demo")
	wantLines(t, got,
		"mission: what this team is for, in one line",
		"team definition: demo",
		"version: 1",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: .hergy/profiles.yaml",
		"document: .hergy/teams/demo/agreement.yaml",
		"member: demo-lead",
		"  role: lead",
		"  kind: claude",
		"  profile: claude-default (.hergy/profiles.yaml)",
		"  env: GY_ACTOR",
		"  cwd: .",
		"  turns: builder, human",
		"  role document: .hergy/roles/lead.yaml",
		"  reads: .hergy/teams/demo/agreement.yaml",
		"member: demo-builder",
		"  role: builder",
		"  kind: claude",
		"  profile: claude-default (.hergy/profiles.yaml)",
		"  env: GY_ACTOR",
		"  cwd: .",
		"  turns: lead",
		"  role document: .hergy/roles/builder.yaml",
		"  reads: .hergy/teams/demo/agreement.yaml",
	)
}

func TestShowLabelsFollowTheLanguage(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	got := mustRun(t, "show", "demo", "--lang", "ja")
	for _, line := range []string{"チーム定義: demo", "メンバー: demo-lead", "  ロール: lead", "  ロールの文書: .hergy/roles/lead.yaml"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("Japanese output is missing %q:\n%s", line, got)
		}
	}
}

func TestUpDryShapePrintsThePromptUnescaped(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	got := mustRun(t, "up", "demo", "--dry")
	if strings.Contains(got, `\n`) {
		t.Fatalf("up --dry escapes the prompt:\n%s", got)
	}
	for _, line := range []string{
		"mode: boot",
		"team: demo",
		"timeout: 30000",
		"member: demo-lead",
		"  role: lead",
		"  role document: .hergy/roles/lead.yaml",
		"  reads: .hergy/teams/demo/agreement.yaml",
		"  prompt: |",
		"    You are demo-lead (lead) on team demo.",
		"    - **L1** Hand over a task the receiver can start without reading your conversation.",
		"    - **C1** Work along the gy-loop skill, so that work can resume from gy when a conversation ends.",
		"next: up demo",
	} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("up --dry is missing %q:\n%s", line, got)
		}
	}
	if !strings.HasSuffix(got, "next: up demo\n") {
		t.Fatalf("up --dry does not end with next:\n%s", got)
	}
}

// TestUpDryLeavesOutARuleThatIsNotInThePrompt: a rule a file keeps out of the
// prompt (in_prompt: false) is in neither the prompt up hands a member nor the
// one reprompt sends; the rules that are in the prompt stand unchanged
// (d-f187).
// TestOldRuleFieldsStopValidateAndUp: a file that still writes an old rule field
// is named by its file and rule id by validate and by up --dry, with where the
// content goes; neither silently drops it (r-81be, ac-9f8d).
func TestOldRuleFieldsStopValidateAndUp(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"),
		"version: 1\nsections:\n  - rules:\n      - id: L1\n        text: 本文。\n        why: 前の形の理由。\n")
	for _, argv := range [][]string{{"validate"}, {"up", "demo", "--dry"}} {
		stdout, stderr, code := run(argv...)
		if code == 0 {
			t.Fatalf("%v accepted an old rule field", argv)
		}
		out := stdout + "\n" + stderr
		for _, want := range []string{"lead.yaml", "L1", "old shape"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v does not name %q:\n%s", argv, want, out)
			}
		}
	}
}

func TestUpDryLeavesOutARuleThatIsNotInThePrompt(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:demo@demo"}`, "")
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"),
		"version: 1\nsections:\n  - heading: 進め方\n    rules:\n"+
			"      - id: L1\n        text: 渡す本文。\n        note: 人向けのメモ。\n"+
			"      - id: L2\n        text: 渡さない本文。\n        in_prompt: false\n")
	for _, argv := range [][]string{
		{"up", "demo", "--dry"},
		{"reprompt", "lead", "--team", "demo", "--dry"},
	} {
		got := mustRun(t, argv...)
		if strings.Contains(got, "渡さない本文。") || strings.Contains(got, "L2") {
			t.Errorf("%v hands out a rule the file keeps out of the prompt:\n%s", argv, got)
		}
		if strings.Contains(got, "人向けのメモ") {
			t.Errorf("%v sends a rule's note to a member:\n%s", argv, got)
		}
		if !strings.Contains(got, "- **L1** 渡す本文。") {
			t.Errorf("%v lost the rule that is in the prompt:\n%s", argv, got)
		}
	}
}

func TestUpDryJSONIsTheSameJSON(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	got := mustRun(t, "up", "demo", "--dry", "--json")
	var plan map[string]any
	if err := json.Unmarshal([]byte(got), &plan); err != nil {
		t.Fatalf("--json output is not JSON: %v\n%s", err, got)
	}
	for _, key := range []string{"mode", "team", "startup_timeout_ms", "members"} {
		if _, ok := plan[key]; !ok {
			t.Fatalf("--json plan is missing %q: %v", key, plan)
		}
	}
	if plan["mode"] != "boot" || plan["team"] != "demo" {
		t.Fatalf("--json plan has the wrong header: %v", plan)
	}
	if strings.Contains(got, "next:") {
		t.Fatalf("--json output must be the JSON alone:\n%s", got)
	}
}

// memberEnv reads one member's env out of an --dry --json plan, by member name.
func memberEnv(t *testing.T, plan map[string]any, name string) map[string]string {
	t.Helper()
	members, _ := plan["members"].([]any)
	for _, raw := range members {
		member, _ := raw.(map[string]any)
		if member["name"] != name {
			continue
		}
		out := map[string]string{}
		env, _ := member["env"].(map[string]any)
		for key, value := range env {
			out[key], _ = value.(string)
		}
		return out
	}
	t.Fatalf("no member %q in the plan: %v", name, plan)
	return nil
}

// TestUpDryNamesTheActorInEveryPane: hergy puts GY_ACTOR in each member's pane,
// set to the member's herdr name, and a profile that writes GY_ACTOR itself does
// not win (r-1c42).
func TestUpDryNamesTheActorInEveryPane(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	// A profile that writes GY_ACTOR: hergy's value wins, the rest of the
	// profile's env stays.
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    env:\n      GY_ACTOR: from-the-profile\n      FOO: bar\n")
	got := mustRun(t, "up", "demo", "--dry", "--json")
	var plan map[string]any
	if err := json.Unmarshal([]byte(got), &plan); err != nil {
		t.Fatalf("--json output is not JSON: %v\n%s", err, got)
	}
	for _, name := range []string{"demo-lead", "demo-builder"} {
		env := memberEnv(t, plan, name)
		if env["GY_ACTOR"] != name {
			t.Errorf("%s's env has GY_ACTOR=%q, want %q: %v", name, env["GY_ACTOR"], name, env)
		}
		if env["FOO"] != "bar" {
			t.Errorf("%s's env lost the profile's own variable: %v", name, env)
		}
	}
	if text := mustRun(t, "up", "demo", "--dry"); !strings.Contains(text, "GY_ACTOR") {
		t.Errorf("up --dry does not show GY_ACTOR:\n%s", text)
	}
}

// TestUpDryActorFollowsTheTeamName: the name hergy passes is the herdr name of
// the run, so --as moves it with the member's name (r-1c42).
func TestUpDryActorFollowsTheTeamName(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	got := mustRun(t, "up", "demo", "--as", "dev2", "--dry", "--json")
	var plan map[string]any
	if err := json.Unmarshal([]byte(got), &plan); err != nil {
		t.Fatalf("--json output is not JSON: %v\n%s", err, got)
	}
	if env := memberEnv(t, plan, "dev2-lead"); env["GY_ACTOR"] != "dev2-lead" {
		t.Errorf("the actor with --as is %q, want dev2-lead", env["GY_ACTOR"])
	}
}

// TestHireDryNamesTheActorInTheWorkersPane: a worker's pane gets GY_ACTOR set to
// the worker's herdr name (r-1c42).
func TestHireDryNamesTheActorInTheWorkersPane(t *testing.T) {
	workersFixture(t)
	got := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry", "--json")
	var plan map[string]any
	if err := json.Unmarshal([]byte(got), &plan); err != nil {
		t.Fatalf("--json output is not JSON: %v\n%s", err, got)
	}
	if env := memberEnv(t, plan, "dev-researcher"); env["GY_ACTOR"] != "dev-researcher" {
		t.Errorf("the worker's env has GY_ACTOR=%q, want dev-researcher", env["GY_ACTOR"])
	}
}

// TestBootstrapPromptNamesTheActor: the prompt hergy hands a member and a worker
// says the name gy is written under, in the language in use (r-1c42).
func TestBootstrapPromptNamesTheActor(t *testing.T) {
	workersFixture(t)
	if got := mustRun(t, "up", "dev", "--dry"); !strings.Contains(got, i18n.Tf("prompt.actor", "dev-lead")) {
		t.Errorf("the member's prompt does not name the actor:\n%s", got)
	}
	if got := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry"); !strings.Contains(got, i18n.Tf("prompt.actor", "dev-researcher")) {
		t.Errorf("the worker's prompt does not name the actor:\n%s", got)
	}
	// The sentence is translated too; ask for Japanese and read it back.
	if got := mustRun(t, "up", "dev", "--dry", "--lang", "ja"); !strings.Contains(got, i18n.Tf("prompt.actor", "dev-lead")) {
		t.Errorf("the Japanese prompt does not name the actor:\n%s", got)
	}
}

// TestValidateWarnsAboutAProfileThatWritesGYActor: a profile's own GY_ACTOR
// never reaches the agent, because hergy sets that name; validate warns and the
// exit code stays 0 (r-1c42).
func TestValidateWarnsAboutAProfileThatWritesGYActor(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    env:\n      GY_ACTOR: builder\n")
	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("validate failed on a profile that writes GY_ACTOR:\n%s", got)
	}
	if !strings.Contains(got, i18n.Tf("validate.profile_env_actor", "claude-default")) {
		t.Fatalf("validate does not warn about GY_ACTOR:\n%s", got)
	}
}

// TestRefreshKeepsThePanesEnv: env reaches a pane only when the pane is made,
// and refresh starts the member again in the pane it already has, so GY_ACTOR
// stays. The check is that refresh makes no pane: the recorded herdr calls hold
// no tab create and no pane split, the two places hergy passes env (r-1c42).
func TestRefreshKeepsThePanesEnv(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.refreshDeadHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "w0:p1")
	mustRun(t, "refresh", "lead")
	record := readFile(t, log)
	for _, made := range []string{"tab create", "pane split"} {
		if strings.Contains(record, made) {
			t.Errorf("refresh made a pane (%q), which would drop GY_ACTOR:\n%s", made, record)
		}
	}
}

// TestRefreshStartsAStoppedMember: refresh starts a member that is not live when
// the pane hergy named after it is still at its shell, using that pane.
func TestRefreshStartsAStoppedMember(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.refreshDeadHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "w0:p1")
	got := mustRun(t, "refresh", "lead")
	if !strings.Contains(got, "restarted: demo-lead\n") {
		t.Fatalf("refresh does not report the restart:\n%s", got)
	}
	record := readFile(t, log)
	if !strings.Contains(record, "agent start demo-lead --kind claude --pane w1:p9") {
		t.Fatalf("refresh did not start the member in its named pane:\n%s", record)
	}
}

// TestRefreshStoppedMemberWithoutAPanePointsAtDownAndUp: when the pane is gone,
// refresh changes nothing and tells the reader to redo down and up.
func TestRefreshStoppedMemberWithoutAPanePointsAtDownAndUp(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, noPaneHerdr)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "w0:p1")
	_, stderr, code := run("refresh", "lead")
	if code == 0 {
		t.Fatal("refresh succeeded with no pane for the member")
	}
	for _, want := range []string{"hergy down", "hergy up"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("refresh does not point at %q:\n%s", want, stderr)
		}
	}
}

func TestRefreshDryEndsWithNext(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	// refresh --dry reads the live agents to show the same refusal the real
	// refresh would, so it needs a herdr that answers agent get.
	f.herdr(t, liveHerdr)
	// refresh points at the short member name written in team.yaml, not the
	// herdr name hergy made from it.
	if got := mustRun(t, "refresh", "lead", "--dry"); !strings.HasSuffix(got, "next: refresh lead --team demo\n") {
		t.Fatalf("refresh <member> --dry does not end with next:\n%s", got)
	}
	if got := mustRun(t, "refresh", "--all", "--dry"); !strings.HasSuffix(got, "next: refresh --all --team demo\n") {
		t.Fatalf("refresh --all --dry does not end with next:\n%s", got)
	}
	if got := mustRun(t, "reprompt", "lead", "--team", "demo", "--dry"); !strings.HasSuffix(got, "next: reprompt lead --team demo\n") {
		t.Fatalf("reprompt <member> --dry does not end with next:\n%s", got)
	}
	if got := mustRun(t, "reprompt", "--all", "--team", "demo", "--dry"); !strings.HasSuffix(got, "next: reprompt --all --team demo\n") {
		t.Fatalf("reprompt --all --dry does not end with next:\n%s", got)
	}
}

func TestDownDryShape(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, liveHerdr)
	got := mustRun(t, "down", "demo", "--dry")
	wantLines(t, got,
		"mission: what this team is for, in one line",
		"team: demo",
		"version: 1",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: .hergy/profiles.yaml",
		"demo-lead: would close (pane p1)",
		"demo-builder: would close (pane p2)",
		"would close: 2",
		"would skip: 0",
		"next: down demo",
	)
}

func TestDownDryWithoutLiveMembersHasNoNext(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, passingHerdr)
	got := mustRun(t, "down", "demo", "--dry")
	wantLines(t, got,
		"mission: what this team is for, in one line",
		"team: demo",
		"version: 1",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: .hergy/profiles.yaml",
		"demo-lead: would skip (not present)",
		"demo-builder: would skip (not present)",
		"note: nothing to close (team \"demo\")",
	)
	if strings.Contains(got, "next:") {
		t.Fatalf("down --dry with nothing to close must not name a next step:\n%s", got)
	}
}

func TestUpProductionShowsNoPerMemberProvenance(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, passingHerdr)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	got := mustRun(t, "up", "demo")
	wantLines(t, got,
		"mission: what this team is for, in one line",
		"team: demo",
		"version: 1",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: .hergy/profiles.yaml",
		"submitted: demo-lead (pane p1)",
		"submitted: demo-builder (pane p2)",
		"booted: demo (tab t1)",
		"prompts: submitted (document reading continues in each pane)",
	)
	for _, unwanted := range []string{"role file", "reads", "member:"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("up (production) shows the per-member list (%q):\n%s", unwanted, got)
		}
	}
}

func TestDownProductionShowsNoPerMemberProvenance(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, liveHerdr)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	got := mustRun(t, "down", "demo")
	wantLines(t, got,
		"mission: what this team is for, in one line",
		"team: demo",
		"version: 1",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: .hergy/profiles.yaml",
		"demo-lead: closing (pane p1)",
		"demo-builder: closing (pane p2)",
		"closed: 2",
		"absent: 0",
	)
	for _, unwanted := range []string{"role file", "reads", "member:"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("down (production) shows the per-member list (%q):\n%s", unwanted, got)
		}
	}
}

func TestNotFoundTeamEndsWithNext(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	stdout, stderr, code := run("up", "nosuch")
	if code == 0 {
		t.Fatal("up nosuch succeeded")
	}
	if stdout != "" {
		t.Fatalf("up nosuch wrote to stdout: %q", stdout)
	}
	wantLines(t, stderr,
		"error: there is no team definition nosuch (looked in: .hergy/teams/nosuch/team.yaml, "+
			filepath.Join(f.cfg, "hergy", "teams", "nosuch", "team.yaml")+"; candidates: demo); pick one of the candidates",
		"next: up demo",
	)
}

func TestNotFoundTeamWithoutCandidatesHasNoNext(t *testing.T) {
	newFixture(t)
	_, stderr, code := run("up", "nosuch")
	if code == 0 {
		t.Fatal("up nosuch succeeded")
	}
	if strings.Contains(stderr, "next:") {
		t.Fatalf("a not-found error without candidates has a next:\n%s", stderr)
	}
}

// TestUpWithoutANameCountsTheProjectsDefinitions: a name that is omitted reads
// the project's definitions only, so the number the stop reports is the project's
// (0 here), while a definition outside the project is still a candidate (D1,
// spec/35).
func TestUpWithoutANameCountsTheProjectsDefinitions(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.cfg, "hergy", "teams", "g", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n  - name: a\n    role: lead\n    profile: claude-default\n")
	_, stderr, code := run("up")
	if code == 0 {
		t.Fatal("up without a name succeeded with no project definition")
	}
	want := i18n.Tf("error.team_count_candidates", 0, "g")
	if !strings.Contains(stderr, want) {
		t.Fatalf("up without a name does not count the project's definitions:\n%s", stderr)
	}
	if !strings.Contains(stderr, "next: up g\n") {
		t.Fatalf("up without a name does not offer the one candidate:\n%s", stderr)
	}
}

// TestUpWithoutAnyDefinitionSaysHowToMakeOne: with no definition anywhere, the
// stop says there are none and names the next command (D1, spec/35).
func TestUpWithoutAnyDefinitionSaysHowToMakeOne(t *testing.T) {
	newFixture(t)
	_, stderr, code := run("up")
	if code == 0 {
		t.Fatal("up without a name succeeded with no definitions")
	}
	want := i18n.T("error.team_count_none") + "\n" + i18n.Tf("next", "new <name>")
	if !strings.Contains(stderr, want) {
		t.Fatalf("up without any definition does not say how to make one:\n%s", stderr)
	}
}

func TestJSONNeedsDry(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	_, stderr, code := run("up", "demo", "--json")
	if code == 0 {
		t.Fatal("up --json without --dry succeeded")
	}
	if !strings.Contains(stderr, "error: --json needs --dry") {
		t.Fatalf("up --json without --dry does not say why:\n%s", stderr)
	}
}

// TestHomePathsUseTilde checks the ~/ form: with XDG_CONFIG_HOME unset, the
// global directory is under the home directory and must show as ~/...
func TestHomePathsUseTilde(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(root, "proj")
	home := filepath.Join(root, "home")
	for _, dir := range []string{proj, home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(proj)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	got := mustRun(t, "new", "gteam")
	if !strings.Contains(got, "dir: ~/.config/hergy/teams/gteam\n") {
		t.Fatalf("a path under the home directory is not shown with ~:\n%s", got)
	}
}

// TestNewWritesIntoTheProjectInsideARepository: hergy new writes where the person
// is (K2): inside a git repository that is the project's own layer, so no layer
// flag is needed for the common case.
func TestNewWritesIntoTheProjectInsideARepository(t *testing.T) {
	f := newFixture(t)
	got := mustRun(t, "new", "demo")
	if !strings.Contains(got, "dir: .hergy/teams/demo\n") {
		t.Fatalf("new did not write into the project inside a git repository:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")); err != nil {
		t.Fatalf("the team definition is not in the project: %v", err)
	}
}

// TestLayerFlagsAreGone: --local and --global are gone from the team commands
// (段 3): typing one stops with the sentence that says a team definition name
// resolves to the project's own first, rather than with "unknown option".
func TestLayerFlagsAreGone(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	for _, flag := range []string{"--local", "--global"} {
		stdout, stderr, code := run("up", "demo", flag, "--dry")
		if code == 0 {
			t.Fatalf("hergy up demo %s succeeded", flag)
		}
		if stdout != "" {
			t.Errorf("hergy up demo %s printed to stdout:\n%s", flag, stdout)
		}
		if !strings.Contains(stderr, flag+" is gone") {
			t.Errorf("hergy up demo %s: the error is %q, want the sentence that says the flag is gone", flag, stderr)
		}
	}
	// new is told where it writes, on a second line (段 7, H6).
	for _, flag := range []string{"--local", "--global"} {
		_, stderr, code := run("new", "fresh", flag)
		if code == 0 {
			t.Fatalf("hergy new fresh %s succeeded", flag)
		}
		if !strings.Contains(stderr, flag+" is gone") || !strings.Contains(stderr, i18n.T("error.new_writes")) {
			t.Errorf("hergy new fresh %s: the error is %q, want the flag sentence and a line saying where new writes", flag, stderr)
		}
	}
}

// TestValidateWarnsAboutAHiddenGlobalDefinition: a global definition that a
// project definition of the same name hides is warned about, where a person
// reading the global file finds it (段 3); the project's own definition is the
// one hergy uses (I11).
func TestValidateWarnsAboutAHiddenGlobalDefinition(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	// A global definition of the same name, with no role file of its own: the
	// project's profiles.yaml answers its profile (K1), so it reads and is only
	// hidden, not broken.
	writeFile(t, filepath.Join(f.cfg, "hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nprompt: \"%{role_doc}%{docs}\"\nmembers:\n  - name: a\n    role: lead\n    profile: claude-default\nturns:\n  - human -> a\n  - a -> human\n")

	out := mustRun(t, "validate")
	want := "warning: a team definition with this name is also in the project, so this one is not used (demo)"
	if !strings.Contains(out, want) {
		t.Fatalf("validate does not warn about the hidden global definition:\n%s", out)
	}
}

// TestNewWritesIntoTheGlobalLayerOutsideARepository: outside a git repository
// there is no project, so hergy new writes into the global layer.
func TestNewWritesIntoTheGlobalLayerOutsideARepository(t *testing.T) {
	f := newFixture(t)
	outside := filepath.Join(filepath.Dir(f.proj), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(outside)
	got := mustRun(t, "new", "demo")
	if !strings.Contains(got, "dir: ") || !strings.Contains(got, "demo") {
		t.Fatalf("new printed no directory:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(f.cfg, "hergy", "teams", "demo", "team.yaml")); err != nil {
		t.Fatalf("the team definition is not in the global layer: %v", err)
	}
}

// TestNewWritesShortMemberNames: hergy new writes the member names as the short
// member names, without quotes; hergy adds the team name when it boots them.
func TestNewWritesShortMemberNames(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	text := readFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"))
	for _, want := range []string{"  - name: lead\n", "  - name: builder\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("team.yaml does not carry the short name %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"%{team}", `"lead"`, "demo-lead"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("team.yaml still carries %q:\n%s", unwanted, text)
		}
	}
}

// TestUpAsDryUsesTheTeamName: --as names the team, and the initial prompt shows
// that name; without --as the team definition name is the team name.
func TestUpAsDryUsesTheTeamName(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")

	plain := mustRun(t, "up", "demo", "--dry")
	for _, line := range []string{
		"team: demo",
		"member: demo-lead",
		"    You are demo-lead (lead) on team demo.",
		"    - lead: demo-lead (you)",
	} {
		if !strings.Contains(plain, line+"\n") {
			t.Fatalf("up demo --dry is missing %q:\n%s", line, plain)
		}
	}
	if !strings.HasSuffix(plain, "next: up demo\n") {
		t.Fatalf("up demo --dry does not end with next:\n%s", plain)
	}

	named := mustRun(t, "up", "demo", "--as", "alpha", "--dry")
	for _, line := range []string{
		"team: alpha (from demo)",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"member: alpha-lead",
		"member: alpha-builder",
		"    You are alpha-lead (lead) on team alpha.",
		"    - lead: alpha-lead (you)",
		"    - builder: alpha-builder",
	} {
		if !strings.Contains(named, line+"\n") {
			t.Fatalf("up demo --as alpha --dry is missing %q:\n%s", line, named)
		}
	}
	if !strings.HasSuffix(named, "next: up demo --as alpha\n") {
		t.Fatalf("up demo --as alpha --dry does not end with next:\n%s", named)
	}
}

// TestUpAsDev2PrefixesTheMemberNames: --as changes only the prefix; the member names stay
// lead and worker, and the herdr names, roster and self-intro become dev2-*.
func TestUpAsDev2PrefixesTheMemberNames(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "dev")
	got := mustRun(t, "up", "dev", "--as", "dev2", "--dry")
	for _, line := range []string{
		"team: dev2 (from dev)",
		"member: dev2-lead",
		"member: dev2-builder",
		"    You are dev2-lead (lead) on team dev2.",
		"    - lead: dev2-lead (you)",
		"    - builder: dev2-builder",
	} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("up dev --as dev2 --dry is missing %q:\n%s", line, got)
		}
	}
	if !strings.HasSuffix(got, "next: up dev --as dev2\n") {
		t.Fatalf("up dev --as dev2 --dry does not end with next:\n%s", got)
	}
}

// TestTwoTeamsWithTheSameMemberNamesDoNotCollide: two settings that both name a member
// "lead" boot side by side. hergy prefixes each with its own team name, so the
// herdr names stay apart and the second boot does not collide.
func TestTwoTeamsWithTheSameMemberNamesDoNotCollide(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	mustRun(t, "new", "discovery")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	dev := mustRun(t, "up", "dev")
	for _, line := range []string{"submitted: dev-lead (pane p1)", "submitted: dev-builder (pane p2)"} {
		if !strings.Contains(dev, line+"\n") {
			t.Fatalf("up dev is missing %q:\n%s", line, dev)
		}
	}
	discovery := mustRun(t, "up", "discovery")
	for _, line := range []string{"submitted: discovery-lead (pane p1)", "submitted: discovery-builder (pane p2)"} {
		if !strings.Contains(discovery, line+"\n") {
			t.Fatalf("up discovery is missing %q:\n%s", line, discovery)
		}
	}
}

// TestUpDryPromptSendsHiringThroughHergy: the member's bootstrap prompt tells it
// to add a teammate with hergy hire, not herdr agent start.
func TestUpDryPromptSendsHiringThroughHergy(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo")
	got := mustRun(t, "up", "demo", "--dry")
	for _, want := range []string{"hergy hire", "herdr agent start"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the member's prompt does not mention %q:\n%s", want, got)
		}
	}
}

// TestHireDryWorkerPromptAddsNoTeammates: a worker's prompt says a worker adds
// no teammates, rather than telling it to hire.
func TestHireDryWorkerPromptAddsNoTeammates(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: dev\n"+
			"workers:\n  - from: lead\n    purpose: research\n    profile: dev\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:dev@dev"}`, "")
	got := mustRun(t, "hire", "researcher", "--purpose", "research",
		"--task", "read the docs", "--team", "dev", "--dry")
	if !strings.Contains(got, "A worker adds no teammates") {
		t.Fatalf("the worker's prompt does not say it adds no teammates:\n%s", got)
	}
	if strings.Contains(got, "hergy hire") {
		t.Fatalf("the worker's prompt tells it to hire:\n%s", got)
	}
}

// TestHireNamesTheWorkerAfterTheTeam: hergy hire takes the short worker name and
// gives the worker the herdr name "<team>-<name>", both in the plan and in the
// prompt the worker receives.
func TestHireNamesTheWorkerAfterTheTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: dev\n"+
			"workers:\n  - from: lead\n    purpose: research\n    profile: dev\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")

	f.runningHerdr(t, `{"tab_id":"t1","label":"team:dev@dev"}`, "")
	got := mustRun(t, "hire", "researcher", "--purpose", "research",
		"--task", "read the docs", "--team", "dev", "--dry")
	for _, line := range []string{
		"mode: spawn",
		"member: dev-researcher",
		"    You are dev-researcher on team dev, a research worker hired by dev-lead.",
	} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("hire --dry is missing %q:\n%s", line, got)
		}
	}
	if !strings.Contains(got, "From dev-researcher:") {
		t.Fatalf("the worker's contact intro is not its herdr name:\n%s", got)
	}
}

// TestHireLabelsTheTabWithTheTeam: the worker's tab is labelled
// "worker:<team>/<short name>", so down finds it; two teams from one team
// definition label their workers apart by the team name.
func TestHireLabelsTheTabWithTheTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: dev\n"+
			"workers:\n  - from: lead\n    purpose: research\n    profile: dev\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	// Two teams from the dev team definition, so the team name decides each
	// worker's tab.
	mustRun(t, "up", "dev")
	mustRun(t, "up", "dev", "--as", "dev2")

	mustRun(t, "hire", "researcher", "--purpose", "research",
		"--task", "read the docs", "--team", "dev")
	record := readFile(t, log)
	if !strings.Contains(record, "--label worker:dev/researcher ") {
		t.Fatalf("hire did not label the tab with the team and short name:\n%s", record)
	}

	mustRun(t, "hire", "reviewer", "--purpose", "research",
		"--task", "review the docs", "--team", "dev2")
	record = readFile(t, log)
	if !strings.Contains(record, "--label worker:dev2/reviewer ") {
		t.Fatalf("hire did not label the tab with the other team name:\n%s", record)
	}
}

// TestUpAsBootsTwoTeamsSideBySide: two team names from one settings both boot.
// The fake herdr remembers the agents it started, so ignoring --as would make the
// second boot collide in preflight.
func TestUpAsBootsTwoTeamsSideBySide(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	alpha := mustRun(t, "up", "demo", "--as", "alpha")
	for _, line := range []string{
		"team: alpha (from demo)",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"submitted: alpha-lead (pane p1)",
		"submitted: alpha-builder (pane p2)",
		"booted: alpha (tab t1)",
	} {
		if !strings.Contains(alpha, line+"\n") {
			t.Fatalf("up demo --as alpha is missing %q:\n%s", line, alpha)
		}
	}

	beta := mustRun(t, "up", "demo", "--as", "beta")
	for _, line := range []string{
		"team: beta (from demo)",
		"submitted: beta-lead (pane p1)",
		"submitted: beta-builder (pane p2)",
		"booted: beta (tab t2)",
	} {
		if !strings.Contains(beta, line+"\n") {
			t.Fatalf("up demo --as beta is missing %q:\n%s", line, beta)
		}
	}

	record := readFile(t, log)
	for _, want := range []string{"--label alpha", "--label beta"} {
		if !strings.Contains(record, want) {
			t.Fatalf("herdr was not asked for %q:\n%s", want, record)
		}
	}
}

// TestDownByTeamNameClosesOnlyThatTeamsMembers: down with a team name closes
// the named team's members and leaves the other team's panes alone, even though
// both teams come from one team definition.
func TestDownByTeamNameClosesOnlyThatTeamsMembers(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	mustRun(t, "up", "demo", "--as", "alpha")
	mustRun(t, "up", "demo", "--as", "beta")

	got := mustRun(t, "down", "alpha")
	for _, line := range []string{
		"team: alpha (from demo)",
		"alpha-lead: closing (pane p-alpha-lead)",
		"alpha-builder: closing (pane p-alpha-builder)",
		"closed: 2",
		"absent: 0",
	} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("down demo --as alpha is missing %q:\n%s", line, got)
		}
	}
	if strings.Contains(got, "beta-") {
		t.Fatalf("down demo --as alpha reports the other team:\n%s", got)
	}

	closes := readFile(t, log)
	if !strings.Contains(closes, "pane close p-alpha-lead") || !strings.Contains(closes, "pane close p-alpha-builder") {
		t.Fatalf("down demo --as alpha did not close alpha's panes:\n%s", closes)
	}
	if strings.Contains(closes, "pane close p-beta-lead") || strings.Contains(closes, "pane close p-beta-builder") {
		t.Fatalf("down demo --as alpha closed a beta pane:\n%s", closes)
	}
}

// TestUpStopsOnTeamPlaceholderInMemberName: the team name is added to every
// member name automatically, so a literal %{team} in a member name is refused
// with a message that says not to write it, --as or not.
func TestUpStopsOnTeamPlaceholderInMemberName(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: \"%{team}-lead\"\n    role: lead\n    profile: dev\n")

	for _, argv := range [][]string{
		{"up", "demo", "--dry"},
		{"up", "demo", "--as", "alpha", "--dry"},
	} {
		stdout, stderr, code := run(argv...)
		if code == 0 {
			t.Fatalf("%v succeeded with %%{team} in a member name", argv)
		}
		if stdout != "" {
			t.Fatalf("%v wrote to stdout: %q", argv, stdout)
		}
		if !strings.Contains(stderr, "%{team}") || !strings.Contains(stderr, "automatically") {
			t.Fatalf("%v does not explain the %%{team} rule:\n%s", argv, stderr)
		}
	}
}

// TestHireCommandCarriesTheTeam: the hire command written into a prompt names
// the team by team name (never --as; hire takes a team name), and --from carries
// the short member name, not the herdr name.
func TestHireCommandCarriesTheTeam(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nprompt: \"%{role_doc}%{docs}%{name}\\n%{workers}\"\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: dev\n"+
			"workers:\n  - from: lead\n    purpose: research\n    profile: dev\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"),
		"version: 1\nsections:\n  - rules:\n      - id: L1\n        text: hand the turn to whoever acts next\n")

	plain := mustRun(t, "up", "demo", "--dry")
	if !strings.Contains(plain, "--team demo --lang en") {
		t.Fatalf("the hire command without --as is wrong:\n%s", plain)
	}
	if !strings.Contains(plain, "--from lead") {
		t.Fatalf("the hire command does not take the short member name:\n%s", plain)
	}
	if strings.Contains(plain, "--as") {
		t.Fatalf("the hire command carries --as:\n%s", plain)
	}
	if strings.Contains(plain, "--profile") {
		t.Fatalf("the hire command still carries --profile:\n%s", plain)
	}

	named := mustRun(t, "up", "demo", "--as", "alpha", "--dry")
	if !strings.Contains(named, "--team alpha --lang en") {
		t.Fatalf("the hire command with --as is wrong:\n%s", named)
	}
	if strings.Contains(named, "--team demo") {
		t.Fatalf("the hire command names the team definition, not the team:\n%s", named)
	}
	if !strings.Contains(named, "--from lead") {
		t.Fatalf("the hire command with --as does not take the short member name:\n%s", named)
	}
}

// workersFixture writes a dev team whose lead may hire for 調査 (seeker) and
// 実装 (developer), and points the fake herdr at the running team.
func workersFixture(t *testing.T) {
	t.Helper()
	f := newFixture(t)
	workersProject(t, f)
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:dev@dev"}`, "")
}

// workersProject lays the team definition and the profiles the hire tests use,
// in a project of its own, and says nothing about a team running.
func workersProject(t *testing.T, f *fixture) {
	t.Helper()
	mustRun(t, "new", "dev")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: seeker\n"+
			"workers:\n"+
			"  - from: lead\n    purpose: 調査\n    profile: seeker\n"+
			"  - from: lead\n    purpose: 実装\n    profile: developer\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  seeker:\n    kind: claude\n  developer:\n    kind: codex\n")
}

// TestHireDryPlansWithoutHerdr: hire --dry is planned from the team definition
// and reads no herdr, so a team that is not running still gets the worker's
// prompt (r-fe31, ac-73a9). The team is the one --team names; a name the launch
// record does not know is a team definition name.
func TestHireDryPlansWithoutHerdr(t *testing.T) {
	f := newFixture(t)
	workersProject(t, f)

	got := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry", "--json")
	var plan struct {
		Team    string `json:"team"`
		Members []struct {
			Name   string `json:"name"`
			Prompt string `json:"prompt"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(got), &plan); err != nil {
		t.Fatalf("--json output is not JSON: %v\n%s", err, got)
	}
	if plan.Team != "dev" {
		t.Errorf("the plan is for team %q, want dev", plan.Team)
	}
	if len(plan.Members) != 1 || plan.Members[0].Name != "dev-researcher" {
		t.Fatalf("the plan's members are %v, want the one dev-researcher", plan.Members)
	}
	if !strings.Contains(plan.Members[0].Prompt, "## Task\nread") {
		t.Errorf("the worker's prompt is not the plan's own:\n%s", plan.Members[0].Prompt)
	}

	// The readable form is the same plan, and it also needs no herdr.
	if got := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry"); !strings.Contains(got, "member: dev-researcher") {
		t.Errorf("hire --dry without --json does not show the worker:\n%s", got)
	}
}

// TestHireDryNeedsTeamWithoutALaunchRecord: --dry does not read herdr, so with no
// --team and no launch record to say which team it is, the command stops and asks
// for the name rather than guessing.
func TestHireDryNeedsTeamWithoutALaunchRecord(t *testing.T) {
	f := newFixture(t)
	workersProject(t, f)

	_, stderr, code := run("hire", "researcher", "--purpose", "調査", "--task", "read", "--dry")
	if code == 0 {
		t.Fatal("hire --dry without --team succeeded with no launch record")
	}
	if !strings.Contains(stderr, "--team") {
		t.Errorf("hire --dry without --team does not ask for the name:\n%s", stderr)
	}
}

// TestHireDryTakesTheTeamFromTheLaunchRecord: without --team, the one team this
// project's launch record holds is the team the plan is made for, read from the
// record rather than from herdr (which is gone by then).
func TestHireDryTakesTheTeamFromTheLaunchRecord(t *testing.T) {
	f := newFixture(t)
	workersProject(t, f)
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "dev")

	// herdr is not there any more: only the launch record can say which team.
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "no-herdr"))
	got := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--dry", "--json")
	var plan struct {
		Team string `json:"team"`
	}
	if err := json.Unmarshal([]byte(got), &plan); err != nil {
		t.Fatalf("--json output is not JSON: %v\n%s", err, got)
	}
	if plan.Team != "dev" {
		t.Errorf("the plan is for team %q, want the team the launch record holds, dev", plan.Team)
	}
}

// TestHireWithoutDryStopsForAStoppedTeam: a real hire still stops on a team that
// is not running (it is herdr that says so), and only --dry plans without herdr.
func TestHireWithoutDryStopsForAStoppedTeam(t *testing.T) {
	f := newFixture(t)
	workersProject(t, f)
	f.runningHerdr(t, "", "")
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	_, stderr, code := run("hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev")
	if code == 0 {
		t.Fatal("a real hire succeeded with the team not running")
	}
	if !strings.Contains(stderr, i18n.Tf("error.team_not_running", "dev")) {
		t.Errorf("a real hire does not say the team is not running:\n%s", stderr)
	}
}

// TestHireUsesTheProfilePairedWithThePurpose: hire takes the profile the
// member's workers: pairs with --purpose, so one purpose runs one engine and the
// next runs another.
func TestHireUsesTheProfilePairedWithThePurpose(t *testing.T) {
	workersFixture(t)

	researcher := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry")
	for _, want := range []string{"member: dev-researcher", "kind: claude", "profile: seeker"} {
		if !strings.Contains(researcher, want) {
			t.Fatalf("hire --purpose 調査 is missing %q:\n%s", want, researcher)
		}
	}

	builder := mustRun(t, "hire", "builder", "--purpose", "実装", "--task", "build", "--team", "dev", "--dry")
	for _, want := range []string{"member: dev-builder", "kind: codex", "profile: developer"} {
		if !strings.Contains(builder, want) {
			t.Fatalf("hire --purpose 実装 is missing %q:\n%s", want, builder)
		}
	}
}

// TestHirePurposeOutsideWorkers: a purpose the hiring member's workers: does not
// name stops the hire, and the sentence lists the purposes that member may hire
// for (I6). The profile comes from the workers: entry, so there is no --profile;
// a purpose the entry does name hires.
func TestHirePurposeOutsideWorkers(t *testing.T) {
	t.Run("a purpose no member hires for", func(t *testing.T) {
		workersFixture(t)
		_, stderr, code := run("hire", "w", "--purpose", "未知", "--task", "t", "--team", "dev", "--dry")
		if code == 0 {
			t.Fatal("hire with a purpose outside workers: succeeded")
		}
		for _, want := range []string{"error:", "未知", "調査", "実装"} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("the error does not name %q:\n%s", want, stderr)
			}
		}
		if strings.Contains(stderr, "[[") {
			t.Fatalf("the error prints an untranslated key:\n%s", stderr)
		}
	})

	t.Run("a purpose another member hires for", func(t *testing.T) {
		f := newFixture(t)
		mustRun(t, "new", "dev")
		writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
			"version: 1\nmission: a mission for the test\nmembers:\n"+
				"  - name: lead\n    role: lead\n    profile: seeker\n"+
				"  - name: spec\n    role: spec\n    profile: seeker\n"+
				"workers:\n"+
				"  - from: lead\n    purpose: 実装\n    profile: developer\n"+
				"  - from: spec\n    purpose: 調査\n    profile: seeker\n"+
				"turns:\n  - human -> lead\n  - lead -> human\n  - lead -> spec\n  - spec -> lead\n")
		writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
			"version: 1\nprofiles:\n  seeker:\n    kind: claude\n  developer:\n    kind: codex\n")
		// The second member's role needs its document (roles live once per
		// layer).
		writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "spec.yaml"),
			"version: 1\nsections:\n  - rules:\n      - id: S1\n        text: hand the turn to lead\n")
		f.runningHerdr(t, `{"tab_id":"t1","label":"team:dev@dev"}`, "")

		// --from names the member, so the sentence is about what that member may
		// hire for, not about the member whose purpose was written.
		_, stderr, code := run("hire", "w", "--from", "lead", "--purpose", "調査", "--task", "t", "--team", "dev", "--dry")
		if code == 0 {
			t.Fatal("hire for another member's purpose succeeded")
		}
		for _, want := range []string{"lead", "調査", "実装"} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("the error does not name %q:\n%s", want, stderr)
			}
		}
	})

	t.Run("--profile is gone", func(t *testing.T) {
		workersFixture(t)
		_, stderr, code := run("hire", "w", "--profile", "seeker", "--purpose", "調査", "--task", "t", "--team", "dev", "--dry")
		if code == 0 {
			t.Fatal("hire --profile succeeded")
		}
		for _, want := range []string{"--profile", "workers:"} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("the error does not name %q:\n%s", want, stderr)
			}
		}
		if strings.Contains(stderr, "unknown option") {
			t.Fatalf("--profile is refused as an unknown option, not as a flag that is gone:\n%s", stderr)
		}
	})

	t.Run("a purpose the entry names hires", func(t *testing.T) {
		workersFixture(t)
		stdout, stderr, code := run("hire", "w", "--purpose", "実装", "--task", "t", "--team", "dev", "--dry")
		if code != 0 {
			t.Fatalf("hire for a purpose workers: names: exit %d\nstderr:\n%s", code, stderr)
		}
		if !strings.Contains(stdout, "mode: spawn") || !strings.Contains(stdout, "profile: developer") {
			t.Fatalf("hire did not plan the worker with the paired profile:\n%s", stdout)
		}
		if stderr != "" {
			t.Fatalf("hire for a purpose workers: names noted something:\n%s", stderr)
		}
	})
}

// TestHirePurposeNobodyHiresFor: no member's workers: names the purpose and
// --from is not written. --from would not help (every member would refuse that
// purpose), so the hire is refused with who may hire for what, rather than
// asking for --from. The purposes are listed member by member, with the member's
// short name.
func TestHirePurposeNobodyHiresFor(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: seeker\n"+
			"  - name: spec\n    role: spec\n    profile: seeker\n"+
			"workers:\n"+
			"  - from: lead\n    purpose: 実装\n    profile: developer\n"+
			"  - from: spec\n    purpose: 調査\n    profile: seeker\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n  - lead -> spec\n  - spec -> lead\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  seeker:\n    kind: claude\n  developer:\n    kind: codex\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "spec.yaml"),
		"version: 1\nsections:\n  - rules:\n      - id: S1\n        text: hand the turn to lead\n")
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:dev@dev"}`, "")

	_, stderr, code := run("hire", "w", "--purpose", "デザイン", "--task", "t", "--team", "dev", "--dry")
	if code == 0 {
		t.Fatal("hire for a purpose nobody hires for succeeded")
	}
	for _, want := range []string{"デザイン", "--from", "lead: 実装 / spec: 調査"} {
		if want == "--from" {
			// The refusal must not ask for --from.
			if strings.Contains(stderr, want) {
				t.Fatalf("the refusal asks for --from, which would not help:\n%s", stderr)
			}
			continue
		}
		if !strings.Contains(stderr, want) {
			t.Fatalf("the error does not name %q:\n%s", want, stderr)
		}
	}

	_, stderr, code = run("hire", "w", "--purpose", "デザイン", "--task", "t", "--team", "dev", "--dry", "--lang", "ja")
	if code == 0 {
		t.Fatal("hire for a purpose nobody hires for succeeded (ja)")
	}
	if !strings.Contains(stderr, "lead は 実装／spec は 調査") {
		t.Fatalf("the Japanese refusal does not list who may hire for what:\n%s", stderr)
	}
}

// TestHireFromIsOptionalWithOneCandidate: --from may be left out when exactly
// one member hires for the purpose (d-6f03). Two members hiring for it stop the
// hire and list them.
func TestHireFromIsOptionalWithOneCandidate(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: seeker\n"+
			"  - name: other\n    role: lead\n    profile: seeker\n"+
			"workers:\n"+
			"  - from: lead\n    purpose: 調査\n    profile: seeker\n"+
			"turns:\n  - human -> lead\n  - lead -> other\n  - other -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  seeker:\n    kind: claude\n")
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:dev@dev"}`, "")

	got := mustRun(t, "hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry")
	if !strings.Contains(got, "member: dev-researcher") {
		t.Fatalf("hire without --from did not pick the one member hiring for 調査:\n%s", got)
	}

	// A second member hiring for the same purpose: the candidates are listed.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: seeker\n"+
			"  - name: other\n    role: lead\n    profile: seeker\n"+
			"workers:\n"+
			"  - from: lead\n    purpose: 調査\n    profile: seeker\n"+
			"  - from: other\n    purpose: 調査\n    profile: seeker\n"+
			"turns:\n  - human -> lead\n  - lead -> other\n  - other -> human\n")
	_, stderr, code := run("hire", "researcher", "--purpose", "調査", "--task", "read", "--team", "dev", "--dry")
	if code == 0 {
		t.Fatal("hire without --from succeeded with two members hiring for the purpose")
	}
	for _, want := range []string{"--from", "lead, other"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("the error does not name %q:\n%s", want, stderr)
		}
	}
	// The candidates are the short names team.yaml writes, not the herdr names.
	if strings.Contains(stderr, "dev-lead") || strings.Contains(stderr, "dev-other") {
		t.Fatalf("the error names the herdr names, not the short ones:\n%s", stderr)
	}
}

// TestUpDryPromptShowsWorkerPairs: the %{workers} note lists purpose-to-profile
// pairs, and the hire command it embeds needs no --profile. show and --dry
// --json carry the same pairs.
func TestUpDryPromptShowsWorkerPairs(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nprompt: \"%{role_doc}%{docs}%{workers}\"\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: seeker\n"+
			"workers:\n"+
			"  - from: lead\n    purpose: 調査\n    profile: seeker\n"+
			"  - from: lead\n    purpose: 実装\n    profile: developer\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  seeker:\n    kind: claude\n  developer:\n    kind: codex\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"),
		"version: 1\nsections:\n  - rules:\n      - id: L1\n        text: hand the turn to whoever acts next\n")

	got := mustRun(t, "up", "demo", "--dry", "--lang", "ja")
	for _, want := range []string{"## ワーカーの追加", "調査 → seeker", "実装 → developer", "hire <ワーカー名> --purpose <目的>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("up --dry is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "--profile") {
		t.Fatalf("the hire command in the prompt still carries --profile:\n%s", got)
	}

	shown := mustRun(t, "show", "demo", "--lang", "ja")
	if !strings.Contains(shown, "ワーカー: 調査 → seeker、実装 → developer\n") {
		t.Fatalf("show does not list the worker pairs:\n%s", shown)
	}

	raw := mustRun(t, "up", "demo", "--dry", "--json")
	var plan struct {
		Members []struct {
			Workers []struct {
				From    string `json:"from"`
				Purpose string `json:"purpose"`
				Profile string `json:"profile"`
			} `json:"workers"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		t.Fatalf("--json output is not JSON: %v\n%s", err, raw)
	}
	if len(plan.Members) == 0 || len(plan.Members[0].Workers) != 2 {
		t.Fatalf("--json does not carry two workers: %s", raw)
	}
	first := plan.Members[0].Workers[0]
	if first.From != "lead" || first.Purpose != "調査" || first.Profile != "seeker" {
		t.Fatalf("--json first worker = %+v, want lead/調査/seeker", first)
	}
}

// TestStatusShapeWaitsOnTheHuman: status prints one line per member and per
// worker, separates the worker, and ends with who is waiting on the human.
func TestStatusShapeWaitsOnTheHuman(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, statusHerdr)
	got := mustRun(t, "status", "demo")
	wantLines(t, got,
		"mission: what this team is for, in one line",
		"team: demo",
		"version: 1",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: .hergy/profiles.yaml",
		"demo-lead: working",
		"demo-builder: blocked (waiting for permission)",
		"demo-w1: idle (worker)",
		"waiting for a human: demo-builder",
	)
}

// TestStatusLabelsFollowTheLanguage: the header and the judgement follow --lang;
// the agent states are herdr's words, whatever the language.
func TestStatusLabelsFollowTheLanguage(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, statusHerdr)
	got := mustRun(t, "status", "demo", "--lang", "ja")
	for _, line := range []string{
		"チーム: demo",
		"demo-lead: working",
		"demo-builder: blocked（許可待ち）",
		"demo-w1: idle（ワーカー）",
		"人間の返事待ち: demo-builder",
	} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("Japanese status is missing %q:\n%s", line, got)
		}
	}
}

// TestStatusNobodyWorking: when every member is idle, the judgement says so.
func TestStatusNobodyWorking(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, liveHerdr)
	got := mustRun(t, "status", "demo")
	for _, line := range []string{
		"demo-lead: idle",
		"demo-builder: idle",
		"nobody is working",
	} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("status is missing %q:\n%s", line, got)
		}
	}
}

// TestStatusAbsentAndNoJudgement: a member that is not live reads "not present",
// and one working member with nobody blocked has no judgement line.
func TestStatusAbsentAndNoJudgement(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, absentHerdr)
	got := mustRun(t, "status", "demo")
	for _, line := range []string{"demo-lead: idle", "demo-builder: not present"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("status is missing %q:\n%s", line, got)
		}
	}
	if !strings.HasSuffix(got, "nobody is working\n") {
		t.Fatalf("status's last line is not the judgement:\n%s", got)
	}
}

// TestStatusWorkingHasNoJudgement: one working member and nobody blocked leaves
// no judgement line at all.
func TestStatusWorkingHasNoJudgement(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, statusHerdr)
	// demo-lead works and demo-builder is blocked here, so drop the blocked one by
	// selecting a team whose only member is working: rewrite team.yaml.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: a mission for the test\nmembers:\n  - name: lead\n    role: lead\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n")
	got := mustRun(t, "status", "demo")
	if !strings.Contains(got, "demo-lead: working\n") {
		t.Fatalf("status does not report the working member:\n%s", got)
	}
	for _, unwanted := range []string{"nobody is working", "waiting for a human"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("status printed the judgement %q with a working member:\n%s", unwanted, got)
		}
	}
}

// TestStatusDoesNotNeedAHerdrPane: status only reads, so it runs outside a
// herdr-managed pane (the fixture leaves HERDR_ENV unset).
func TestStatusDoesNotNeedAHerdrPane(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.herdr(t, statusHerdr)
	if _, stderr, code := run("status", "demo"); code != 0 {
		t.Fatalf("status outside herdr: exit %d\nstderr:\n%s", code, stderr)
	}
}

// TestCheatAIListsEveryCommandAndFlag reads the command and flag tables from
// options and checks the agent cheatsheet names each one, so a command or flag
// added later cannot be left out of the cheatsheet.
func TestCheatAIListsEveryCommandAndFlag(t *testing.T) {
	newFixture(t)
	got := mustRun(t, "cheat", "--ai")
	for _, command := range options.Commands() {
		if !strings.Contains(got, "hergy "+command) {
			t.Errorf("cheat --ai does not name the command %q", command)
		}
	}
	for _, kind := range options.Kinds {
		for _, flag := range options.Flags(kind) {
			if !strings.Contains(got, "--"+flag) {
				t.Errorf("cheat --ai does not name the flag --%s", flag)
			}
		}
	}
}

// TestCheatPeopleLeadsWithTheFirstSteps checks the short cheatsheet opens with
// the three steps a newcomer types, in order, and names every command.
func TestCheatPeopleLeadsWithTheFirstSteps(t *testing.T) {
	newFixture(t)
	tests := []struct {
		lang  string
		steps []string
	}{
		{"en", []string{"hergy new myteam", "edit team.yaml", "hergy up myteam"}},
		{"ja", []string{"hergy new myteam", "team.yaml を編集", "hergy up myteam"}},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			got := mustRun(t, "cheat", "--lang", tt.lang)
			at := -1
			for _, step := range tt.steps {
				next := strings.Index(got, step)
				if next < 0 {
					t.Fatalf("cheat (%s) does not hold the step %q:\n%s", tt.lang, step, got)
				}
				if next < at {
					t.Fatalf("cheat (%s) has the steps out of order at %q:\n%s", tt.lang, step, got)
				}
				at = next
			}
			for _, command := range options.Commands() {
				if !strings.Contains(got, "hergy "+command) {
					t.Errorf("cheat (%s) does not name the command %q", tt.lang, command)
				}
			}
		})
	}
}

// TestCheatNeedsNoSettingsOrHerdr: cheat reads no team settings and touches no
// Herdr, so it prints and exits 0 with no config.yaml, with a broken one, and
// with HERGY_HERDR pointed at a path that does not exist.
func TestCheatNeedsNoSettingsOrHerdr(t *testing.T) {
	f := newFixture(t)
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "no-such-herdr"))
	configPath := filepath.Join(f.cfg, "hergy", "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		t.Fatal("the fixture unexpectedly already has a config.yaml")
	}
	for _, argv := range [][]string{{"cheat"}, {"cheat", "--ai"}} {
		stdout, stderr, code := run(argv...)
		if code != 0 {
			t.Fatalf("%v: exit %d\nstderr:\n%s", argv, code, stderr)
		}
		if strings.TrimSpace(stdout) == "" {
			t.Fatalf("%v printed nothing", argv)
		}
	}
	// A broken config.yaml must not stop it either.
	writeFile(t, configPath, "version: 2\n")
	for _, argv := range [][]string{{"cheat"}, {"cheat", "--ai"}} {
		if _, stderr, code := run(argv...); code != 0 {
			t.Fatalf("%v with a broken config.yaml: exit %d\nstderr:\n%s", argv, code, stderr)
		}
	}
}

// TestCheatRejectsOtherFlags: cheat takes --ai and --lang only, so any other
// flag is refused rather than ignored.
func TestCheatRejectsOtherFlags(t *testing.T) {
	newFixture(t)
	for _, argv := range [][]string{
		{"cheat", "--dry"},
		{"cheat", "--ai", "--dry"},
		{"cheat", "--ai", "--json"},
		{"cheat", "--global"},
		{"cheat", "extra"},
	} {
		_, stderr, code := run(argv...)
		if code == 0 {
			t.Fatalf("%v succeeded, want an error", argv)
		}
		if !strings.Contains(stderr, "error:") {
			t.Fatalf("%v does not say why:\n%s", argv, stderr)
		}
		if strings.Contains(stderr, "[[") {
			t.Fatalf("%v prints an untranslated key:\n%s", argv, stderr)
		}
	}
}

// TestCheatAIIsEnglishWhateverLang: --lang does not translate the agent
// cheatsheet.
func TestCheatAIIsEnglishWhateverLang(t *testing.T) {
	newFixture(t)
	got := mustRun(t, "cheat", "--ai", "--lang", "ja")
	if strings.Contains(got, "短い早見") || strings.Contains(got, "はじめの3手") {
		t.Fatalf("cheat --ai --lang ja is not English:\n%s", got)
	}
	if !strings.Contains(got, "hergy cheat --ai") {
		t.Fatalf("cheat --ai --lang ja is not the agent cheatsheet:\n%s", got)
	}
}

// TestUpLabelsTheTeamTab: the team's tab carries both names, so a team command
// can find the team definition behind the team name.
func TestUpLabelsTheTeamTab(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	mustRun(t, "up", "demo")
	mustRun(t, "up", "demo", "--as", "alpha")

	record := readFile(t, log)
	for _, want := range []string{"--label demo", "--label alpha"} {
		if !strings.Contains(record, want) {
			t.Fatalf("up did not name the team tab %q:\n%s", want, record)
		}
	}
}

// TestTeamCommandFollowsTheTeamNameNotTheDefinition: two teams booted from one
// team definition are told apart by the team name in their tabs, so a command
// for one never shows the other.
func TestTeamCommandFollowsTheTeamNameNotTheDefinition(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	tabs := `{"tab_id":"t1","label":"team:qw@qw"},{"tab_id":"t2","label":"team:qw2@qw"}`
	agents := `{"name":"qw-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1"}` +
		`,{"name":"qw2-lead","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t2"}`
	f.runningHerdr(t, tabs, agents)

	first := mustRun(t, "status", "qw")
	for _, line := range []string{"team: qw", "qw-lead: working", "qw-builder: not present"} {
		if !strings.Contains(first, line+"\n") {
			t.Fatalf("status qw is missing %q:\n%s", line, first)
		}
	}
	if strings.Contains(first, "qw2-lead") {
		t.Fatalf("status qw shows the other team:\n%s", first)
	}

	second := mustRun(t, "status", "qw2")
	for _, line := range []string{"team: qw2 (from qw)", "qw2-lead: idle"} {
		if !strings.Contains(second, line+"\n") {
			t.Fatalf("status qw2 is missing %q:\n%s", line, second)
		}
	}
	if strings.Contains(second, "qw-lead:") {
		t.Fatalf("status qw2 shows the other team:\n%s", second)
	}
}

// TestTeamCommandReadsTheOlderTabLabel: a tab named "team:<team>" with no team
// definition names the team definition after the team.
func TestTeamCommandReadsTheOlderTabLabel(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:dev"}`,
		`{"name":"dev-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}`)
	got := mustRun(t, "status", "dev")
	for _, line := range []string{"team: dev", "dev-lead: idle"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("status dev is missing %q:\n%s", line, got)
		}
	}
}

// TestTeamCommandWithNoTabStops: a team name with no tab is an error that points
// at hergy status.
func TestTeamCommandWithNoTabStops(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev")
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:other@other"}`, "")
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	stdout, stderr, code := run("down", "dev")
	if code == 0 {
		t.Fatal("down dev succeeded with no dev tab")
	}
	if stdout != "" {
		t.Fatalf("down dev wrote to stdout: %q", stdout)
	}
	for _, want := range []string{`no running team "dev"`, "hergy status", "next: status"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("down dev does not say %q:\n%s", want, stderr)
		}
	}
}

// TestTeamCommandWithoutArgumentUsesTheOneRunningTeam: with one running team, a
// team command's omitted team name picks it.
func TestTeamCommandWithoutArgumentUsesTheOneRunningTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:qw@qw"}`, "")
	recordTeamLaunch(t, "qw")
	got := mustRun(t, "reprompt", "lead", "--dry")
	if !strings.HasSuffix(got, "next: reprompt lead --team qw\n") {
		t.Fatalf("reprompt with no team does not name the one running team:\n%s", got)
	}
}

// TestTeamCommandWithoutArgumentStopsWhenNone: with no running team, a team
// command's omitted team name stops.
func TestTeamCommandWithoutArgumentStopsWhenNone(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	f.runningHerdr(t, "", "")
	_, stderr, code := run("reprompt", "lead", "--dry")
	if code == 0 {
		t.Fatal("reprompt --dry with no team and no record succeeded")
	}
	for _, want := range []string{"--dry does not read herdr", "--team"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("reprompt --dry with no team does not say %q:\n%s", want, stderr)
		}
	}
}

// TestTeamCommandWithoutArgumentStopsWhenSeveral: with more than one running
// team, a team command's omitted team name stops and lists them.
func TestTeamCommandWithoutArgumentStopsWhenSeveral(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	tabs := `{"tab_id":"t1","label":"team:qw@qw"},{"tab_id":"t2","label":"team:qw2@qw"}`
	f.runningHerdr(t, tabs, "")
	recordTeamLaunch(t, "qw")
	recordTeamLaunch(t, "qw2")
	_, stderr, code := run("reprompt", "lead", "--dry")
	if code == 0 {
		t.Fatal("reprompt --dry with several records succeeded")
	}
	for _, want := range []string{"name a team", "qw", "qw2"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("reprompt --dry with several records does not say %q:\n%s", want, stderr)
		}
	}
}

// TestStatusListsEveryRunningTeam: status with no team name lists every running
// team, with its team definition, its counts, and the judgement.
func TestStatusListsEveryRunningTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	tabs := `{"tab_id":"t1","label":"team:qw@qw"},{"tab_id":"t2","label":"team:qw2@qw"},{"tab_id":"tw","label":"worker:qw/w1"}`
	agents := `{"name":"qw-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1"}` +
		`,{"name":"qw-builder","agent":"claude","agent_status":"blocked","pane_id":"p2","tab_id":"t1"}` +
		`,{"name":"qw-w1","agent":"claude","agent_status":"idle","pane_id":"p3","tab_id":"tw"}` +
		`,{"name":"qw2-lead","agent":"claude","agent_status":"idle","pane_id":"p4","tab_id":"t2"}` +
		`,{"name":"qw2-builder","agent":"claude","agent_status":"idle","pane_id":"p5","tab_id":"t2"}`
	f.runningHerdr(t, tabs, agents)
	got := mustRun(t, "status")
	wantLines(t, got,
		"team: qw",
		"  members: 2",
		"  workers: 1",
		"  waiting for a human: qw-builder",
		"",
		"team: qw2 (from qw)",
		"  members: 2",
		"  workers: 0",
		"  nobody is working",
	)
}

// TestStatusListKeepsGoingPastAnotherProjectsTeam: a running team whose
// definition is in another project is listed with a note, and the teams after
// it are still listed.
func TestStatusListKeepsGoingPastAnotherProjectsTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	tabs := `{"tab_id":"t0","label":"team:othersite"},{"tab_id":"t1","label":"team:qw@qw"}`
	agents := `{"name":"othersite-lead","agent":"claude","agent_status":"idle","pane_id":"p0","tab_id":"t0"}` +
		`,{"name":"qw-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}`
	f.runningHerdr(t, tabs, agents)
	got := mustRun(t, "status")
	wantLines(t, got,
		"team: othersite",
		"  the team definition is not in this project; run hergy status from the project that booted it",
		"",
		"team: qw",
		"  members: 2",
		"  workers: 0",
		"  nobody is working",
	)
}

// TestStatusWithoutRunningTeams: status with no team name and nothing running
// says so.
func TestStatusWithoutRunningTeams(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	f.runningHerdr(t, "", "")
	if got := mustRun(t, "status"); !strings.Contains(got, "no teams are running\n") {
		t.Fatalf("status with nothing running does not say so:\n%s", got)
	}
}

// TestLsShowsTheRunningTeams: ls reads herdr's tabs and names the running team
// names each team definition booted.
func TestLsShowsTheRunningTeams(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	tabs := `{"tab_id":"t1","label":"team:qw@qw"},{"tab_id":"t2","label":"team:qw2@qw"}`
	f.runningHerdr(t, tabs, "")
	got := mustRun(t, "ls")
	wantLines(t, got,
		"team definition: qw",
		"  path: .hergy/teams/qw/team.yaml",
		"  running: qw, qw2",
	)
}

// TestLsWithoutHerdrShowsOnlyDefinitions: when herdr cannot be read (outside
// herdr), ls shows the team definitions and no running teams.
func TestLsWithoutHerdrShowsOnlyDefinitions(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "qw")
	got := mustRun(t, "ls")
	if strings.Contains(got, "running:") {
		t.Fatalf("ls without herdr shows running teams:\n%s", got)
	}
	if !strings.Contains(got, "team definition: qw\n") {
		t.Fatalf("ls without herdr does not show the team definition:\n%s", got)
	}
}

// TestLsSaysWhenANameIsAlsoOutsideTheProject: a name written both in the project
// and outside it is read from the project, and ls says so, naming the place by
// its path rather than by the layer (段 7, H8).
func TestLsSaysWhenANameIsAlsoOutsideTheProject(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	writeFile(t, filepath.Join(f.cfg, "hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: outside the project\nmembers:\n  - name: a\n    role: lead\n    profile: claude-default\n")
	got := mustRun(t, "ls")
	want := i18n.Tf("note.elsewhere", filepath.Join(f.cfg, "hergy", "teams", "demo", "team.yaml"))
	if !strings.Contains(got, want+"\n") {
		t.Fatalf("ls does not say the name is also outside the project:\n%s", got)
	}
	if !strings.Contains(got, "  path: .hergy/teams/demo/team.yaml\n") {
		t.Fatalf("ls does not read the project's definition:\n%s", got)
	}
}

// TestTeamCommandTargetsTheNamedTeam: --team picks one running team by its team
// name, even when another team comes from the same team definition.
func TestTeamCommandTargetsTheNamedTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "qw")
	tabs := `{"tab_id":"t1","label":"team:qw@qw"},{"tab_id":"t2","label":"team:qw2@qw"}`
	f.runningHerdr(t, tabs, "")
	recordTeamLaunch(t, "qw2")
	got := mustRun(t, "reprompt", "lead", "--team", "qw2", "--dry")
	for _, line := range []string{"team: qw2 (from qw)", "member: qw2-lead"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("reprompt --team qw2 is missing %q:\n%s", line, got)
		}
	}
	if strings.Contains(got, "member: qw-lead") {
		t.Fatalf("reprompt --team qw2 picked the other team:\n%s", got)
	}
	if !strings.HasSuffix(got, "next: reprompt lead --team qw2\n") {
		t.Fatalf("reprompt --team qw2 does not end with next:\n%s", got)
	}
}

// TestTeamCommandsRejectAs: --as is up's flag alone, so a team command refuses
// it rather than ignoring it.
func TestTeamCommandsRejectAs(t *testing.T) {
	newFixture(t)
	for _, argv := range [][]string{
		{"status", "demo", "--as", "a"},
		{"down", "demo", "--as", "a"},
		{"refresh", "lead", "--as", "a"},
		{"reprompt", "lead", "--as", "a"},
		{"hire", "w", "--as", "a"},
	} {
		_, stderr, code := run(argv...)
		if code == 0 {
			t.Fatalf("%v succeeded, want an error", argv)
		}
		if !strings.Contains(stderr, "error:") {
			t.Fatalf("%v does not say why:\n%s", argv, stderr)
		}
	}
}
