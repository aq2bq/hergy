package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"
	"github.com/aq2bq/hergy/internal/ops/internal/team" // passingHerdr is a fake herdr that answers every command the CLI may need, so

	// a boot, a refresh or a down runs to the end.
	// liveHerdr is a fake herdr that reports the demo team's two members as live, so
	// a down has panes to close.
	"github.com/aq2bq/hergy/internal/source"
)

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

const liveHerdr = `#!/bin/sh
closed="${HERGY_HERDR}.closed"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "agent list") echo '{"result":{"agents":[{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1"},{"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2"}]}}' ;;
  "pane close") touch "$closed"; echo '{"result":{}}' ;;
  "agent get")
    if [ -f "$closed" ]; then echo '{"error":{"code":"agent_not_found"}}'; exit 0; fi
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
      demo-lead|p1) echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}}}' ;;
      *) echo '{"error":{"code":"agent_not_found"}}' ;;
    esac
    ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo@demo"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[{"pane_id":"p1","label":"demo-lead","tab_id":"t1"}]}}' ;;
  "pane split") echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  "pane get") echo '{"result":{"pane":{"pane_id":"p2","scroll":{"viewport_rows":20}}}}' ;;
  "pane process-info") echo '{"result":{"process_info":{"foreground_process_group_id":900,"shell_pid":900,"pane_id":"p1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return log
}
func wantLines(t *testing.T, got string, want ...string) {
	t.Helper()
	wantText := strings.Join(want, "\n") + "\n"
	if got != wantText {
		t.Fatalf("output differs\n got:\n%s\nwant:\n%s", got, wantText)
	}
}

// TestSharedAgreementIsNotReadAndStops: the agreement every team used to share
// (agreement.md beside the teams, in either place) does not reach a member's
// prompt any more; while one is left, the commands that build a prompt and
// validate stop on it, name the path and say where the rules go (d-31ad,
// r-7de2, ac-b7b6).
// sharedID finds the shared agreement's rule id S1 as an id of its own: hergy's
// ids are four random characters, and one of them (MJS1) may hold S1 inside it.
var sharedID = regexp.MustCompile(`(^|[^A-Z0-9])S1([^A-Z0-9]|$)`)

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
		"version: 1\ndescription: a description for the test\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: seeker\n"+
			"workers:\n"+
			"  - from: lead\n    purpose: 調査\n    profile: seeker\n"+
			"  - from: lead\n    purpose: 実装\n    profile: developer\n"+
			"turns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  seeker:\n    kind: claude\n  developer:\n    kind: codex\n")
}
func applyLeftHerdr(t *testing.T, f *fixture) {
	t.Helper()
	const builder = `{"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t1","state_change_seq":1}`
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1","state_change_seq":1},`+builder)
	path := filepath.Join(f.cfg, "herdr")
	script := strings.Replace(readFile(t, path), `case "$g $s" in`,
		`case "$g $s" in
  "agent get") echo '{"result":{"agent":`+builder+`}}' ;;`, 1)
	f.herdr(t, script)
}
func checkApplyLeftJSON(t *testing.T, stdout, why string) {
	t.Helper()
	var list wireApplyList
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("apply --dry --json: %v\n%s", err, stdout)
	}
	for _, team := range list.Teams {
		for _, member := range team.Members {
			if member.Name == "demo-lead" && member.Short == "lead" && !member.CanApply && member.WhyNot == why {
				return
			}
		}
	}
	t.Errorf("JSON does not name the member left with reason %q:\n%s", why, stdout)
}

// recordAgain writes the launch record of the demo team as it stands now, the
// way an up would: the point a test starts from when it changes the definition
// after a member was handed its prompt.
func recordAgain(t *testing.T) {
	t.Helper()
	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	r, err := ReadDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.Starts, launchlog.Workspace{}); err != nil {
		t.Fatalf("record demo: %v", err)
	}
}

// japaneseDemo is the demo team made and launched with the global config saying
// ja, so the prompts and the launch record are Japanese, the way a Japanese
// project writes them.
func japaneseDemo(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nlang: ja\n")
	mustRun(t, "new", "demo")
	useJapanese(t)
	recordAgain(t)
	return f
}

// changedWorkerProfile gives the demo team a worker and changes that worker's
// profile after the launch, and answers the team's note about it.
func changedWorkerProfile(t *testing.T, f *fixture) string {
	t.Helper()
	teamPath := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	writeFile(t, teamPath, readFile(t, teamPath)+"workers:\n  - from: lead\n    purpose: 調査\n    profile: claude-default\n")
	recordAgain(t)

	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n  scout:\n    kind: claude\n")
	writeFile(t, teamPath, strings.Replace(readFile(t, teamPath),
		"purpose: 調査\n    profile: claude-default", "purpose: 調査\n    profile: scout", 1))
	return i18n.Tf("apply.note_worker", "調査")
}
func dryJSONForCheat(t *testing.T, command string) map[string]any {
	t.Helper()
	f := applyDemo(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	writeFile(t, path, readFile(t, path)+"workers:\n  - from: lead\n    purpose: inspect\n    profile: claude-default\n")
	recordAgain(t)
	f.herdr(t, liveHerdr)
	args := map[string][]string{
		"up":       {"up", "demo"},
		"refresh":  {"refresh", "lead", "--team", "demo"},
		"reprompt": {"reprompt", "--all", "--team", "demo"},
		"hire":     {"hire", "scout", "--purpose", "inspect", "--task", "read", "--team", "demo"},
		"apply":    {"apply", "--team", "demo"},
		"down":     {"down", "demo"},
	}[command]
	wantCode := 0
	if command == "apply" {
		for _, member := range []string{"lead", "builder"} {
			role := filepath.Join(f.proj, ".hergy", "roles", member+".md")
			writeFile(t, role, readFile(t, role)+"\nName the files to read.\n")
		}
		applyLeftHerdr(t, f)
		wantCode = 1
	}
	args = append(args, "--dry", "--json")
	stdout, stderr, code := run(args...)
	if code != wantCode {
		t.Fatalf("%v: exit %d, want %d\n%s\n%s", args, code, wantCode, stdout, stderr)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, stdout)
	}
	return plan
}
func checkCheatJSONMembers(t *testing.T, doc, command string, plan map[string]any) {
	t.Helper()
	if command == "down" {
		if !strings.Contains(doc, "\n    down members[]: herdr name strings\n") {
			t.Fatal("cheat --ai does not describe down members as herdr name strings")
		}
		members, ok := plan["members"].([]any)
		if !ok || len(members) == 0 {
			t.Fatalf("down members = %#v, want a nonempty list", plan["members"])
		}
		for _, member := range members {
			if _, ok := member.(string); !ok {
				t.Errorf("down member = %#v, want a herdr name string", member)
			}
		}
		return
	}
	label := "up/refresh/reprompt/hire members[]"
	members := plan["members"]
	if command == "apply" {
		label = "apply teams[].members[]"
		teams := jsonObjectList(t, plan["teams"])
		members = teams[0]["members"]
	}
	always, optional := cheatJSONKeys(t, doc, label)
	applyStates := map[bool]bool{}
	for _, member := range jsonObjectList(t, members) {
		if command == "apply" {
			canApply, ok := member["can_apply"].(bool)
			if !ok {
				t.Fatalf("apply can_apply = %#v, want a boolean", member["can_apply"])
			}
			applyStates[canApply] = true
		}
		want := slices.Clone(always)
		for _, key := range optional {
			_, present := member[key]
			if command == "apply" {
				present = member["can_apply"] == false
			}
			if present {
				want = append(want, key)
			}
		}
		sameJSONKeys(t, member, want)
	}
	if command == "apply" && (!applyStates[true] || !applyStates[false]) {
		t.Fatal("apply fixture must exercise members with and without why_not")
	}
}

// The cheat uses one explicit row per shape, with always and optional lists.
// Read those lists instead of keeping another copy of the keys in the tests.
func cheatJSONKeys(t *testing.T, doc, label string) ([]string, []string) {
	t.Helper()
	prefix := "    " + label + ": always: "
	for line := range strings.SplitSeq(doc, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		required, conditional, hasConditional := strings.Cut(strings.TrimPrefix(line, prefix), "; optional: ")
		var optional []string
		if hasConditional {
			optional = strings.Split(conditional, ", ")
		}
		return strings.Split(required, ", "), optional
	}
	t.Fatalf("cheat --ai has no JSON key description for %q", label)
	return nil, nil
}
func sameJSONKeys(t *testing.T, object map[string]any, want []string) {
	t.Helper()
	got := slices.Sorted(maps.Keys(object))
	want = slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Errorf("JSON keys = %v, cheat --ai says %v", got, want)
	}
}
func jsonObjectList(t *testing.T, raw any) []map[string]any {
	t.Helper()
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("JSON list = %#v, want a nonempty list", raw)
	}
	objects := make([]map[string]any, len(items))
	for i, item := range items {
		objects[i], ok = item.(map[string]any)
		if !ok {
			t.Fatalf("JSON member = %#v, want an object", item)
		}
	}
	return objects
}

// controlHerdr writes a fake herdr for the member commands (focus, nudge, stop,
// tell) over one running team, demo. <cfg>/status.txt holds one live agent per
// line, "<name> <status> <pane> <screen>": agent list, agent get and agent read
// answer from it. Every call is recorded in <cfg>/herdr.log, and every prompt in
// <cfg>/prompts.txt as "<target>\t<text>" with newlines turned to spaces, so a
// test can read the body a member was handed.
//
// agent send-keys counts the escs sent to each member in <cfg>/esc.txt and, when
// a test wrote a stop plan, moves the member to the state that plan gives after
// that count (<cfg>/stop-plan.txt, one "<herdr name> <esc count> <status>" line
// per step). That is how the fake herdr stands for a harness that stops on the
// first esc, one that stops on the second, and one that never st
func (f *fixture) controlHerdr(t *testing.T, agents string) (logPath, promptsPath string) {
	t.Helper()
	status := filepath.Join(f.cfg, "status.txt")
	logPath = filepath.Join(f.cfg, "herdr.log")
	promptsPath = filepath.Join(f.cfg, "prompts.txt")
	esc := filepath.Join(f.cfg, "esc.txt")
	plan := filepath.Join(f.cfg, "stop-plan.txt")
	if err := os.WriteFile(status, []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
status='` + status + `'
log='` + logPath + `'
prompts='` + promptsPath + `'
esc='` + esc + `'
plan='` + plan + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[{"tab_id":"t1","label":"team:demo"}]}}' ;;
  "agent list")
    printf '{"result":{"agents":['
    first=1
    while read -r name st pane screen; do
      [ -z "$name" ] && continue
      if [ $first -eq 1 ]; then first=0; else printf ','; fi
      printf '{"name":"%s","agent":"claude","agent_status":"%s","pane_id":"%s","tab_id":"t1"}' "$name" "$st" "$pane"
    done < "$status"
    printf ']}}'
    ;;
  "agent get")
    name="$3"
    case "$3" in p-*) name="${3#p-}" ;; esac
    while read -r n st pane screen; do
      [ -z "$n" ] && continue
      if [ "$n" = "$name" ] || [ "$pane" = "$3" ]; then
        printf '{"result":{"agent":{"name":"%s","agent":"claude","agent_status":"%s","pane_id":"%s","tab_id":"t1"}}}\n' "$n" "$st" "$pane"
        exit 0
      fi
    done < "$status"
    echo '{"error":{"code":"agent_not_found"}}'
    ;;
  "agent read")
    while read -r n st pane screen; do
      [ -z "$n" ] && continue
      if [ "$n" = "$3" ]; then printf '%s' "$screen"; exit 0; fi
    done < "$status"
    ;;
  "agent prompt")
    printf '%s\t' "$3" >> "$prompts"
    printf '%s' "$4" | tr '\n' ' ' >> "$prompts"
    printf '\n' >> "$prompts"
    echo '{"result":{}}'
    ;;
  "agent focus")     echo '{"result":{}}' ;;
  "agent send-keys")
    name="$3"
    case "$3" in p-*) name="${3#p-}" ;; esac
    n=0
    if [ -f "$esc" ]; then n=$(cat "$esc"); fi
    n=$((n+1))
    printf '%s' "$n" > "$esc"
    if [ -f "$plan" ]; then
      new=$(awk -v n="$n" -v nm="$name" '$1==nm && $2==n {print $3}' "$plan")
      if [ -n "$new" ]; then
        awk -v nm="$name" -v st="$new" '{ if ($1==nm) $2=st; print }' "$status" > "$status.new"
        mv "$status.new" "$status"
      fi
    fi
    echo '{"result":{}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return logPath, promptsPath
}

// stopPlan tells the fake herdr when each esc ends a member's turn: one
// "<herdr name> <esc count> <status>" line per step. A plan with no line for a
// member leaves it working through every esc.
func (f *fixture) stopPlan(t *testing.T, plan string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.cfg, "stop-plan.txt"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
}

// logCallCount is how many times the fake herdr recorded a call whose first two
// words are group and sub.
func logCallCount(t *testing.T, path, group, sub string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == group && fields[1] == sub {
			n++
		}
	}
	return n
}

// mustFail runs a command that is expected to exit non-zero and returns what a
// person would see.
func mustFail(t *testing.T, argv ...string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, code := run(argv...)
	if code == 0 {
		t.Fatalf("%v: exit 0, want a failure\nstdout:\n%s", argv, stdout)
	}
	return stdout, stderr
}

// promptsRead returns the prompts a fake herdr recorded, one per line.
func promptsRead(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// logHasCall reports whether the fake herdr recorded a call whose first two
// words are group and sub.
func logHasCall(t *testing.T, path, group, sub string) bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == group && fields[1] == sub {
			return true
		}
	}
	return false
}

// descriptionTeam writes one local team definition whose team.yaml writes the given
// description line (empty for a definition that writes none), over the starter
// team's own files, so show, up --dry and validate read a definition. The
// starter files are written the first time only; every later call rewrites
// team.yaml alone.
func descriptionTeam(t *testing.T, f *fixture, description string) {
	t.Helper()
	starter := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.md")
	if _, err := os.Stat(starter); err != nil {
		mustRun(t, "new", "demo")
	}
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	body := "version: 1\n" + description +
		"members:\n  - name: lead\n    role: lead\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n"
	writeFile(t, path, body)
}

// stoppedTeamHerdr writes a fake herdr of a team that stopped before any agent
// ran: the tab hergy labelled with the team's name, the panes it named after the
// members, one pane a person opened (no name), and another team's tab and pane.
// No agent is live. It records every call in <cfg>/herdr.log and returns the log.
func (f *fixture) stoppedTeamHerdr(t *testing.T) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
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
  "agent get") echo '{"error":{"code":"agent_not_found"}}' ;;
  "agent list") echo '{"result":{"agents":[]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"turnwalk"},{"tab_id":"t2","label":"other"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[{"pane_id":"p1","label":"turnwalk-lead","tab_id":"t1"},{"pane_id":"p2","label":"turnwalk-builder","tab_id":"t1"},{"pane_id":"p3","tab_id":"t1"},{"pane_id":"p9","label":"other-lead","tab_id":"t2"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	return log
}

// workerHerdr writes a fake herdr with the team draft running and two workers
// hired into tabs of their own. withBenchTeam adds a running team named bench, so
// a test can make one name a team's and a worker's at once. It records every call
// in <cfg>/herdr.log and returns the log.
func (f *fixture) workerHerdr(t *testing.T, withBenchTeam bool) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
	tabs := `{"tab_id":"t1","label":"draft"},{"tab_id":"t2","label":"worker:draft/bench"},{"tab_id":"t3","label":"worker:draft/scout"}`
	agents := `{"name":"draft-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"},{"name":"draft-bench","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t2"},{"name":"draft-scout","agent":"claude","agent_status":"idle","pane_id":"p3","tab_id":"t3"}`
	panes := `{"pane_id":"p1","label":"draft-lead","tab_id":"t1"},{"pane_id":"p2","label":"draft-bench","tab_id":"t2"},{"pane_id":"p3","label":"draft-scout","tab_id":"t3"}`
	if withBenchTeam {
		tabs += `,{"tab_id":"t4","label":"bench"}`
		agents += `,{"name":"bench-lead","agent":"claude","agent_status":"idle","pane_id":"p4","tab_id":"t4"}`
		panes += `,{"pane_id":"p4","label":"bench-lead","tab_id":"t4"}`
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
  "agent get") echo '{"error":{"code":"agent_not_found"}}' ;;
  "agent list") echo '{"result":{"agents":[`+agents+`]}}' ;;
  "tab list")   echo '{"result":{"tabs":[`+tabs+`]}}' ;;
  "pane list")  echo '{"result":{"panes":[`+panes+`]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	return log
}

// gitIn runs one git command in dir for a test that lays out worktrees.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// leftoversHerdr writes a fake herdr with the team draft running in one pane
// standing in cwd. It records every call in <cfg>/herdr.log and returns the log.
func (f *fixture) leftoversHerdr(t *testing.T, cwd string) {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
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
  "agent get") echo '{"error":{"code":"agent_not_found"}}' ;;
  "agent list") echo '{"result":{"agents":[{"name":"draft-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"draft"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[{"pane_id":"p1","label":"draft-lead","tab_id":"t1","cwd":"`+cwd+`"}]}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
}
func recordTeamLaunch(t *testing.T, team string) {
	t.Helper()
	env := serveEnv(t)
	r, err := ReadDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "qw", team)
	if err != nil {
		t.Fatalf("read qw as %s: %v", team, err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.Starts, launchlog.Workspace{}); err != nil {
		t.Fatalf("record %s: %v", team, err)
	}
}

// recordDev records one launch of the dev definition from the files on disk,
// the way up does after a boot, and returns what it appended.
func recordDev(t *testing.T, f *fixture) launchlog.Record {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	r, err := ReadDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "dev", "")
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	launchWarning(&stderr, recordLaunch(env, r, launchlog.CommandUp, r.Starts))
	if stderr.Len() > 0 {
		t.Fatalf("recording dev failed: %s", stderr.String())
	}
	_, records := launchLog(t, f)
	if len(records) == 0 {
		t.Fatal("recording dev wrote no record")
	}
	return records[len(records)-1]
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

// ledgerProject is a project with one team definition that writes ledger: rules,
// a gy.toml, and the profiles and role documents the definition reads. script,
// when it is not empty, is written as the fake gy HERGY_GY points at; empty
// leaves the fixture's no-gy path, which is no gy at all.
func ledgerProject(t *testing.T, script string) *fixture {
	t.Helper()
	f := newFixture(t)
	writeFile(t, filepath.Join(f.proj, "gy.toml"), "[scopes.common]\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	for _, role := range []string{"lead", "designer"} {
		writeFile(t, filepath.Join(f.proj, ".hergy", "roles", role+".md"),
			"- one\n")
	}
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\ndescription: a description for the test\nmembers:\n"+
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

// recordStart writes one up line of the launch record for a team in the
// fixture's project, at the time a test says, the way up writes it after a boot.
func recordStart(t *testing.T, f *fixture, teamName string, at time.Time) {
	t.Helper()
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	if err := log.Append(launchlog.Record{
		Time: at.Format(time.RFC3339), Command: launchlog.CommandUp,
		Project: observedplace.Observe(f.proj), Team: fixtureSeen(teamName), Definition: teamName,
	}); err != nil {
		t.Fatal(err)
	}
}

// aliasProjectRun keeps the definition name different from the running team's
// name, and reports the pane cwd with the opposite spelling to the reader.
func aliasProjectRun(t *testing.T, recordAlias bool) (Environment, string) {
	t.Helper()
	f := applyDemo(t)
	alias := filepath.Join(filepath.Dir(f.proj), "project-alias")
	if err := os.Symlink(f.proj, alias); err != nil {
		t.Fatal(err)
	}
	recorded, readPath := f.proj, alias
	if recordAlias {
		recorded, readPath = alias, f.proj
	}
	env := serveEnv(t)
	r, err := ReadDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "demo", "demo-hotfix")
	if err != nil {
		t.Fatal(err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.Starts, launchlog.Workspace{}); err != nil {
		t.Fatal(err)
	}
	records, err := log.Read()
	if err != nil {
		t.Fatal(err)
	}
	record := records[len(records)-1]

	record.Project = observedplace.Observe(recorded)
	record.TeamYAML = observedplace.Observe(filepath.Join(recorded, ".hergy", "teams", "demo", "team.yaml"))
	if err := log.Append(record); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo-hotfix"}`,
		`{"name":"demo-hotfix-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1},
   {"name":"demo-hotfix-builder","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t1","state_change_seq":1}`)
	script := readFile(t, os.Getenv("HERGY_HERDR"))
	panes := `  "pane list") echo '{"result":{"panes":[{"pane_id":"p1","label":"demo-hotfix-lead","tab_id":"t1","cwd":"` + readPath + `"}]}}' ;;` + "\n"
	f.herdr(t, strings.Replace(script, "case \"$g $s\" in\n", "case \"$g $s\" in\n"+panes, 1))
	projectEnv, err := env.WithProject(projectPath(readPath))
	if err != nil {
		t.Fatal(err)
	}
	return projectEnv, readPath
}
func fixtureDefinitionName(s string) domainname.DefinitionName {
	n, _ := domainname.ParseDefinitionName(s)
	return n
}
func fixtureHerdrName(s string) domainname.HerdrName {
	var n domainname.HerdrName
	if e := n.UnmarshalText([]byte(s)); e != nil {
		panic(e)
	}
	return n
}
func fixtureShortName(s string) domainname.ShortName {
	if s == "" {
		return domainname.ShortName{}
	}
	n, e := domainname.ParseMemberName(s)
	if e != nil {
		panic(e)
	}
	return n.Short()
}
func namingStrings(ns []domainname.Naming) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.String()
	}
	return out
}

type startQuestionPort struct{ herdr.Port }
type questionAfterStartPort struct {
	herdr.Port
	started bool
}

// blockedStartHerdr writes a fake herdr whose start is refused because the agent
// asks a person a question while it starts, and whose agent stays blocked until
// <cfg>/answered exists: a person answering in the pane is that file appearing.
// It records every call in <cfg>/herdr.log and returns the log's path.
func (f *fixture) blockedStartHerdr(t *testing.T) string {
	t.Helper()
	log := filepath.Join(f.cfg, "herdr.log")
	answered := filepath.Join(f.cfg, "answered")
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
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "agent list")   echo '{"result":{"agents":[]}}' ;;
  "tab create")   echo '{"result":{"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "pane split")   echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  "agent start")  echo '{"error":{"code":"agent_not_ready","message":"agent demo-lead is blocked during startup and is not ready for prompts"}}' ;;
  "agent get")
    if [ -f '` + answered + `' ]; then
      echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}}}'
    else
      echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"blocked","pane_id":"p1","tab_id":"t1"}}}'
    fi
    ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, script)
	return log
}

// lockedBuffer is a buffer one goroutine may read while another writes. The boot
// runs in its own goroutine, as it does under hergy, and the test reads what it
// printed while it waits for the person's answer.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// runOperation runs one Herdr operation, and on Ctrl-C stops with 130. failure,
// when set, says where a failed operation stopped; interrupted, when set, says
// where an interrupted one stopped. The two differ: an interruption while hergy
// waits on a member's question adds how to go on.
func runOperation(ctx context.Context, stderr io.Writer, failure, interrupted func(io.Writer), fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		if err != nil {
			fail(stderr, err)
			if failure != nil {
				failure(stderr)
			}
			return reportedError{err: err, code: 1}
		}
		return nil
	case <-ctx.Done():
		fmt.Fprint(stderr, i18n.T("launcher.interrupted"))
		if interrupted != nil {
			interrupted(stderr)
		}
		return reportedError{err: context.Canceled, code: 130}
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
  "agent get") echo '{"error":{"code":"agent_not_found"}}' ;;
  "agent list") echo '{"result":{"agents":[{"name":"qw2-m","agent":"codex","agent_status":"blocked","pane_id":"p-qw2-m","tab_id":"t1"},{"name":"qw2-x-lead","agent":"claude","agent_status":"idle","pane_id":"p-qw2-x-lead","tab_id":"t2"}]}}' ;;
  "tab list")   echo '{"result":{"tabs":[{"tab_id":"t1","label":"qw2"},{"tab_id":"t2","label":"qw2-x"}]}}' ;;
  "pane list")  echo '{"result":{"panes":[{"pane_id":"p-qw2-m","label":"qw2-m","tab_id":"t1"},{"pane_id":"p-qw2-x-lead","label":"qw2-x-lead","tab_id":"t2"}]}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	return log
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
		Team:       fixtureSeen(teamName),
		Definition: definition,
	}); err != nil {
		t.Fatal(err)
	}
}

// runWithStdin runs one command with the given stdin, as a hook would call it.
func runWithStdin(t *testing.T, stdin io.Reader, argv ...string) string {
	t.Helper()
	stdout, stderr, code := runStdin(t, stdin, argv...)
	if code != 0 {
		t.Fatalf("%v: exit %d\nstderr:\n%s", argv, code, stderr)
	}
	return stdout
}
func runExpectingError(t *testing.T, stdin io.Reader, argv ...string) (string, error) {
	t.Helper()
	stdout, stderr, code := runStdin(t, stdin, argv...)
	if code == 0 {
		return stdout, nil
	}
	return stdout, errors.New(stderr)
}
func launchWarning(out io.Writer, note string) {
	if note != "" {
		fmt.Fprintln(out, note)
	}
}
func runStdin(t *testing.T, stdin io.Reader, argv ...string) (string, string, int) {
	t.Helper()
	raw, err := io.ReadAll(stdin)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	was := os.Stdin
	os.Stdin = file
	defer func() { os.Stdin = was }()
	return run(argv...)
}
