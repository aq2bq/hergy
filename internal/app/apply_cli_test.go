package app

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestApplyAddsAMemberTheTeamHasNoAgentFor: hergy apply is the road that adds a
// member to a running team now (d-8fcf): the member gets a pane in the team's tab
// and its bootstrap prompt.
func TestApplyAddsAMemberTheTeamHasNoAgentFor(t *testing.T) {
	f := applyDemo(t)
	log := f.addingHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "w0:p1")
	out := mustRun(t, "apply", "builder")
	if !strings.Contains(out, "builder") {
		t.Fatalf("apply does not report the member it applied:\n%s", out)
	}
	if strings.Contains(out, "next:") {
		t.Fatalf("apply names a next command it already did:\n%s", out)
	}
	record := readFile(t, log)
	for _, want := range []string{"pane split", "pane rename", "agent start demo-builder", "agent prompt demo-builder"} {
		if !strings.Contains(record, want) {
			t.Fatalf("apply did not run %q:\n%s", want, record)
		}
	}
	_, records := launchLog(t, f)
	if last := records[len(records)-1]; last.Command != "apply" || len(last.Members) != 1 || last.Members[0].Short != "builder" {
		t.Fatalf("the last record = %+v, want an apply of builder", last)
	}
}

// TestApplyDrySaysHowAndWhatChanged: --dry shows each member's way of applying
// and what changed, and starts nothing.
func TestApplyDrySaysHowAndWhatChanged(t *testing.T) {
	f := applyDemo(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))

	out := mustRun(t, "apply", "--dry")
	for _, want := range []string{"Plan:", "lead", "sent the prompt again", "MAAF"} {
		if !strings.Contains(out, want) {
			t.Errorf("apply --dry does not say %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "applied ") {
		t.Errorf("apply --dry applied something:\n%s", out)
	}
}

// TestApplyDrySaysWhyADefinitionDoesNotBuild: a team whose role file does not
// read cannot be applied, and --dry says the reason and the way to fix it, and
// answers 1 (2.2, d-36ef).
func TestApplyDrySaysWhyADefinitionDoesNotBuild(t *testing.T) {
	f := applyDemo(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)
	writeFile(t, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"), "version: 1\nsections: [\n")

	stdout, stderr, code := run("apply", "--dry")
	if code == 0 {
		t.Fatalf("apply --dry answered 0 for a definition that does not build:\n%s", stdout)
	}
	for _, want := range []string{"does not build", "hergy validate"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("apply --dry does not say %q:\n%s", want, stdout)
		}
	}
	if stderr == "" {
		t.Error("apply --dry did not say why it answered 1")
	}
}

// TestApplyLeavesAWorkingMemberAndExitsOne: a member that is working is left,
// with the reason, and the command answers 1 because one member stayed.
func TestApplyLeavesAWorkingMemberAndExitsOne(t *testing.T) {
	f := applyDemo(t)
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo"}`,
		`{"name":"demo-lead","agent":"claude","agent_status":"working","pane_id":"p1","tab_id":"t1","state_change_seq":1}`)
	rolePath := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
	role := readFile(t, rolePath)
	writeFile(t, rolePath, strings.Replace(role, "Hand over a task the receiver can start without reading your conversation.", "Hand over a task the receiver can start without reading your conversation, and name the files to read.", 1))

	stdout, stderr, code := run("apply", "lead")
	if code == 0 {
		t.Fatalf("apply answered 0 while a member stayed:\n%s", stderr)
	}
	if !strings.Contains(stdout, "lead") {
		t.Errorf("the answer does not name the member left:\n%s", stdout)
	}
}
