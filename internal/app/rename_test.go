package app

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
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
	f.runningHerdr(t, `{"tab_id":"t1","label":"team:foreign@demo"}`, "")
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	reader := newActivityRead(env)
	path := filepath.Join(f.cfg, "hergy", "teams", "demo", "team.yaml")
	// An unrecorded tab must not be interpreted as a stopped definition.
	if err := reader.CheckDefinitionRename(path); err == nil {
		t.Fatal("accepted an unknown running definition")
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(launchlog.Record{Team: "foreign", Definition: "demo", Project: filepath.Join(f.proj, "unlisted"), Layer: source.LayerGlobal, TeamYAML: path}); err != nil {
		t.Fatal(err)
	}
	if err := reader.CheckDefinitionRename(path); err == nil {
		t.Fatal("missed global use by an unlisted project")
	}
	// A same-named, independent project definition is not this global file.
	if err := reader.CheckDefinitionRename(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_HERDR", filepath.Join(f.cfg, "missing"))
	if err := reader.CheckDefinitionRename(path); err == nil {
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
					if err := log.Append(launchlog.Record{Team: "foreign", Definition: definition}); err != nil {
						t.Fatal(err)
					}
				}
				env, err := team.NewEnv()
				if err != nil {
					t.Fatal(err)
				}
				err = newActivityRead(env).CheckDefinitionRename(filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml"))
				if (err != nil) != (definition == "demo") {
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
	blocked := filepath.Join(f.cfg, "blocked-state")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", blocked)
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
			// An invalid workspace stops before booting any asynchronous launcher.
			_, err := newActivityRead(env).StartLaunch(web.HQLaunchIn{
				Project: env.ProjectRoot, Definition: "wave", Place: "project", Workspace: web.HQWorkspaceIn{ID: "missing"},
			})
			refusal, ok := errors.AsType[*web.HQError](err)
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
