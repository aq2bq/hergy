package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/options"
)

// passingHerdr is a fake herdr that answers every command the CLI may need, so
// a boot, a refresh or a down runs to the end.
const passingHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "agent list")   echo '{"result":{"agents":[]}}' ;;
  "tab list")     echo '{"result":{"tabs":[]}}' ;;
  "tab create")   echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")   echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// liveHerdr is a fake herdr that reports the demo team's two members as live, so
// a down has panes to close.
const liveHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1"},{"name":"demo-worker","agent":"claude","agent_status":"idle","pane_id":"p2"}]}}' ;;
  "agent get")
    case "$3" in
      demo-lead)   echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1"}}}' ;;
      demo-worker) echo '{"result":{"agent":{"name":"demo-worker","agent":"claude","agent_status":"idle","pane_id":"p2"}}}' ;;
      *) echo '{"error":{"code":"agent_not_found"}}' ;;
    esac
    ;;
  "tab list")   echo '{"result":{"tabs":[]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// statusHerdr is a fake herdr that reports the demo team's two members and one
// hired worker, with states a status run reports: one working, one blocked, and
// the worker idle.
const statusHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1"},{"name":"demo-worker","agent":"claude","agent_status":"blocked","pane_id":"p2","tab_id":"t1"},{"name":"demo-w1","agent":"claude","agent_status":"idle","pane_id":"p3","tab_id":"tw"}]}}' ;;
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
  for x in get prompt start list current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`

// fixture is one temporary project with a global config directory outside the
// home directory, so paths inside the project go relative and paths under the
// config directory stay absolute.
type fixture struct {
	proj string
	cfg  string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{proj: filepath.Join(root, "proj"), cfg: filepath.Join(root, "cfg")}
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
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("HERDR_ENV", "")
	t.Setenv("HERDR_PANE_ID", "")
	t.Setenv("HERGY_HERDR", "")
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

// statefulHerdr writes a fake herdr that remembers the agents it started, so a
// second boot under the same names collides in preflight the way real herdr
// would. It records every call it gets in <cfg>/herdr.log and returns that path,
// so a test can check a tab label or a closed pane.
func (f *fixture) statefulHerdr(t *testing.T) string {
	t.Helper()
	state := filepath.Join(f.cfg, "agents.txt")
	log := filepath.Join(f.cfg, "herdr.log")
	if err := os.WriteFile(state, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
state='` + state + `'
log='` + log + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "agent list")
    printf '{"result":{"agents":['
    first=1
    while read -r n; do
      [ -z "$n" ] && continue
      if [ $first -eq 1 ]; then first=0; else printf ','; fi
      printf '{"name":"%s","agent":"claude","agent_status":"idle","pane_id":"p-%s"}' "$n" "$n"
    done < "$state"
    printf ']}}'
    ;;
  "agent start")
    name="$3"
    if grep -qx "$name" "$state"; then
      echo '{"error":{"code":"name_in_use"}}'
      exit 1
    fi
    printf '%s\n' "$name" >> "$state"
    echo '{"result":{}}'
    ;;
  "tab list") echo '{"result":{"tabs":[]}}' ;;
  "tab create") echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
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
	got := mustRun(t, "new", "demo", "--local")
	wantLines(t, got,
		"team: demo",
		"layer: local",
		"dir: .hergy/teams/demo",
		"file: .hergy/teams/demo/team.yaml",
		"file: .hergy/teams/demo/protocol.md",
		"file: .hergy/teams/demo/roles/lead.md",
		"file: .hergy/teams/demo/roles/worker.md",
		"config: "+filepath.Join(f.cfg, "hergy", "config.yaml"),
		"next: up demo --dry",
	)
}

// TestNewCreatesTheGlobalConfigWhenMissing: a first new writes the global
// config.yaml (and the hergy/ directory) with version: 1 and the run's language,
// and reports the file as one more output line.
func TestNewCreatesTheGlobalConfigWhenMissing(t *testing.T) {
	f := newFixture(t)
	got := mustRun(t, "new", "demo", "--local")
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
		mustRun(t, "new", "demo", "--local", "--lang", "ja")
		wantLines(t, readFile(t, filepath.Join(f.cfg, "hergy", "config.yaml")), "version: 1", "lang: ja")
	})
	t.Run("no --lang", func(t *testing.T) {
		f := newFixture(t)
		mustRun(t, "new", "demo", "--local")
		wantLines(t, readFile(t, filepath.Join(f.cfg, "hergy", "config.yaml")), "version: 1", "lang: en")
	})
}

// TestNewLeavesAnExistingConfigAlone: when config.yaml is already there, new
// neither rewrites it nor reports it, whatever --lang says.
func TestNewLeavesAnExistingConfigAlone(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.cfg, "hergy", "config.yaml")
	writeFile(t, path, "version: 1\nlang: ja\n")
	got := mustRun(t, "new", "demo", "--local", "--lang", "en")
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
	mustRun(t, "new", "demo", "--local", "--lang", "ja")
	got := mustRun(t, "ls")
	if !strings.Contains(got, "チーム: demo\n") {
		t.Fatalf("ls after a Japanese new is not Japanese:\n%s", got)
	}
	if !strings.Contains(got, "ローカル: .hergy\n") {
		t.Fatalf("ls after a Japanese new is not Japanese:\n%s", got)
	}
}

// TestValidateAcceptsTheCreatedConfig: the config.yaml new writes is one
// validate reads without complaint.
func TestValidateAcceptsTheCreatedConfig(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	path := filepath.Join(f.cfg, "hergy", "config.yaml")
	got := mustRun(t, "validate")
	if !strings.Contains(got, "file: "+path+"\n  result: ok\n") {
		t.Fatalf("validate does not accept the created config.yaml:\n%s", got)
	}
	if !strings.Contains(got, "problems: 0\n") {
		t.Fatalf("validate found a problem in the created config.yaml:\n%s", got)
	}
}

func TestLsShape(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	got := mustRun(t, "ls")
	wantLines(t, got,
		"local: .hergy",
		"global: "+filepath.Join(f.cfg, "hergy"),
		"team: demo",
		"  layer: local",
		"  path: .hergy/teams/demo/team.yaml",
	)
}

func TestValidateShape(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	got := mustRun(t, "validate")
	wantLines(t, got,
		"file: "+filepath.Join(f.cfg, "hergy", "config.yaml"),
		"  result: ok",
		"file: .hergy/teams/demo/team.yaml",
		"  result: ok",
		"checked: 2",
		"problems: 0",
	)
}

func TestValidateShapeReportsABrokenFile(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
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
	if !strings.Contains(got, "checked: 2\nproblems: 1\n") {
		t.Fatalf("validate summary is not in the one shape:\n%s", got)
	}
}

func TestShowShape(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo", "--local")
	got := mustRun(t, "show", "demo")
	wantLines(t, got,
		"team: demo",
		"version: 1",
		"layer: local",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: none",
		"document: .hergy/teams/demo/protocol.md",
		"member: demo-lead",
		"  role: lead",
		"  kind: claude",
		"  cwd: .",
		"  role file: .hergy/teams/demo/roles/lead.md",
		"  reads: .hergy/teams/demo/protocol.md",
		"member: demo-worker",
		"  role: worker",
		"  kind: claude",
		"  cwd: .",
		"  role file: .hergy/teams/demo/roles/worker.md",
		"  reads: .hergy/teams/demo/protocol.md",
	)
}

func TestShowLabelsFollowTheLanguage(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo", "--local")
	got := mustRun(t, "show", "demo", "--lang", "ja")
	for _, line := range []string{"チーム: demo", "層: local", "担当: demo-lead", "  役割: lead", "  役割ファイル: .hergy/teams/demo/roles/lead.md"} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("Japanese output is missing %q:\n%s", line, got)
		}
	}
}

func TestUpDryShapePrintsThePromptUnescaped(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo", "--local")
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
		"  role file: .hergy/teams/demo/roles/lead.md",
		"  reads: .hergy/teams/demo/protocol.md",
		"  prompt: |",
		"    You are demo-lead (lead) on team demo.",
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

func TestUpDryJSONIsTheSameJSON(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo", "--local")
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

func TestRefreshDryEndsWithNext(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	// refresh --dry reads the live agents to show the same refusal the real
	// refresh would, so it needs a herdr that answers agent get.
	f.herdr(t, liveHerdr)
	// refresh points at the short seat name written in team.yaml, not the
	// herdr name hergy made from it.
	if got := mustRun(t, "refresh", "lead", "--dry"); !strings.HasSuffix(got, "next: refresh lead\n") {
		t.Fatalf("refresh <seat> --dry does not end with next:\n%s", got)
	}
	if got := mustRun(t, "refresh", "--all", "--dry"); !strings.HasSuffix(got, "next: refresh --all\n") {
		t.Fatalf("refresh --all --dry does not end with next:\n%s", got)
	}
	if got := mustRun(t, "reprompt", "lead", "--dry"); !strings.HasSuffix(got, "next: reprompt lead\n") {
		t.Fatalf("reprompt <seat> --dry does not end with next:\n%s", got)
	}
	if got := mustRun(t, "reprompt", "--all", "--dry"); !strings.HasSuffix(got, "next: reprompt --all\n") {
		t.Fatalf("reprompt --all --dry does not end with next:\n%s", got)
	}
}

func TestDownDryShape(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, liveHerdr)
	got := mustRun(t, "down", "demo", "--dry")
	wantLines(t, got,
		"team: demo",
		"version: 1",
		"layer: local",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: none",
		"demo-lead: would close (pane p1)",
		"demo-worker: would close (pane p2)",
		"would close: 2",
		"would skip: 0",
		"next: down demo",
	)
}

func TestDownDryWithoutLiveMembersHasNoNext(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, passingHerdr)
	got := mustRun(t, "down", "demo", "--dry")
	wantLines(t, got,
		"team: demo",
		"version: 1",
		"layer: local",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: none",
		"demo-lead: would skip (not present)",
		"demo-worker: would skip (not present)",
		"note: nothing to close (team \"demo\")",
	)
	if strings.Contains(got, "next:") {
		t.Fatalf("down --dry with nothing to close must not name a next step:\n%s", got)
	}
}

func TestUpProductionShowsNoPerMemberProvenance(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, passingHerdr)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	got := mustRun(t, "up", "demo")
	wantLines(t, got,
		"team: demo",
		"version: 1",
		"layer: local",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: none",
		"submitted: demo-lead (pane p1)",
		"submitted: demo-worker (pane p2)",
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
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, liveHerdr)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	got := mustRun(t, "down", "demo")
	wantLines(t, got,
		"team: demo",
		"version: 1",
		"layer: local",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: none",
		"demo-lead: closing (pane p1)",
		"demo-worker: closing (pane p2)",
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
	mustRun(t, "new", "demo", "--local")
	stdout, stderr, code := run("up", "nosuch")
	if code == 0 {
		t.Fatal("up nosuch succeeded")
	}
	if stdout != "" {
		t.Fatalf("up nosuch wrote to stdout: %q", stdout)
	}
	wantLines(t, stderr,
		`error: no team "nosuch"`,
		"  local: .hergy/teams/nosuch/team.yaml",
		"  global: "+filepath.Join(f.cfg, "hergy", "teams", "nosuch", "team.yaml"),
		"  candidates: demo",
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

func TestJSONNeedsDry(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo", "--local")
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
	if out, err := exec.Command("git", "-C", proj, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	t.Chdir(proj)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	got := mustRun(t, "new", "gteam")
	if !strings.Contains(got, "dir: ~/.config/hergy/teams/gteam\n") {
		t.Fatalf("a path under the home directory is not shown with ~:\n%s", got)
	}
}

// TestNewWritesShortMemberNames: hergy new writes the member names as the short
// seats, without quotes; hergy adds the team name when it boots them.
func TestNewWritesShortMemberNames(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	text := readFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"))
	for _, want := range []string{"  - name: lead\n", "  - name: worker\n"} {
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
// that name; without --as the settings name is the team name.
func TestUpAsDryUsesTheTeamName(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo", "--local")

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
		"team: alpha",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"member: alpha-lead",
		"member: alpha-worker",
		"    You are alpha-lead (lead) on team alpha.",
		"    - lead: alpha-lead (you)",
		"    - worker: alpha-worker",
	} {
		if !strings.Contains(named, line+"\n") {
			t.Fatalf("up demo --as alpha --dry is missing %q:\n%s", line, named)
		}
	}
	if !strings.HasSuffix(named, "next: up demo --as alpha\n") {
		t.Fatalf("up demo --as alpha --dry does not end with next:\n%s", named)
	}
}

// TestUpAsDev2PrefixesTheSeats: --as changes only the prefix; the seats stay
// lead and worker, and the herdr names, roster and self-intro become dev2-*.
func TestUpAsDev2PrefixesTheSeats(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "dev", "--local")
	got := mustRun(t, "up", "dev", "--as", "dev2", "--dry")
	for _, line := range []string{
		"team: dev2",
		"member: dev2-lead",
		"member: dev2-worker",
		"    You are dev2-lead (lead) on team dev2.",
		"    - lead: dev2-lead (you)",
		"    - worker: dev2-worker",
	} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("up dev --as dev2 --dry is missing %q:\n%s", line, got)
		}
	}
	if !strings.HasSuffix(got, "next: up dev --as dev2\n") {
		t.Fatalf("up dev --as dev2 --dry does not end with next:\n%s", got)
	}
}

// TestTwoTeamsWithTheSameSeatsDoNotCollide: two settings that both name a seat
// "lead" boot side by side. hergy prefixes each with its own team name, so the
// herdr names stay apart and the second boot does not collide.
func TestTwoTeamsWithTheSameSeatsDoNotCollide(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev", "--local")
	mustRun(t, "new", "discovery", "--local")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	dev := mustRun(t, "up", "dev")
	for _, line := range []string{"submitted: dev-lead (pane p1)", "submitted: dev-worker (pane p2)"} {
		if !strings.Contains(dev, line+"\n") {
			t.Fatalf("up dev is missing %q:\n%s", line, dev)
		}
	}
	discovery := mustRun(t, "up", "discovery")
	for _, line := range []string{"submitted: discovery-lead (pane p1)", "submitted: discovery-worker (pane p2)"} {
		if !strings.Contains(discovery, line+"\n") {
			t.Fatalf("up discovery is missing %q:\n%s", line, discovery)
		}
	}
}

// TestHireNamesTheWorkerAfterTheTeam: hergy hire takes the short worker name and
// gives the worker the herdr name "<team>-<name>", both in the plan and in the
// prompt the worker receives.
func TestHireNamesTheWorkerAfterTheTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev", "--local")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmembers:\n"+
			"  - name: lead\n    role: lead\n    kind: claude\n"+
			"    may_spawn:\n      purposes: [research]\n      profiles: [dev]\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")

	got := mustRun(t, "hire", "researcher", "--profile", "dev", "--purpose", "research",
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
// "worker:<team>/<short name>", so down finds it; with --as, the label carries
// the --as name.
func TestHireLabelsTheTabWithTheTeam(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "dev", "--local")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "dev", "team.yaml"),
		"version: 1\nmembers:\n"+
			"  - name: lead\n    role: lead\n    kind: claude\n"+
			"    may_spawn:\n      purposes: [research]\n      profiles: [dev]\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	mustRun(t, "hire", "researcher", "--profile", "dev", "--purpose", "research",
		"--task", "read the docs", "--team", "dev")
	record := readFile(t, log)
	if !strings.Contains(record, "--label worker:dev/researcher ") {
		t.Fatalf("hire did not label the tab with the team and short name:\n%s", record)
	}

	mustRun(t, "hire", "reviewer", "--profile", "dev", "--purpose", "research",
		"--task", "review the docs", "--team", "dev", "--as", "dev2")
	record = readFile(t, log)
	if !strings.Contains(record, "--label worker:dev2/reviewer ") {
		t.Fatalf("hire --as did not label the tab with the --as name:\n%s", record)
	}
}

// TestUpAsBootsTwoTeamsSideBySide: two team names from one settings both boot.
// The fake herdr remembers the agents it started, so ignoring --as would make the
// second boot collide in preflight.
func TestUpAsBootsTwoTeamsSideBySide(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	alpha := mustRun(t, "up", "demo", "--as", "alpha")
	for _, line := range []string{
		"team: alpha",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"submitted: alpha-lead (pane p1)",
		"submitted: alpha-worker (pane p2)",
		"booted: alpha (tab t1)",
	} {
		if !strings.Contains(alpha, line+"\n") {
			t.Fatalf("up demo --as alpha is missing %q:\n%s", line, alpha)
		}
	}

	beta := mustRun(t, "up", "demo", "--as", "beta")
	for _, line := range []string{
		"team: beta",
		"submitted: beta-lead (pane p1)",
		"submitted: beta-worker (pane p2)",
		"booted: beta (tab t1)",
	} {
		if !strings.Contains(beta, line+"\n") {
			t.Fatalf("up demo --as beta is missing %q:\n%s", line, beta)
		}
	}

	record := readFile(t, log)
	for _, want := range []string{"--label team:alpha", "--label team:beta"} {
		if !strings.Contains(record, want) {
			t.Fatalf("herdr was not asked for %q:\n%s", want, record)
		}
	}
}

// TestDownAsClosesOnlyThatTeamsMembers: down --as closes the named team's
// members and leaves the other team's panes alone.
func TestDownAsClosesOnlyThatTeamsMembers(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	log := f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	mustRun(t, "up", "demo", "--as", "alpha")
	mustRun(t, "up", "demo", "--as", "beta")

	got := mustRun(t, "down", "demo", "--as", "alpha")
	for _, line := range []string{
		"team: alpha",
		"alpha-lead: closing (pane p-alpha-lead)",
		"alpha-worker: closing (pane p-alpha-worker)",
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
	if !strings.Contains(closes, "pane close p-alpha-lead") || !strings.Contains(closes, "pane close p-alpha-worker") {
		t.Fatalf("down demo --as alpha did not close alpha's panes:\n%s", closes)
	}
	if strings.Contains(closes, "pane close p-beta-lead") || strings.Contains(closes, "pane close p-beta-worker") {
		t.Fatalf("down demo --as alpha closed a beta pane:\n%s", closes)
	}
}

// TestUpStopsOnTeamPlaceholderInMemberName: the team name is added to every
// member name automatically, so a literal %{team} in a member name is refused
// with a message that says not to write it, --as or not.
func TestUpStopsOnTeamPlaceholderInMemberName(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmembers:\n"+
			"  - name: \"%{team}-lead\"\n    role: lead\n    kind: claude\n")

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

// TestHireCommandCarriesAs: the hire command written into a prompt names the
// settings and adds --as only when the team name differs from it, and --from
// carries the short seat name, not the herdr name.
func TestHireCommandCarriesAs(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nprompt: \"%{name}\\n%{spawn}\"\nmembers:\n"+
			"  - name: lead\n    role: lead\n    kind: claude\n"+
			"    may_spawn:\n      purposes: [research]\n      profiles: [dev]\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  dev:\n    kind: claude\n")

	plain := mustRun(t, "up", "demo", "--dry")
	if !strings.Contains(plain, "--team demo --local --lang en") {
		t.Fatalf("the hire command without --as is wrong:\n%s", plain)
	}
	if !strings.Contains(plain, "--from lead") {
		t.Fatalf("the hire command does not take the short seat:\n%s", plain)
	}
	if strings.Contains(plain, "--as") {
		t.Fatalf("the hire command without --as carries one:\n%s", plain)
	}

	named := mustRun(t, "up", "demo", "--as", "alpha", "--dry")
	if !strings.Contains(named, "--team demo --as alpha --local --lang en") {
		t.Fatalf("the hire command with --as is wrong:\n%s", named)
	}
	if !strings.Contains(named, "--from lead") {
		t.Fatalf("the hire command with --as does not take the short seat:\n%s", named)
	}
}

// TestStatusShapeWaitsOnTheHuman: status prints one line per member and per
// worker, separates the worker, and ends with who is waiting on the human.
func TestStatusShapeWaitsOnTheHuman(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, statusHerdr)
	got := mustRun(t, "status", "demo")
	wantLines(t, got,
		"team: demo",
		"version: 1",
		"layer: local",
		"team.yaml: .hergy/teams/demo/team.yaml",
		"profiles: none",
		"demo-lead: working",
		"demo-worker: blocked",
		"demo-w1: idle (worker)",
		"waiting for a human: demo-worker",
	)
}

// TestStatusLabelsFollowTheLanguage: the header and the judgement follow --lang;
// the agent states are herdr's words, whatever the language.
func TestStatusLabelsFollowTheLanguage(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, statusHerdr)
	got := mustRun(t, "status", "demo", "--lang", "ja")
	for _, line := range []string{
		"チーム: demo",
		"demo-lead: working",
		"demo-worker: blocked",
		"demo-w1: idle（ワーカー）",
		"人間待ち: demo-worker",
	} {
		if !strings.Contains(got, line+"\n") {
			t.Fatalf("Japanese status is missing %q:\n%s", line, got)
		}
	}
}

// TestStatusNobodyWorking: when every member is idle, the judgement says so.
func TestStatusNobodyWorking(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, liveHerdr)
	got := mustRun(t, "status", "demo")
	for _, line := range []string{
		"demo-lead: idle",
		"demo-worker: idle",
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
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, absentHerdr)
	got := mustRun(t, "status", "demo")
	for _, line := range []string{"demo-lead: idle", "demo-worker: not present"} {
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
	mustRun(t, "new", "demo", "--local")
	f.herdr(t, statusHerdr)
	// demo-lead works and demo-worker is blocked here, so drop the blocked one by
	// selecting a team whose only member is working: rewrite team.yaml.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmembers:\n  - name: lead\n    role: lead\n    kind: claude\n")
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
	mustRun(t, "new", "demo", "--local")
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
		{"ja", []string{"hergy new myteam", "team.yaml を書き換える", "hergy up myteam"}},
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
