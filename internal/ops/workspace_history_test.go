package ops

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"
	"github.com/aq2bq/hergy/internal/source"
)

func TestWorkspaceHistorySurvivesLaterOperations(t *testing.T) {
	for _, command := range []string{launchlog.CommandRefresh, launchlog.CommandReprompt, launchlog.CommandApply} {
		t.Run(command, func(t *testing.T) {
			f, env, _ := hqFixture(t)
			log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
			path := projectPath(filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml"))
			if err := log.Append(launchlog.Record{Command: command, Team: fixtureSeen("core"), Project: env.ProjectRoot.Seen(), TeamYAML: observedplace.Observe(path.String())}); err != nil {
				t.Fatal(err)
			}
			h := newActivityRead(env).hq
			t.Run("project", func(t *testing.T) {
				got, found := h.latestProjectLaunch(env.ProjectRoot)
				if !found || got.WorkspaceID != "w1" {
					t.Fatalf("latest project launch = %+v found=%t; want workspace w1", got, found)
				}
			})
			t.Run("definition", func(t *testing.T) {
				got, found, err := latestLaunch(path)
				if err != nil || !found || got.WorkspaceID != "w1" {
					t.Fatalf("latest definition launch = %+v found=%t err=%v; want workspace w1", got, found, err)
				}
			})
			t.Run("start sheet", func(t *testing.T) {
				got := h.lastWorkspaceOf(herdrNow{}, path)
				if got == nil || got.ID != "w1" {
					t.Fatalf("last workspace = %+v; want w1", got)
				}
			})
		})
	}
}

func TestLegacyHireCannotBecomeProjectTeamOrWorkspaceHistory(t *testing.T) {
	f, env, _ := hqFixture(t)
	log := launchlog.At(filepath.Join(f.state, "hergy", "launches"))
	path := projectPath(filepath.Join(f.proj, ".hergy", "teams", "core", "team.yaml"))
	h := newActivityRead(env).hq
	before := launchProjects(h.launchRecords())
	for _, project := range []string{f.proj, t.TempDir()} {
		// These old entries carry real launch identity fields. Skipping empty team
		// or definition names is insufficient to keep them out of history.
		if err := log.Append(launchlog.Record{Command: "hire", Team: fixtureSeen("legacy-alias"), Project: observedplace.Observe(project), TeamYAML: path.Seen(), Workspace: "proj-dev", WorkspaceID: "w1"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := launchProjects(h.launchRecords()); !reflect.DeepEqual(got, before) {
		t.Fatalf("hire introduced a project: got=%v want=%v", got, before)
	}
	if got, found := h.latestProjectLaunch(env.ProjectRoot); !found || got.Team.String() != "core" {
		t.Fatalf("hire replaced project history: %+v %t", got, found)
	}
	if got := h.lastTeamsByWorkspace()["proj-dev"]; got != "core" {
		t.Fatalf("hire replaced workspace team: %q", got)
	}
	if got := h.lastWorkspaceOf(herdrNow{}, path); got == nil || got.ID != "w1" {
		t.Fatalf("hire replaced definition workspace: %+v", got)
	}
	r, err := ReadDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "core", "core")
	if err != nil {
		t.Fatal(err)
	}
	names := RecordedTeamNames(r.Team.Definition)
	if len(names) != 1 || names[0].String() != "core" {
		t.Fatalf("hire introduced an alias: %+v", names)
	}
}
