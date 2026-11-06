package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/team"
)

// stopHerdr is stateHerdr with the call log: a stop's panes are the
// "pane close <id>" lines, and members/workers are the ones stopTargets names.
func (f *fixture) stopHerdr(t *testing.T, worker bool) string {
	t.Helper()
	dir := filepath.Join(f.cfg, "stop-herdr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(f.cfg, "herdr.log")
	tabs := `{"result":{"tabs":[{"tab_id":"t1","label":"wave","workspace_id":"w1"}` // + worker tab below
	if worker {
		tabs += `,{"tab_id":"tw","label":"worker:wave/deps","workspace_id":"w1"}`
	}
	tabs += `]}}`
	panes := `{"result":{"panes":[{"pane_id":"p1","label":"wave-lead","tab_id":"t1","workspace_id":"w1"},{"pane_id":"p2","label":"wave-builder","tab_id":"t1","workspace_id":"w1"}`
	if worker {
		panes += `,{"pane_id":"p3","label":"wave-deps","tab_id":"tw","workspace_id":"w1"}`
	}
	panes += `]}}`
	agents := `{"result":{"agents":[{"name":"wave-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1"},{"name":"wave-builder","agent":"codex","agent_status":"working","pane_id":"p2","tab_id":"t1"}`
	if worker {
		agents += `,{"name":"wave-deps","agent":"codex","agent_status":"working","pane_id":"p3","tab_id":"tw"}`
	}
	agents += `]}}`
	for name, text := range map[string]string{"tabs.json": tabs, "panes.json": panes, "agents.json": agents} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	spaces := `{"result":{"workspaces":[{"workspace_id":"w1","label":"wave"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(spaces), 0o644); err != nil {
		t.Fatal(err)
	}
	f.herdr(t, `#!/bin/sh
d='`+dir+`'
log='`+log+`'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") cat "$d/tabs.json" ;;
  "pane list") cat "$d/panes.json" ;;
  "agent list") cat "$d/agents.json" ;;
  "workspace list") cat "$d/workspaces.json" ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  "pane close")
    printf '{"result":{"tabs":[]}}' > "$d/tabs.json"
    printf '{"result":{"panes":[]}}' > "$d/panes.json"
    printf '{"result":{"agents":[]}}' > "$d/agents.json"
    echo '{"result":{}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	return log
}

// TestStopClosesWhatStopTargetsNames: the panes Stop closes, the members and
// workers stopTargets answers with, and the Closed it reports are one answer
// (r-ccf8, ac-ddb6), with and without a worker.
func TestStopClosesWhatStopTargetsNames(t *testing.T) {
	for _, worker := range []bool{false, true} {
		t.Run(map[bool]string{false: "no worker", true: "a worker"}[worker], func(t *testing.T) {
			f := newFixture(t)
			mustRun(t, "new", "wave")
			log := f.stopHerdr(t, worker)
			env, err := team.NewEnv()
			if err != nil {
				t.Fatal(err)
			}
			read := newActivityRead(env)
			read.now = func() time.Time { return activityNow }
			running := read.hq.act.readHerdr(read.hq.act.clock())
			one, found := read.hq.findRunning(running, "wave")
			if !found {
				t.Fatalf("the fixture's team is not running:\n%s", calls(t, log))
			}
			penv, err := read.hq.act.envAt(one.Project)
			if err != nil {
				t.Fatal(err)
			}
			target, err := read.hq.stopTargets(penv, one, running)
			if err != nil {
				t.Fatal(err)
			}
			closed, err := read.Stop("wave")
			if err != nil {
				t.Fatal(err)
			}
			if closed.Closed.Members != len(target.members) || closed.Closed.Workers != len(target.workers) {
				t.Errorf("Closed = %d members, %d workers; stopTargets = %d, %d", closed.Closed.Members, closed.Closed.Workers, len(target.members), len(target.workers))
			}
			closes := calls(t, log)
			if !strings.Contains(closes, "pane close p1") || !strings.Contains(closes, "pane close p2") {
				t.Errorf("Stop closed no member panes:\n%s", closes)
			}
			if worker && !strings.Contains(closes, "pane close p3") {
				t.Errorf("Stop closed no worker pane:\n%s", closes)
			}
		})
	}
}
