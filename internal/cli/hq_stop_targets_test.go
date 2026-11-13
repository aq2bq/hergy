package cli

import (
	"os"
	"path/filepath"
	"testing" // stopHerdr is stateHerdr with the call log: a stop's panes are the
	// "pane close <id>" lines, and members/workers are the ones stopTargets names.
)

func (f *fixture) stopHerdr(t *testing.T, worker bool) string {
	seedRunningTeam(t, f.proj, "wave", "wave")
	t.Helper()
	dir := filepath.Join(f.cfg, "stop-herdr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(f.cfg, "herdr.log")
	tabs := `{"result":{"tabs":[{"tab_id":"t1","label":"wave","workspace_id":"w1"}`
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
d='`+
		dir+`'
log='`+
		log+`'
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
  "agent get") echo '{"error":{"code":"agent_not_found"}}' ;;
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
