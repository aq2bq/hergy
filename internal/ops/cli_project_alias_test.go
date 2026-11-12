package ops

import (
	"strings"
	"testing"
	"time"
)

func TestProjectAliasStatus(t *testing.T) {
	for _, direction := range []struct {
		name  string
		alias bool
	}{{"real-record", false}, {"alias-record", true}} {
		t.Run(direction.name, func(t *testing.T) {
			_, readPath := aliasProjectRun(t, direction.alias)
			out, stderr, code := run("status", "-C", readPath)
			if code != 0 || !strings.Contains(out, "demo-hotfix") || strings.Contains(out, "not found") {
				t.Fatalf("status exit=%d\n%s\n%s", code, out, stderr)
			}
		})
	}
}
func TestProjectAliasApplyDry(t *testing.T) {
	for _, direction := range []struct {
		name  string
		alias bool
	}{{"real-record", false}, {"alias-record", true}} {
		t.Run(direction.name, func(t *testing.T) {
			_, readPath := aliasProjectRun(t, direction.alias)
			out, stderr, code := run("apply", "--dry", "--team", "demo-hotfix", "-C", readPath)
			if code != 0 || !strings.Contains(out, "there is no change to apply") {
				t.Fatalf("apply --dry exit=%d\n%s\n%s", code, out, stderr)
			}
		})
	}
}
func TestProjectAliasPromptRead(t *testing.T) {
	for _, direction := range []struct {
		name  string
		alias bool
	}{{"real-record", false}, {"alias-record", true}} {
		t.Run(direction.name, func(t *testing.T) {
			env, _ := aliasProjectRun(t, direction.alias)
			read := newActivityRead(env)
			read.now = func() time.Time { return activityNow }
			runs, err := read.PromptRuns(env.ProjectRoot, "demo", "project")
			if err != nil || len(runs) != 1 || runs[0].Team.String() != "demo-hotfix" || runs[0].Project != env.ProjectRoot {
				t.Fatalf("prompt runs=%+v err=%v", runs, err)
			}
			history, err := read.PromptHistory(env.ProjectRoot, "demo", "project", "lead", "demo-hotfix")
			if err != nil || history.Status != "recorded" || history.Recorded == nil || !strings.Contains(history.Recorded.Text, "demo-hotfix-lead") {
				t.Fatalf("prompt history=%+v err=%v", history, err)
			}
		})
	}
}
