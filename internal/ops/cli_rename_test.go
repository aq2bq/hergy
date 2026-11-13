package ops

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"
	"github.com/aq2bq/hergy/internal/ops/internal/team"
	"github.com/aq2bq/hergy/internal/ops/teststate"
	"github.com/aq2bq/hergy/internal/source"
)

func TestUpHonorsRenameLock(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	release, err := source.LockTeams(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, stderr, code := run("up", "demo")
	if code == 0 || !strings.Contains(stderr, i18n.T("web.rename.busy")) {
		t.Fatalf("up ignored lock: %d %s", code, stderr)
	}
}
func TestRenameReadsUnlistedRunningProjects(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.runningHerdr(t, `{"tab_id":"t1","label":"foreign"}`, "")
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	reader := newActivityRead(env)
	path := filepath.Join(f.cfg, "hergy", "teams", "demo", "team.yaml")
	if err := reader.CheckDefinitionRename(projectPath(path)); err != nil {
		t.Fatal("unregistered tab blocked rename:", err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(launchlog.Record{Team: fixtureSeen("foreign"), Definition: "demo", Project: observedplace.Observe(filepath.Join(f.proj, "unlisted")), Layer: source.LayerGlobal, TeamYAML: observedplace.Observe(path)}); err != nil {
		t.Fatal(err)
	}
	if err := teststate.Write(teststate.Team{Name: "foreign", Definition: "demo", Project: filepath.Join(f.proj, "unlisted"), Layer: "global", TeamYAML: path}); err != nil {
		t.Fatal(err)
	}
	if err := reader.CheckDefinitionRename(projectPath(path)); err == nil {
		t.Fatal("missed global use by an unlisted project")
	}

	if err := reader.CheckDefinitionRename(projectPath(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"))); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "missing"))
	if err := reader.CheckDefinitionRename(projectPath(path)); err == nil {
		t.Fatal("accepted unavailable herdr")
	}
}
func TestRenameIgnoresUnrelatedUnknownDefinitions(t *testing.T) {
	for _, recorded := range []bool{false, true} {
		for _, definition := range []string{"demo", "other"} {
			t.Run(fmt.Sprintf("recorded=%t/%s", recorded, definition), func(t *testing.T) {
				f := newFixture(t)
				mustRun(t, "new", "demo")
				f.runningHerdr(t, `{"tab_id":"t1","label":"team:foreign@`+definition+`"}`, "")
				if recorded {
					log, err := launchlog.Open()
					if err != nil {
						t.Fatal(err)
					}
					if err := log.Append(launchlog.Record{Team: fixtureSeen("foreign"), Definition: definition}); err != nil {
						t.Fatal(err)
					}
				}
				env, err := team.NewEnv()
				if err != nil {
					t.Fatal(err)
				}
				err = newActivityRead(env).CheckDefinitionRename(projectPath(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")))
				if err != nil {
					t.Fatalf("rename refusal = %v, definition = %s", err, definition)
				}
			})
		}
	}
}
func TestUpContinuesWithoutRenameLockState(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	writeFile(t, filepath.Join(f.state, "hergy", "team-locks"), "not a directory")
	mustRun(t, "up", "demo")
}
func TestWebStartOnlyRefusesConflictingRenameLock(t *testing.T) {
	for _, state := range []string{"shared", "exclusive", "unavailable"} {
		t.Run(state, func(t *testing.T) {
			f, env, _ := hqFixture(t)
			mustRun(t, "new", "wave")
			path := filepath.Join(f.proj, ".hergy", "teams", "wave", "team.yaml")
			if state == "unavailable" {
				blocked := filepath.Join(f.cfg, "blocked-state")
				if err := os.WriteFile(blocked, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("XDG_STATE_HOME", blocked)
			} else {
				lock := source.LockTeamStart
				if state == "exclusive" {
					lock = func(path string) (func(), error) { return source.LockTeams(path) }
				}
				release, err := lock(path)
				if err != nil {
					t.Fatal(err)
				}
				defer release()
			}

			_, err := newActivityRead(env).StartLaunch(HQLaunchIn{
				Project: domainplace.Input(env.ProjectRoot.String()), Definition: "wave", Place: "project", Workspace: HQWorkspaceIn{ID: "missing"},
			})
			refusal, ok := errors.AsType[*HQError](err)
			want := http.StatusUnprocessableEntity
			if state == "exclusive" {
				want = http.StatusConflict
			}
			if !ok || refusal.Status != want {
				t.Fatalf("start returned %v, want status %d", err, want)
			}
		})
	}
}
