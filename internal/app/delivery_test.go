package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
)

// TestDeliveryFindsTheStaleMembersOfTwoTeamsAndSendsToOne: the launch record is
// per running team, not per definition, so two teams booted from one definition
// are compared separately. A changed role rule makes both leads stale; a working
// lead is refused; telling one waiting lead moves its prompt, takes it off the
// list and leaves the other team's count.
func TestDeliveryWithoutALaunchRecordSaysNothingIsStale(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)
	a := newActivityRead(serveEnv(t))
	if got := a.Delivery().Count; got != 0 {
		t.Errorf("a running team with no launch record reads %d stale members, want 0", got)
	}
}

func TestDeliveryFindsTheStaleMembersOfTwoTeamsAndSendsToOne(t *testing.T) {
	useJapanese(t)
	f := newFixture(t)
	mustRun(t, "new", "demo")
	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	for _, team := range []string{"demo", "demo2"} {
		r, err := readDefinition(cat, env.ProjectRoot, "demo", team)
		if err != nil {
			t.Fatalf("read %s: %v", team, err)
		}
		if err := writeLaunch(log, env, r, launchlog.CommandUp, r.starts, launchlog.Workspace{}); err != nil {
			t.Fatalf("record %s: %v", team, err)
		}
	}

	// The role changes after both teams were handed their prompts.
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))

	// Two running teams from the same definition. demo2's lead is working, so
	// it cannot be selected; its prompt is still stale.
	fake := `#!/bin/sh
log='` + filepath.Join(f.cfg, "herdr.log") + `'
printf '%s\n' "$*" >> "$log"
g="$1"; s="$2"
if [ -z "$s" ]; then
  echo "herdr $g"
  for x in get prompt start list read focus send-keys current split rename close create process-info; do echo "  herdr $g $x"; done
  exit 0
fi
case "$g $s" in
  "tab list") echo '{"result":{"tabs":[{"tab_id":"t1","label":"demo"},{"tab_id":"t2","label":"demo2"}]}}' ;;
  "agent list") echo '{"result":{"agents":[
    {"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1},
    {"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t1","state_change_seq":2},
    {"name":"demo2-lead","agent":"claude","agent_status":"working","pane_id":"p3","tab_id":"t2","state_change_seq":3},
    {"name":"demo2-builder","agent":"claude","agent_status":"idle","pane_id":"p4","tab_id":"t2","state_change_seq":4}
  ]}}' ;;
  "agent get")
    case "$3" in
      demo-lead|p1) echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}}}' ;;
      demo2-builder|p4) echo '{"result":{"agent":{"name":"demo2-builder","agent":"claude","agent_status":"idle","pane_id":"p4","tab_id":"t2","state_change_seq":4}}}' ;;
      *) echo '{"error":{"code":"agent_not_found"}}'; exit 1 ;;
    esac ;;
  "agent prompt") echo '{"result":{}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`
	f.herdr(t, fake)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")

	a := newActivityRead(env)
	answer := a.Delivery()
	if answer.Count != 2 || len(answer.Teams) != 2 {
		t.Fatalf("delivery = %+v, want two stale members in two teams", answer)
	}
	for _, team := range answer.Teams {
		if len(team.Members) != 1 || team.Members[0].Short != "lead" {
			t.Errorf("team %s = %+v, want only its lead", team.Name, team)
			continue
		}
		if team.Members[0].Name != team.Name+"-lead" {
			t.Errorf("team %s names %q, want %s-lead", team.Name, team.Members[0].Name, team.Name)
		}
		if len(team.Members[0].Changes) == 0 {
			t.Errorf("team %s's lead has no summary", team.Name)
		}
		found := false
		for _, change := range team.Members[0].Changes {
			if change.Kind == "rule" && change.Rule == "L1" {
				found = true
			}
		}
		if !found {
			t.Errorf("team %s's summary = %+v, want rule L1", team.Name, team.Members[0].Changes)
		}
	}

	doc, ok := a.One("demo")
	if !ok {
		t.Fatal("demo is running")
	}
	for _, node := range doc.Nodes {
		if node.Name == "lead" && !node.Stale {
			t.Errorf("demo's lead is not marked 定義が変わった: %+v", doc.Nodes)
		}
	}

	// A working member is refused, and nothing is sent.
	promptCalls := func() int {
		n := 0
		for line := range strings.SplitSeq(readFile(t, filepath.Join(f.cfg, "herdr.log")), "\n") {
			if strings.HasPrefix(line, "agent prompt ") {
				n++
			}
		}
		return n
	}
	before := promptCalls()
	if _, err := a.Deliver([]string{"demo2-lead"}); err == nil || !strings.Contains(err.Error(), "working") {
		t.Fatalf("sending to a working lead = %v, want the working refusal", err)
	}
	if after := promptCalls(); after != before {
		t.Errorf("a refused send prompted %d times", after-before)
	}

	// A waiting lead is sent; its record becomes current, the other team stays.
	sent, err := a.Deliver([]string{"demo-lead"})
	if err != nil {
		t.Fatalf("send to demo-lead: %v", err)
	}
	if sent.Count != 1 {
		t.Errorf("after the send count = %d, want 1 (demo2's working lead)", sent.Count)
	}
	if len(sent.Sent) != 1 || sent.Sent[0] != "demo-lead" {
		t.Errorf("sent = %+v, want demo-lead", sent.Sent)
	}
	if after := promptCalls(); after != before+1 {
		t.Errorf("the send prompted %d times, want one", after-before)
	}
	for _, team := range a.Delivery().Teams {
		for _, member := range team.Members {
			if member.Name == "demo-lead" {
				t.Errorf("demo-lead is still stale after the send: %+v", member)
			}
		}
	}
	doc, _ = a.One("demo")
	for _, node := range doc.Nodes {
		if node.Name == "lead" && node.Stale {
			t.Errorf("demo's lead is still marked 定義が変わった: %+v", doc.Nodes)
		}
	}
}
