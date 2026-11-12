package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/ops"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
)

// aliasProjectRun keeps the definition name different from the running team's
// name, and reports the pane cwd with the opposite spelling to the reader.
func aliasProjectRun(t *testing.T, recordAlias bool) (team.Env, string) {
	t.Helper()
	f := applyDemo(t)
	alias := filepath.Join(filepath.Dir(f.proj), "project-alias")
	if err := os.Symlink(f.proj, alias); err != nil {
		t.Fatal(err)
	}
	recorded, readPath := f.proj, alias
	if recordAlias {
		recorded, readPath = alias, f.proj
	}
	env := serveEnv(t)
	r, err := ops.ReadDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "demo", "demo-hotfix")
	if err != nil {
		t.Fatal(err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := ops.WriteLaunch(log, env, r, launchlog.CommandUp, r.Starts, launchlog.Workspace{}); err != nil {
		t.Fatal(err)
	}
	records, err := log.Read()
	if err != nil {
		t.Fatal(err)
	}
	record := records[len(records)-1]
	// This is an older external record: its on-disk spelling is intentionally kept.
	record.Project = domainplace.Observe(recorded)
	record.TeamYAML = domainplace.Observe(filepath.Join(recorded, ".hergy", "teams", "demo", "team.yaml"))
	if err := log.Append(record); err != nil {
		t.Fatal(err)
	}
	f.runningHerdr(t, `{"tab_id":"t1","label":"demo-hotfix"}`,
		`{"name":"demo-hotfix-lead","agent":"claude","agent_status":"idle","pane_id":"p1","tab_id":"t1","state_change_seq":1},
   {"name":"demo-hotfix-builder","agent":"claude","agent_status":"idle","pane_id":"p2","tab_id":"t1","state_change_seq":1}`)
	script := readFile(t, os.Getenv("HERGY_HERDR"))
	panes := `  "pane list") echo '{"result":{"panes":[{"pane_id":"p1","label":"demo-hotfix-lead","tab_id":"t1","cwd":"` + readPath + `"}]}}' ;;` + "\n"
	f.herdr(t, strings.Replace(script, "case \"$g $s\" in\n", "case \"$g $s\" in\n"+panes, 1))
	projectEnv, err := env.WithProject(projectPath(readPath))
	if err != nil {
		t.Fatal(err)
	}
	return projectEnv, readPath
}

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
