package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/ops/internal/herdr"
	"github.com/aq2bq/hergy/internal/ops/internal/herdrcli"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
	"github.com/aq2bq/hergy/internal/ops/teststate"
)

func teamState(t *testing.T, name string) runningstate.State {
	t.Helper()
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	key, err := domainname.NewTeamName(name)
	if err != nil {
		t.Fatal(err)
	}
	state, exists, err := store.Read(key)
	if err != nil || !exists {
		t.Fatalf("state %s: exists=%t err=%v", name, exists, err)
	}
	return state
}

func noTeamState(t *testing.T, name string) {
	t.Helper()
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	key, err := domainname.NewTeamName(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.Read(key); err != nil || exists {
		t.Fatalf("state %s: exists=%t err=%v", name, exists, err)
	}
}

type stateBootPort struct {
	herdr.Port
	beforeTab func()
	failAt    string
	badRoot   bool
}

func (p *stateBootPort) JSON(args ...string) (map[string]any, error) {
	command := strings.Join(args[:2], " ")
	if command == "workspace get" {
		return map[string]any{"workspace": map[string]any{"workspace_id": "w1", "label": "workspace"}}, nil
	}
	if command == "tab create" && p.beforeTab != nil {
		p.beforeTab()
	}
	if command == p.failAt {
		return nil, fmt.Errorf("injected %s failure", command)
	}
	result, err := p.Port.JSON(args...)
	if command == "tab create" && p.badRoot {
		delete(result, "root_pane")
	}
	return result, err
}

func TestRunningStateExistsBeforeAliasTabAndKeepsStartupFacts(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	env := serveEnv(t)
	at := time.Date(2026, 11, 13, 1, 2, 3, 0, time.UTC)
	checked := false
	port := &stateBootPort{Port: herdrcli.New(os.Getenv("HERGY_HERDR")), beforeTab: func() {
		checked = true
		state := teamState(t, "fresh")
		launch := state.Launch()
		if state.Name().String() != "fresh" || launch.Definition.String() != "demo" || launch.Project != env.ProjectRoot ||
			launch.TeamYAML.String() != filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml") ||
			launch.Layer != "local" || launch.Workspace.ID != "w1" || launch.Workspace.Label != "workspace" || !launch.StartedAt.Equal(at) {
			t.Fatalf("startup facts before tab: name=%s launch=%+v", state.Name(), launch)
		}
		if len(state.Members()) != 0 || len(state.Workers()) != 0 {
			t.Fatal("state inherited an earlier run")
		}
	}}
	definition, _ := domainname.NewDefinitionName("demo")
	_, err := StartTeam(context.Background(), StartInput{Env: env, Definition: definition, Name: "fresh", Workspace: WorkspaceInput{Current: true},
		Port: port, Timeout: 1000, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("tab was not requested")
	}
	state := teamState(t, "fresh")
	launch := state.Launch()
	if launch.Executable == "" || launch.Lang == "" || len(launch.Workers) != 1 || launch.Workers[0].From != "a" ||
		launch.Workers[0].Purpose != "research" || launch.Workers[0].Profile != "claude-default" || launch.Workers[0].Start == "" || launch.Workers[0].Env == "" {
		t.Fatalf("startup prompt facts: %+v", launch)
	}
	if len(state.Members()) != 3 {
		t.Fatalf("receipts: %+v", state.Members())
	}
	for _, member := range state.Members() {
		if member.Name != "fresh-"+member.Short || member.Prompt == "" || member.Command != "up" || !member.Time.Equal(at) ||
			member.Start == "" || member.Env == "" || member.Executable != launch.Executable || member.Lang != launch.Lang || len(member.Docs) == 0 {
			t.Fatalf("receipt: %+v", member)
		}
		for _, doc := range member.Docs {
			if doc.Text != readFile(t, doc.Path) || doc.Layer == "" {
				t.Fatalf("document: %+v", doc)
			}
		}
	}
}

func TestRunningStateRetainedWithNoReceipts(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	script := readFile(t, os.Getenv("HERGY_HERDR"))
	script = strings.Replace(script, "case \"$g $s\" in\n", "case \"$g $s\" in\n  \"agent prompt\") echo '{\"error\":{\"code\":\"agent_prompt_stalled\"}}'; exit 1 ;;\n", 1)
	f.herdr(t, script)
	_, _, code := run("up", "demo")
	if code == 0 {
		t.Fatal("missing receipts succeeded")
	}
	if state := teamState(t, "demo"); len(state.Members()) != 0 {
		t.Fatalf("unconfirmed members: %+v", state.Members())
	}
}

func TestRunningStateStartFailureAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name, failAt     string
		cancel, retained bool
		badRoot          bool
	}{
		{name: "tab-failed", failAt: "tab create"},
		{name: "root-missing", badRoot: true, retained: true},
		{name: "pane-failed", failAt: "pane split", retained: true},
		{name: "cancel-before-tab", cancel: true},
		{name: "cancel-after-tab", cancel: true, retained: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			receiptTeam(t, f)
			env := serveEnv(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			port := &stateBootPort{Port: herdrcli.New(os.Getenv("HERGY_HERDR")), failAt: tc.failAt, badRoot: tc.badRoot}
			if tc.cancel && !tc.retained {
				port.beforeTab = cancel
				port.failAt = "tab create"
			}
			definition, _ := domainname.NewDefinitionName("demo")
			_, err := StartTeam(ctx, StartInput{Env: env, Definition: definition, Workspace: WorkspaceInput{Current: true}, Port: port, Timeout: 1000,
				Observe: func(event Event) {
					if tc.cancel && tc.retained && event.Progress != nil && event.Progress.CreatedTab != "" {
						cancel()
					}
				}})
			if err == nil {
				t.Fatal("expected failure")
			}
			if tc.retained {
				teamState(t, "demo")
			} else {
				noTeamState(t, "demo")
			}
		})
	}
}

func TestRunningStateUpdatesOnlyConfirmedMembers(t *testing.T) {
	for _, operation := range []string{"apply", "refresh", "reprompt"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			receiptTeam(t, f)
			mustRun(t, "up", "demo")
			before := teamState(t, "demo")
			role := filepath.Join(f.proj, ".hergy", "roles", "lead.md")
			writeFile(t, role, readFile(t, role)+"\nChanged state test rule.\n")
			receiptHerdr(t, f, "demo-b", "agent_prompt_stalled", false)
			args := []string{operation, "--all", "--team", "demo"}
			if operation == "apply" {
				args = []string{operation, "--team", "demo"}
			}
			_, stderr, code := run(args...)
			if code == 0 {
				t.Fatalf("missing receipt succeeded: %s", stderr)
			}
			after := teamState(t, "demo")
			if !reflect.DeepEqual(before.Launch(), after.Launch()) {
				t.Fatal("member update changed startup facts")
			}
			old, latest := before.Members(), after.Members()
			if len(latest) != len(old) {
				t.Fatalf("members: %+v", latest)
			}
			for i, member := range latest {
				switch {
				case member.Short == "b":
					if !reflect.DeepEqual(member, old[i]) {
						t.Fatalf("unreceived member changed: before=%+v after=%+v", old[i], member)
					}
				case member.Prompt == old[i].Prompt || !strings.Contains(member.Prompt, "Changed state test rule.") || member.Command != operation:
					t.Fatalf("confirmed body not updated: %+v", member)
				default:
					// The stored body must be the actual plan, without apply's reason preamble.
					plan, err := memberPlan(MemberInput{Env: serveEnv(t), Team: downInput("demo"), Member: member.Short}, "reprompt", true)
					if err != nil || member.Prompt != strings.ReplaceAll(plan.Members[0].Prompt, executable(), member.Executable) {
						t.Fatalf("body contains preamble: err=%v body=%s", err, member.Prompt)
					}
				}
			}
		})
	}
}

func TestRunningStateHireAndWorkerDownAndTeamDown(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	before := teamState(t, "demo")
	for _, name := range []string{"one", "two"} {
		mustRun(t, "hire", name, "--team", "demo", "--purpose", "research", "--task", "read docs", "--from", "a")
	}
	after := teamState(t, "demo")
	if !reflect.DeepEqual(before.Members(), after.Members()) || len(after.Workers()) != 2 {
		t.Fatalf("hire state: %+v", after.Workers())
	}
	for _, worker := range after.Workers() {
		if worker.Name != "demo-"+strings.TrimPrefix(worker.Tab, "worker:demo/") || worker.From != "a" || worker.Purpose != "research" || worker.Time.IsZero() {
			t.Fatalf("worker: %+v", worker)
		}
	}
	// This fixture must report the named panes and actually close worker tabs;
	// the basic startup fake deliberately reports no panes.
	script := readFile(t, os.Getenv("HERGY_HERDR"))
	script = strings.Replace(script, `  "pane list") echo '{"result":{"panes":[]}}' ;;`, `  "pane list")
    printf '{"result":{"panes":['
    first=1
    while read -r tab n; do
      [ -z "$n" ] && continue
      if [ "$first" = 1 ]; then first=0; else printf ','; fi
      printf '{"pane_id":"p-%s","label":"%s","tab_id":"%s"}' "$n" "$n" "$tab"
    done < "$state"
    printf ']}}' ;;
  "tab close")
    awk -v tab="$3" '$1 != tab' "$tabs" > "$tabs.tmp"; mv "$tabs.tmp" "$tabs"
    awk -v tab="$3" '$1 != tab' "$state" > "$state.tmp"; mv "$state.tmp" "$state"
    echo '{"result":{}}' ;;`, 1)
	f.herdr(t, script)
	mustRun(t, "down", "demo-one")
	after = teamState(t, "demo")
	if len(after.Workers()) != 1 || after.Workers()[0].Name != "demo-two" || !reflect.DeepEqual(before.Members(), after.Members()) {
		t.Fatalf("worker down: %+v", after.Workers())
	}
	mustRun(t, "down", "demo")
	noTeamState(t, "demo")
}

func TestRunningStateRepromptClock(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	_, err := Reprompt(context.Background(), MemberInput{Env: serveEnv(t), Team: downInput("demo"), Member: "a", Timeout: 1000, Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	member := teamState(t, "demo").Members()[0]
	if !member.Time.Equal(at) || member.Command != "reprompt" {
		t.Fatalf("receipt clock: %+v", member)
	}
}

func TestRunningStateFailedTabLeavesNoFile(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	script := readFile(t, os.Getenv("HERGY_HERDR"))
	script = strings.Replace(script, `  "tab create")`, `  "tab create") exit 1 ;;
  "unused create")`, 1)
	f.herdr(t, script)
	if _, _, code := run("up", "demo"); code == 0 {
		t.Fatal("tab failure succeeded")
	}
	path := filepath.Join(f.state, "hergy", "teams", "demo.json")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed tab left state: %v", err)
	}
}

func TestRunningStateUnavailableRefusesBeforeTab(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	writeFile(t, filepath.Join(f.state, "hergy", "teams"), "not a directory")
	_, stderr, code := run("up", "demo")
	if code == 0 || !strings.Contains(stderr, "cannot save the running team state") {
		t.Fatalf("startup did not refuse: %d %s", code, stderr)
	}
	if calls := readFile(t, filepath.Join(f.cfg, "herdr.log")); strings.Contains(calls, "tab create") {
		t.Fatal("refusal created tab")
	}
	_, records := launchLog(t, f)
	if len(records) != 0 {
		t.Fatal("refusal appended launch")
	}
}

func TestRunningStateUsesTeamStopCleanupBoundary(t *testing.T) {
	for _, outcome := range []string{"success", "timeout", "close", "read"} {
		t.Run(outcome, func(t *testing.T) {
			_, env, r, port, h := stopFixture(t)
			store, err := runningstate.Open()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := beginRunningState(store, env, r, runningstate.Workspace{ID: "w1", Label: "demo"}, h.stopNow); err != nil {
				t.Fatal(err)
			}
			port.failure = outcome
			if outcome == "timeout" {
				port.survive = 1000
			}
			_, err = h.Stop("demo")
			if (err == nil) != (outcome == "success") {
				t.Fatalf("stop: %v", err)
			}
			if outcome == "success" || outcome == "timeout" {
				noTeamState(t, "demo")
			} else {
				teamState(t, "demo")
			}
		})
	}
}

func TestRunningStateUnavailableHireKeepsExistingSingleWarning(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	if err := os.RemoveAll(filepath.Join(f.state, "hergy", "launches")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.state, "hergy", "launches"), "not a directory")
	_, stderr, code := run("hire", "one", "--team", "demo", "--purpose", "research", "--task", "read docs", "--from", "a")
	if code != 0 {
		t.Fatalf("state failure stopped hire: %s", stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "the launch was not recorded:") {
		t.Fatalf("duplicate warning: %q", stderr)
	}
}

func TestTeamCommandsResolveWithoutLaunchHistory(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo", "--as", "alpha")
	if err := os.RemoveAll(filepath.Join(f.state, "hergy", "launches")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"status", "alpha"}, {"tell", "a", "hello", "--team", "alpha"}, {"stop", "a", "--team", "alpha"},
		{"focus", "a", "--team", "alpha"}, {"nudge", "a", "continue", "--team", "alpha"}, {"down", "alpha"},
	} {
		mustRun(t, args...)
	}
	noTeamState(t, "alpha")
}

func TestRepromptKeepsTheGenerationItResolved(t *testing.T) {
	f := newFixture(t)
	receiptTeam(t, f)
	mustRun(t, "up", "demo")
	env := serveEnv(t)
	replaced := false
	port := &applyCallbackPort{Port: herdrcli.New(os.Getenv("HERGY_HERDR")), onPrompt: func() {
		if replaced {
			return
		}
		replaced = true
		store, err := runningstate.Open()
		if err != nil {
			t.Fatal(err)
		}
		state := teamState(t, "demo")
		if err := store.End(state.Name()); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Begin(state.Name(), state.Launch()); err != nil {
			t.Fatal(err)
		}
	}}
	_, err := Reprompt(context.Background(), MemberInput{Env: env, Team: downInput("demo"), Member: "a", Port: port, Timeout: 1000})
	if err != nil || !replaced {
		t.Fatalf("reprompt: replaced=%t err=%v", replaced, err)
	}
	if members := teamState(t, "demo").Members(); len(members) != 0 {
		t.Fatalf("old operation wrote to new run: %+v", members)
	}
}

func TestUnregisteredTabIsOnlyAStopCleanupTarget(t *testing.T) {
	f := newFixture(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"orphan"}`, "")
	path := os.Getenv("HERGY_HERDR")
	script := readFile(t, path)
	script = strings.Replace(script, `  *) echo '{"result":{}}' ;;`, `  "pane list") echo '{"result":{"panes":[{"pane_id":"p1","label":"orphan-lead","tab_id":"t1"},{"pane_id":"mine","label":"personal","tab_id":"t1"}]}}' ;;
  "agent get") echo '{"error":{"code":"agent_not_found"}}' ;;
  *) echo '{"result":{}}' ;;`, 1)
	log := filepath.Join(f.cfg, "herdr.log")
	script = strings.Replace(script, `g="$1"; s="$2"`, `printf '%s\n' "$*" >> '`+log+`'
g="$1"; s="$2"`, 1)
	f.herdr(t, script)
	if status := mustRun(t, "status"); !strings.Contains(status, "no teams are running") {
		t.Fatal(status)
	}
	if rows := newActivityRead(serveEnv(t)).Running().Teams; len(rows) != 0 {
		t.Fatalf("unregistered tab appears on screen: %+v", rows)
	}
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "outside")
	mustRun(t, "down", "orphan")
	calls := readFile(t, log)
	if !strings.Contains(calls, "pane close p1") || strings.Contains(calls, "pane close mine") {
		t.Fatalf("cleanup closed wrong panes: %s", calls)
	}
}

func TestRunningDefinitionUsesItsSavedTeamYAML(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	original := filepath.Join(f.cfg, "hergy", "teams", "original")
	if err := os.MkdirAll(filepath.Dir(original), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(f.proj, ".hergy", "teams", "demo"), original); err != nil {
		t.Fatal(err)
	}
	if err := teststate.Write(teststate.Team{Name: "alias", Definition: "original", Project: f.proj, Layer: "global", TeamYAML: filepath.Join(original, "team.yaml")}); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, `{"tab_id":"t1","label":"alias"}`, `{"name":"alias-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"}`)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stdout, stderr, code := run("status", "alias")
	if code != 0 || !strings.Contains(stdout, "alias-lead: idle") {
		t.Fatalf("saved definition was replaced by caller config: %d %s %s", code, stdout, stderr)
	}
}
