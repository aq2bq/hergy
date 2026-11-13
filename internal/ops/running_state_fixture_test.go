package ops

import (
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/ops/internal/runningstate"

	"github.com/aq2bq/hergy/internal/ops/teststate"
)

func seedRunningTeam(t *testing.T, project, name, definition string) {
	t.Helper()
	if err := teststate.Write(teststate.Team{Name: name, Definition: definition, Project: project, WorkspaceID: "w1", WorkspaceLabel: "workspace"}); err != nil {
		t.Fatal(err)
	}
}

// startTestState prepares a completed startup without running a harness.
func startTestState(t *testing.T, env Environment, r *Resolved, members []definition.Start) {
	t.Helper()
	store, err := runningstate.Open()
	if err != nil {
		t.Fatal(err)
	}
	r.generation, err = beginRunningState(store, env, r, runningstate.Workspace{}, func() time.Time { return activityNow })
	if err != nil {
		t.Fatal(err)
	}
	if err := receiveRunningState(r, "up", members, func() time.Time { return activityNow }, func(string) bool { return true }, func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
}
