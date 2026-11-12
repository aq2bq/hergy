package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/launchlog"
	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"
	"github.com/aq2bq/hergy/internal/source"
)

// Decision 5: new display/records use one canonical definition path. Old lines
// keep their spelling and are compared through their canonical identity.
func TestCanonicalDefinitionPathKeepsOldRecordsAndNormalizesNewOnes(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	alias := filepath.Join(f.cfg, "settings-alias")
	if err := os.Symlink(filepath.Join(f.proj, ".hergy"), alias); err != nil {
		t.Fatal(err)
	}
	env := serveEnv(t)
	env.GlobalDir = alias
	env.LocalDir = filepath.Join(f.proj, "missing-layer")
	r, err := ReadDefinition(env.Catalog(source.Disk{}), env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(filepath.Join(alias, "teams", "demo", "team.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Team.Definition.TeamYAML.String() != canonical {
		t.Fatalf("definition path=%s", r.Team.Definition.TeamYAML)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	old := launchlog.Record{Command: launchlog.CommandUp, Team: fixtureSeen("demo"), Definition: "demo", Project: env.ProjectRoot.Seen(), TeamYAML: observedplace.Observe(filepath.Join(alias, "teams", "demo", "team.yaml"))}
	if err := log.Append(old); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(log.Dir(), "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := log.Latest(projectPath(canonical)); err != nil || !found {
		t.Fatalf("old alias record not found: %t %v", found, err)
	}
	if warning := recordLaunch(env, r, launchlog.CommandUp, r.Starts); warning != "" {
		t.Fatal(warning)
	}
	after, err := os.ReadFile(filepath.Join(log.Dir(), "log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatal("old record was rewritten")
	}
	records, err := log.Read()
	if err != nil || len(records) != 2 {
		t.Fatalf("records=%+v %v", records, err)
	}
	if records[0].TeamYAML.String() != old.TeamYAML.String() || records[1].TeamYAML.String() != canonical {
		t.Fatalf("record spellings: %+v", records)
	}
}
