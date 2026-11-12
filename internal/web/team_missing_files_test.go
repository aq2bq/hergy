package web

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestTeamsMissingDocumentIsOnlyAnUnreadableRow(t *testing.T) {
	f := newFixture(t)
	f.write(t, "teams/demo/notes.txt", "keep this folder\n")
	if err := os.Remove(f.teamYAML()); err != nil {
		t.Fatal(err)
	}
	server := f.server(t)
	resp, body := f.get(t, server, "/teams")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /teams answers %d: %s", resp.StatusCode, body)
	}
	list := teamsIsland(t, body).Team
	if len(list.Items) != 1 || list.Items[0].Name != "demo" || list.Items[0].Problems.Err == 0 {
		t.Fatalf("the missing document must leave one unreadable row: %+v", list.Items)
	}
	for _, file := range list.Files {
		if file.Kind == "team" {
			t.Errorf("a missing team.yaml must not be a YAML file: %+v", file)
		}
	}
	if len(list.Files) != 1 || list.Files[0].Kind != "agreement" || filepath.Base(list.Files[0].Path) != "agreement.md" {
		t.Errorf("only the existing agreement belongs in files: %+v", list.Files)
	}
}
