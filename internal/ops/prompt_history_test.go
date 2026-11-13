package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"
)

func TestLastPromptReceiptIsPerMemberAndExactTarget(t *testing.T) {
	f := newFixture(t)
	seedRunningTeam(t, f.proj, "demo", "core")
	store, _ := runningstate.Open()
	state := teamState(t, "demo")
	if err := store.Receive(state.Name(), state.Generation(), []runningstate.Receipt{{Name: "demo-lead", Short: "lead", Prompt: "old-lead"}, {Name: "demo-builder", Short: "builder", Prompt: "old-builder"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Resend(state.Name(), state.Generation(), []runningstate.Receipt{{Name: "demo-lead", Short: "lead", Prompt: "new-lead"}}); err != nil {
		t.Fatal(err)
	}
	state = teamState(t, "demo")
	path := state.Launch().TeamYAML
	for member, want := range map[string]string{"lead": "new-lead", "builder": "old-builder"} {
		got, ok := lastPromptReceipt(state, projectPath(f.proj), path, "core", member)
		if !ok || got.Prompt != want {
			t.Fatalf("%s: %+v %t", member, got, ok)
		}
	}
	if _, ok := lastPromptReceipt(state, projectPath(t.TempDir()), path, "core", "lead"); ok {
		t.Fatal("foreign project adopted")
	}
	if _, ok := lastPromptReceipt(state, projectPath(f.proj), projectPath(filepath.Join(f.proj, "other.yaml")), "core", "lead"); ok {
		t.Fatal("foreign path adopted")
	}
	if _, ok := lastPromptReceipt(state, projectPath(f.proj), path, "other", "lead"); ok {
		t.Fatal("foreign definition adopted")
	}
}
func TestPromptHistoryReadsLiveMembersOnly(t *testing.T) {
	_, env, _ := hqFixture(t)
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }
	runs, err := read.PromptRuns(env.ProjectRoot, "core", "project")
	if err != nil || len(runs) != 1 || runs[0].Team.String() != "core" {
		t.Fatalf("runs = %+v, %v", runs, err)
	}
	if _, err := read.PromptHistory(env.ProjectRoot, "core", "project", "lead", "core"); err != nil {
		t.Fatal(err)
	}
	absent, err := read.PromptHistory(env.ProjectRoot, "core", "project", "absent", "core")
	if err != nil || absent.Status != "stopped" || absent.Recorded != nil {
		t.Fatalf("%+v %v", absent, err)
	}
}
func TestPromptHistoryExactBodyAndReadFailures(t *testing.T) {
	_, env, _ := hqFixture(t)
	store, _ := runningstate.Open()
	state := teamState(t, "core")
	body := "recorded\r\n\t  \nno final newline"
	if err := store.Receive(state.Name(), state.Generation(), []runningstate.Receipt{{Name: "core-lead", Short: "lead", Prompt: body, Command: "refresh", Time: activityNow}}); err != nil {
		t.Fatal(err)
	}
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }
	out, err := read.PromptHistory(env.ProjectRoot, "core", "project", "lead", "core")
	if err != nil || out.Recorded == nil || out.Recorded.Text != body || out.Recorded.Command != "refresh" {
		t.Fatalf("%+v %v", out, err)
	}
	if err := os.WriteFile(store.Path(state.Name()), []byte("broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = read.PromptHistory(env.ProjectRoot, "core", "project", "lead", "core"); err == nil {
		t.Fatal("broken state hidden")
	}
}
