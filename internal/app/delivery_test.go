package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
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

// TestDeliveryCountsTheRunningMembersAfterAMemberIsAdded: a member added to a
// running team's definition changes the roster the members already running were
// handed, so they are the ones 伝えていない after the new member is raised. The
// raise's own record names only the raised member, so it does not clear them
// (r-5d1a, ac-d090).
func TestDeliveryCountsTheRunningMembersAfterAMemberIsAdded(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	// The two members were handed their prompts at the boot.
	r, err := readDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.starts, launchlog.Workspace{}); err != nil {
		t.Fatal(err)
	}

	// The definition gains a third member, as the screen writes it.
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"),
		"version: 1\nmission: what this team is for, in one line\nmembers:\n"+
			"  - name: lead\n    role: lead\n    profile: claude-default\n"+
			"  - name: builder\n    role: builder\n    profile: claude-default\n"+
			"  - name: spec2\n    role: builder\n    profile: claude-default\n"+
			"turns:\n  - human -> lead\n  - lead -> builder\n  - builder -> lead\n  - lead -> human\n  - spec2 -> lead\n")
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1},`+
			`{"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t1","state_change_seq":2}`)

	// The raise records the member it started, and nothing else.
	raised, err := readDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	spec2, err := raised.member("spec2")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, raised, launchlog.CommandRefresh, []definition.Start{*spec2}, launchlog.Workspace{}); err != nil {
		t.Fatal(err)
	}

	a := newActivityRead(env)
	answer := a.Delivery()
	if answer.Count != 2 || len(answer.Teams) != 1 {
		t.Fatalf("delivery = %+v, want the two members already running", answer)
	}
	got := []string{}
	for _, member := range answer.Teams[0].Members {
		got = append(got, member.Short)
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"builder", "lead"}) {
		t.Errorf("the members counted as 伝えていない are %v, want lead and builder", got)
	}
	for _, member := range answer.Teams[0].Members {
		if member.Short == "spec2" {
			t.Errorf("the member just raised is counted as 伝えていない: %+v", member)
		}
	}
}

// TestDeliveryCountsATeamOfAnotherListedProject: the headquarters shows several
// projects, and a team running in one of them is counted against that project's
// own launch record — not the record of the project serve was started in. A role
// changed there makes that team's members 伝えていない, and a send reaches them
// (r-2e49, ac-9cab, n-a1a4).
func TestDeliveryCountsATeamOfAnotherListedProject(t *testing.T) {
	useJapanese(t)
	f, env, other := hqFixture(t)
	otherEnv, err := env.AtProject(other)
	if err != nil {
		t.Fatal(err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	// The other project's team was handed its prompts at its boot.
	cat := otherEnv.Catalog(source.Disk{})
	r, err := readDefinition(cat, other, "ledger", "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, otherEnv, r, launchlog.CommandUp, r.starts, launchlog.Workspace{}); err != nil {
		t.Fatal(err)
	}

	// A rule of the role the team's lead plays changes after the launch.
	rolePath := filepath.Join(other, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))

	// The serve was started in this fixture's project, and the other one is listed
	// beside it: the team is the other project's.
	if !samePath(other, otherEnv.ProjectRoot) || samePath(other, env.ProjectRoot) {
		t.Fatalf("the fixture's projects are %q and %q", env.ProjectRoot, other)
	}
	a := newActivityRead(env)
	answer := a.Delivery()
	if answer.Count != 1 || len(answer.Teams) != 1 {
		t.Fatalf("delivery = %+v, want the other project's lead", answer)
	}
	if answer.Teams[0].Name != "ledger" || len(answer.Teams[0].Members) != 1 || answer.Teams[0].Members[0].Name != "ledger-lead" {
		t.Fatalf("delivery names %+v, want ledger's lead", answer.Teams)
	}

	// The send reaches it, in that project, and takes it off the count.
	sent, err := a.Deliver([]string{"ledger-lead"})
	if err != nil {
		t.Fatalf("send to ledger-lead: %v", err)
	}
	if len(sent.Sent) != 1 || sent.Sent[0] != "ledger-lead" {
		t.Errorf("sent = %+v, want ledger-lead", sent.Sent)
	}
	if after := a.Delivery().Count; after != 0 {
		t.Errorf("the lead is still 伝えていない after the send: %d", after)
	}
	// The record the send wrote belongs to the team's own project.
	_, records := launchLog(t, f)
	last := records[len(records)-1]
	if last.Command != launchlog.CommandReprompt || !samePath(last.Project, other) {
		t.Errorf("the send recorded %q in %q, want a reprompt of the other project", last.Command, last.Project)
	}
}
