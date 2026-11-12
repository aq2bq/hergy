package app

import (
	"testing"

	"github.com/aq2bq/hergy/internal/launchlog"
	"github.com/aq2bq/hergy/internal/ops" // applyDemo is a demo team launched once, with its launch record written: the

	// starting point of the tests that change the definition after the launch.
	"github.com/aq2bq/hergy/internal/source"
)

func applyDemo(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	mustRun(t, "new", "demo")
	env := serveEnv(t)
	cat := env.Catalog(source.Disk{})
	r, err := ops.ReadDefinition(cat, env.ProjectRoot, "demo", "demo")
	if err != nil {
		t.Fatal(err)
	}
	log, err := launchlog.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := ops.WriteLaunch(log, env, r, launchlog.CommandUp, r.Starts, launchlog.Workspace{}); err != nil {
		t.Fatalf("record demo: %v", err)
	}
	return f
}
