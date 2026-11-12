package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"

	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
)

func TestLastPromptRecordIsPerMemberAndExactTarget(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "team.yaml")
	base := launchlog.Record{Team: fixtureSeen("demo"), Definition: "core", Project: observedplace.Observe(project), TeamYAML: observedplace.Observe(path), Command: "up", Members: []launchlog.Member{{Name: fixtureSeen("demo-lead"), Prompt: "old-lead"}, {Name: fixtureSeen("demo-builder"), Prompt: "old-builder"}}}
	latest := base
	latest.Command = "reprompt"
	latest.Members = []launchlog.Member{{Name: fixtureSeen("demo-lead"), Prompt: "new-lead"}}
	foreign := latest
	foreign.Project = observedplace.Observe(t.TempDir())
	foreign.Members = []launchlog.Member{{Name: fixtureSeen("demo-lead"), Prompt: "foreign"}}
	wrongPath := latest
	wrongPath.TeamYAML = observedplace.Observe(filepath.Join(project, "other.yaml"))
	hire := latest
	hire.Command = "hire"
	records := []launchlog.Record{base, latest, foreign, wrongPath, hire}
	for member, want := range map[string]string{"lead": "new-lead", "builder": "old-builder"} {
		_, got, ok := lastPromptRecord(records, projectPath(project), projectPath(path), "core", "demo", member)
		if !ok || got.Prompt != want {
			t.Fatalf("%s: %+v %t", member, got, ok)
		}
	}
	legacy := base
	legacy.Project = observedplace.Observe("")
	if _, _, ok := lastPromptRecord([]launchlog.Record{legacy}, projectPath(project), projectPath(path), "core", "demo", "lead"); !ok {
		t.Fatal("exact legacy path lost")
	}
	legacy.TeamYAML = observedplace.Observe("")
	if _, _, ok := lastPromptRecord([]launchlog.Record{legacy}, projectPath(project), projectPath(path), "core", "demo", "lead"); ok {
		t.Fatal("ambiguous legacy record adopted")
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
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	body := "recorded\r\n\t  \nno final newline"
	hash, err := log.Put([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	record := launchlog.Record{Team: fixtureSeen("core"), Definition: "core", Project: env.ProjectRoot.Seen(), TeamYAML: observedplace.Observe(filepath.Join(env.ProjectRoot.String(), ".hergy", "teams", "core", "team.yaml")), Command: launchlog.CommandRefresh, Members: []launchlog.Member{{Name: fixtureSeen("core-lead"), Prompt: hash}}}
	if err := log.Append(record); err != nil {
		t.Fatal(err)
	}
	read := newActivityRead(env)
	read.now = func() time.Time { return activityNow }
	out, err := read.PromptHistory(env.ProjectRoot, "core", "project", "lead", "core")
	if err != nil || out.Recorded == nil || out.Recorded.Text != body || out.Recorded.Command != "refresh" {
		t.Fatalf("%+v %v", out, err)
	}
	record.Members[0].Prompt = "../log.jsonl"
	if err := log.Append(record); err != nil {
		t.Fatal(err)
	}
	if _, err = read.PromptHistory(env.ProjectRoot, "core", "project", "lead", "core"); err == nil {
		t.Fatal("invalid hash accepted")
	}
	record.Members[0].Prompt = hash
	if err := log.Append(record); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(log.Dir(), "blobs", hash)); err != nil {
		t.Fatal(err)
	}
	if _, err = read.PromptHistory(env.ProjectRoot, "core", "project", "lead", "core"); err == nil {
		t.Fatal("missing blob hidden")
	}
	if err := os.WriteFile(filepath.Join(log.Dir(), "log.jsonl"), []byte("broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = read.PromptHistory(env.ProjectRoot, "core", "project", "lead", "core"); err == nil {
		t.Fatal("broken record hidden")
	}
}
