package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/web"
)

// applyDemo is a demo team launched once, with its launch record written: the
// starting point of the tests that change the definition after the launch.
func applyDemo(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	mustRun(t, "new", "demo")
	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	r, err := readDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.starts, launchlog.Workspace{}); err != nil {
		t.Fatalf("record demo: %v", err)
	}
	return f
}

// TestApplyRestartsAMemberWhoseProfileChanged: a changed profile is not a
// changed prompt, so the launch record's fingerprint is what tells hergy to
// restart the member. The row says the conversation goes, and applying it keeps
// the member available.
func TestApplyRestartsAMemberWhoseProfileChanged(t *testing.T) {
	useJapanese(t)
	f := applyDemo(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    args: [--model, opus]\n")
	env := serveEnv(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

	got := newActivityRead(env).Apply()
	if len(got.Teams) != 1 {
		t.Fatalf("apply = %+v, want the demo team", got)
	}
	var lead web.ApplyMember
	for _, member := range got.Teams[0].Members {
		if member.Short == "lead" {
			lead = member
		}
	}
	if lead.How != web.ApplyRestart || !lead.CanApply || !lead.ConversationLost {
		t.Fatalf("lead = %+v, want a restart that can be applied and loses the conversation", lead)
	}
	found := false
	for _, change := range lead.Changes {
		if change.Kind == "start" {
			found = true
		}
	}
	if !found {
		t.Errorf("changes = %+v, want the start named", lead.Changes)
	}
}

// TestApplyRefusesAChangedEnvAndNamesDownUp: env is fixed when the pane is made,
// so hergy cannot apply it and says so instead of restarting the agent.
func TestApplyRefusesAChangedEnvAndNamesDownUp(t *testing.T) {
	useJapanese(t)
	f := applyDemo(t)
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    env:\n      DEMO_MODE: on\n")
	env := serveEnv(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

	got := newActivityRead(env).Apply()
	for _, member := range got.Teams[0].Members {
		if member.Short != "lead" {
			continue
		}
		if member.How != web.ApplyCannot || member.CanApply || member.WhyNot == "" {
			t.Fatalf("lead = %+v, want cannot with a reason", member)
		}
		if member.ConversationLost {
			t.Errorf("lead = %+v, want no conversation lost by a refusal", member)
		}
	}
}

// TestRepromptHeadsThePromptWithWhatChanged: a reprompt after a change sends the
// preamble and the current prompt, and the record keeps the plain prompt, so the
// next comparison does not see the preamble as a change.
func TestRepromptHeadsThePromptWithWhatChanged(t *testing.T) {
	useJapanese(t)
	f := applyDemo(t)
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))

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
    {"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1},
    {"name":"demo-builder","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t1","state_change_seq":2}
  ]}}' ;;
  "agent get") echo '{"result":{"agent":{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}}}' ;;
  "agent prompt") echo '{"result":{}}' ;;
  "pane current") echo '{"result":{"pane":{"workspace_id":"w1"}}}' ;;
  *) echo '{"result":{}}' ;;
esac
`)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "reprompt", "lead", "--team", "demo")

	calls := readFile(t, logPath)
	if !strings.Contains(calls, i18n.T("apply.preamble_lead")) {
		t.Fatalf("the reprompt did not head the prompt with what changed:\n%s", calls)
	}
	launches, records := launchLog(t, f)
	last := records[len(records)-1]
	text, err := launches.Text(last.Members[0].Prompt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(text), i18n.T("apply.preamble_lead")) {
		t.Errorf("the record kept the preamble as the member's prompt: %q", string(text))
	}
}

// TestApplyNotesAChangedWorkerProfile: a worker's profile reaches the next worker
// hired, not the live ones, so the change is the team's note and not a member to
// apply (d-36ef).
func TestApplyNotesAChangedWorkerProfile(t *testing.T) {
	useJapanese(t)
	f := applyDemo(t)
	teamPath := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	writeFile(t, teamPath, readFile(t, teamPath)+"workers:\n  - from: lead\n    purpose: coder\n    profile: claude-default\n")
	// The launch is recorded again with the worker entry, as a real up would.
	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	r, err := readDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunch(log, env, r, launchlog.CommandUp, r.starts, launchlog.Workspace{}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.proj, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n    args: [--model, opus]\n")
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)

	got := newActivityRead(serveEnv(t)).Apply()
	if len(got.Teams) != 1 {
		t.Fatalf("apply = %+v, want the demo team", got)
	}
	joined := strings.Join(got.Teams[0].Notes, "\n")
	if !strings.Contains(joined, "coder") {
		t.Errorf("notes = %q, want the changed worker named", joined)
	}
}
