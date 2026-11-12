package cli

import (
	"os/exec"
	"path/filepath"
	"testing"
)

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
