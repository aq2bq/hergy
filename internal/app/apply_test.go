package app

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/web"
)

// TestApplySaysHowEachMemberIsPutRight: a running member whose prompt moved is
// sent its prompt again, a member the definition has and no agent is live for is
// started, and the current answer counts both for the band and the apply sheet.
func TestApplySaysHowEachMemberIsPutRight(t *testing.T) {
	useJapanese(t)
	f := newFixture(t)
	mustRun(t, "new", "demo")
	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	r, err := readDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.starts, launchlog.Workspace{}); err != nil {
		t.Fatalf("record demo: %v", err)
	}

	// The lead's role changes after the launch, and the builder has no live
	// agent: one is sent its prompt again, the other is started.
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))

	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

	a := newActivityRead(env)
	got := a.Apply()
	if got.Count != 2 || len(got.Teams) != 1 {
		t.Fatalf("apply = %+v, want the lead and the builder of one team", got)
	}
	if a.ApplyCount() != 2 {
		t.Errorf("apply count = %d, want 2", a.ApplyCount())
	}
	byShort := map[string]web.ApplyMember{}
	for _, member := range got.Teams[0].Members {
		byShort[member.Short] = member
	}
	lead, ok := byShort["lead"]
	if !ok {
		t.Fatalf("apply members = %+v, want the lead", got.Teams[0].Members)
	}
	if lead.How != web.ApplyResend || !lead.CanApply || lead.ConversationLost {
		t.Errorf("lead = %+v, want a resend that can be applied and keeps the conversation", lead)
	}
	if len(lead.Changes) == 0 {
		t.Errorf("the lead has no changes: %+v", lead)
	}
	builder, ok := byShort["builder"]
	if !ok {
		t.Fatalf("apply members = %+v, want the builder", got.Teams[0].Members)
	}
	if builder.How != web.ApplyStart || !builder.CanApply || builder.State != "absent" {
		t.Errorf("builder = %+v, want a start of a member that is not running", builder)
	}
	if got.Teams[0].Team != "demo" || got.Teams[0].Definition != "demo" {
		t.Errorf("team = %+v, want demo of the demo definition", got.Teams[0])
	}
}

// TestApplyLeavesAWorkingMemberAndSaysWhy: a member that is working is not
// applied now, and the row says so. The other member of the team is still put
// right: choosing the boundary is the human's (d-aacf).
func TestApplyLeavesAWorkingMemberAndSaysWhy(t *testing.T) {
	useJapanese(t)
	f := newFixture(t)
	mustRun(t, "new", "demo")
	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	r, err := readDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.starts, launchlog.Workspace{}); err != nil {
		t.Fatalf("record demo: %v", err)
	}
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))

	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

	a := newActivityRead(env)
	got := a.Apply()
	if got.Count != 2 {
		t.Fatalf("apply = %+v, want the lead and the builder counted", got)
	}
	for _, member := range got.Teams[0].Members {
		if member.Short != "lead" {
			continue
		}
		if member.How != web.ApplyCannot || member.CanApply || member.WhyNot == "" {
			t.Errorf("working lead = %+v, want cannot with a reason", member)
		}
	}
}

// TestApplyNamesAStartWithAnEmptyChangesList: a member hergy starts has no
// change of its own, and the promise says the member's changes are an array, so
// the answer carries [] and never null.
func TestApplyNamesAStartWithAnEmptyChangesList(t *testing.T) {
	f := applyDemo(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)
	got := newActivityRead(serveEnv(t)).Apply()
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, `"changes":null`) {
		t.Errorf("a member's changes are null: %s", text)
	}
	if !strings.Contains(text, `"changes":[]`) {
		t.Errorf("a start's changes are not an empty array: %s", text)
	}
}
