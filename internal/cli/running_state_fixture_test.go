package cli

import (
	"testing"

	"github.com/aq2bq/hergy/internal/ops/teststate"
)

func seedRunningTeam(t *testing.T, project, name, definition string) {
	t.Helper()
	if err := teststate.Write(teststate.Team{Name: name, Definition: definition, Project: project, WorkspaceID: "w1", WorkspaceLabel: "workspace"}); err != nil {
		t.Fatal(err)
	}
}
