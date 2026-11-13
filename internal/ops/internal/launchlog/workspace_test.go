package launchlog

import (
	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"
	"testing"
)

func TestLatestWorkspaceSurvivesLaterOperations(t *testing.T) {
	for _, command := range []string{CommandRefresh, CommandReprompt, CommandApply} {
		t.Run(command, func(t *testing.T) {
			log := testLog(t)
			path := projectPath("/project/.hergy/teams/demo/team.yaml")
			up := Record{Command: CommandUp, TeamYAML: path.Seen(), Team: fixtureSeen("demo"), Definition: "demo", Workspace: "workspace", WorkspaceID: "w1"}
			if err := log.Append(up); err != nil {
				t.Fatal(err)
			}
			later := Record{Command: command, TeamYAML: observedplace.Observe(path.String()), Team: fixtureSeen("demo"), Definition: "demo"}
			if err := log.Append(later); err != nil {
				t.Fatal(err)
			}
			got, found, err := log.Latest(path)
			if err != nil || !found || got.Command != CommandUp || got.WorkspaceID != "w1" {
				t.Fatalf("Latest after %s = %+v found=%t err=%v; want workspace w1", command, got, found, err)
			}
		})
	}
}

func TestLatestRequiresAWorkspaceStart(t *testing.T) {
	path := projectPath("/project/.hergy/teams/demo/team.yaml")
	for _, later := range []Record{
		{Command: CommandUp},
		{Command: CommandUp, Workspace: "label-only"},
		{Command: CommandUp, WorkspaceID: "id-only"},
		{Command: CommandRefresh, Workspace: "not-a-start", WorkspaceID: "w2"},
	} {
		t.Run(later.Command+later.Workspace+later.WorkspaceID, func(t *testing.T) {
			log := testLog(t)
			later.TeamYAML = path.Seen()
			if err := log.Append(later); err != nil {
				t.Fatal(err)
			}
			if got, found, err := log.Latest(path); err != nil || found {
				t.Fatalf("non-start used as workspace history: %+v found=%t err=%v", got, found, err)
			}
			up := Record{Command: CommandUp, TeamYAML: path.Seen(), Workspace: "original", WorkspaceID: "w1"}
			if err := log.Append(up); err != nil {
				t.Fatal(err)
			}
			if err := log.Append(later); err != nil {
				t.Fatal(err)
			}
			if got, found, err := log.Latest(path); err != nil || !found || got.WorkspaceID != "w1" {
				t.Fatalf("non-start hid workspace: %+v found=%t err=%v", got, found, err)
			}
		})
	}
}
