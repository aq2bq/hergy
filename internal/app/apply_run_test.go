package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/web"
)

// waitLaunch waits for one launch to end, as the screens' polling does, and
// answers its last state.
func waitLaunch(t *testing.T, read *activityRead, id string) web.HQLaunch {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		launch, ok := read.LaunchState(id)
		if !ok {
			t.Fatalf("launch %s is not there", id)
		}
		if launch.Phase != "starting" && launch.Phase != "awaiting" {
			return launch
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("launch %s did not end: %+v", id, launch)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestApplyMembersSendsAndStartsInOneOperation: a request names a member whose
// prompt moved and a member no agent is live for; hergy sends the first its
// prompt again (with the preamble) and starts the second in the team's tab, as
// one operation, and records what both were handed.
func TestApplyMembersSendsAndStartsInOneOperation(t *testing.T) {
	useJapanese(t)
	f := applyDemo(t)
	logPath := filepath.Join(f.cfg, "herdr.log")
	f.herdr(t, `#!/bin/sh
log='`+logPath+`'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[{"tab_id":"t1","label":"demo"}]}}' ;;
  "agent list") echo '{"result":{"agents":[
    {"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}
  ]}}' ;;
  "agent get")
    case "$3" in
      demo-lead|p1) echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}}}' ;;
      *) echo '{"error":{"code":"agent_not_found"}}' ;;
    esac ;;
  "pane list") echo '{"result":{"panes":[{"pane_id":"p1","label":"demo-lead","tab_id":"t1"}]}}' ;;
  "pane split") echo '{"result":{"pane":{"pane_id":"p2"}}}' ;;
  "pane process-info") echo '{"result":{"process_info":{"foreground_process_group_id":900,"shell_pid":900,"pane_id":"p1"}}}' ;;
  "agent prompt") echo '{"result":{}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))

	env := serveEnv(t)
	read := newActivityRead(env)
	launch, err := read.ApplyMembers([]string{"demo-lead", "demo-builder"})
	if err != nil {
		t.Fatalf("ApplyMembers: %v", err)
	}
	if launch.Action != web.ActionApply || launch.Team != "demo" || launch.Project != env.ProjectRoot {
		t.Fatalf("the answer = %+v, want an apply of demo in this project", launch)
	}
	done := waitLaunch(t, read, launch.ID)
	if done.Phase != "done" {
		t.Fatalf("the apply ended %q: %s\n%s", done.Phase, done.Error, readFile(t, logPath))
	}
	record := readFile(t, logPath)
	for _, want := range []string{"pane split", "agent start demo-builder", "agent prompt demo-builder", "agent prompt p1"} {
		if !strings.Contains(record, want) {
			t.Errorf("the apply did not run %q:\n%s", want, record)
		}
	}
	if !strings.Contains(record, i18n.T("apply.preamble_lead")) {
		t.Errorf("the resend has no preamble:\n%s", record)
	}
	_, records := launchLog(t, f)
	last := records[len(records)-1]
	if last.Command != launchlog.CommandApply || len(last.Members) != 2 {
		t.Fatalf("the last record = %+v, want an apply of the two members", last)
	}
}

// TestApplyMembersRefusesAWorkingMember: a member that is working is not applied
// now, and the refusal says why; nothing is applied for the request.
func TestApplyMembersRefusesAWorkingMember(t *testing.T) {
	useJapanese(t)
	f := applyDemo(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))
	env := serveEnv(t)
	read := newActivityRead(env)
	if _, err := read.ApplyMembers([]string{"demo-lead"}); err == nil {
		t.Fatal("ApplyMembers applied a working member")
	} else if !strings.Contains(err.Error(), i18n.Tf("apply.refused_state", "lead")) {
		t.Errorf("the refusal = %q, want it to name the reason", err)
	}
}
