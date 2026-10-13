package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// passingHerdr is a fake herdr that answers every command the CLI may need, so
// a boot, a refresh or a down runs to the end.
const passingHerdr = `#!/bin/sh
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list current split rename close create; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "agent list")   echo '{"result":{"agents":[]}}' ;;
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
  for x in get prompt start list current split rename close create; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1"},{"name":"demo-worker","agent":"claude","agent_status":"idle","pane_id":"p2"}]}}' ;;
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
	newFixture(t)
	got := mustRun(t, "new", "demo", "--local")
	wantLines(t, got,
		"team: demo",
		"layer: local",
		"dir: .hergy/teams/demo",
		"file: .hergy/teams/demo/team.yaml",
		"file: .hergy/teams/demo/protocol.md",
		"file: .hergy/teams/demo/roles/lead.md",
		"file: .hergy/teams/demo/roles/worker.md",
		"next: up demo --dry",
	)
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
	newFixture(t)
	mustRun(t, "new", "demo", "--local")
	got := mustRun(t, "validate")
	wantLines(t, got,
		"file: .hergy/teams/demo/team.yaml",
		"  result: ok",
		"checked: 1",
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
	if !strings.Contains(got, "checked: 1\nproblems: 1\n") {
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
	newFixture(t)
	mustRun(t, "new", "demo", "--local")
	if got := mustRun(t, "refresh", "demo-lead", "--dry"); !strings.HasSuffix(got, "next: refresh demo-lead\n") {
		t.Fatalf("refresh <member> --dry does not end with next:\n%s", got)
	}
	if got := mustRun(t, "refresh", "--all", "--dry"); !strings.HasSuffix(got, "next: refresh --all\n") {
		t.Fatalf("refresh --all --dry does not end with next:\n%s", got)
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
