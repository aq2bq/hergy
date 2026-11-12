package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

func TestResourcesRejectEmptyRoleSave(t *testing.T) {
	f := newFixture(t)
	status, body := put(t, f.server(t), "/api/role/new", `{"place":"project","doc":{"title":"","name":"","text":"# empty"}}`)
	if status != 422 {
		t.Fatalf("empty role saved: %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(f.proj, ".hergy/roles/.md")); !os.IsNotExist(err) {
		t.Fatalf("empty role file exists: %v", err)
	}
}

func TestResourcesDeleteAgreementWithoutTeam(t *testing.T) {
	f := newFixture(t)
	agree := filepath.Join(filepath.Dir(f.teamYAML()), "agreement.md")
	hash := source.Hash(readFile(t, agree))
	if err := os.Remove(f.teamYAML()); err != nil {
		t.Fatal(err)
	}
	resp, body := f.api(t, f.server(t), http.MethodDelete, "/api/team/demo?place=project&hash=&agreement_hash="+hash, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("agreement-only delete: %d %s", resp.StatusCode, body)
	}
	if _, err := os.Stat(agree); !os.IsNotExist(err) {
		t.Fatalf("agreement remains: %v", err)
	}
}

func TestResourcesCopyMissingAndUnreadableAgreement(t *testing.T) {
	for _, unreadable := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "unreadable"}[unreadable], func(t *testing.T) {
			f := newFixture(t)
			agree := filepath.Join(filepath.Dir(f.teamYAML()), "agreement.md")
			if err := os.Remove(agree); err != nil {
				t.Fatal(err)
			}
			if unreadable {
				if err := os.Mkdir(agree, 0755); err != nil {
					t.Fatal(err)
				}
			}
			status, body := put(t, f.server(t), "/api/team/copied", `{"place":"project","name":"copied","part":"folder","base":{"place":"project","name":"demo"},"doc":{"ops":[]}}`)
			if unreadable {
				if status != 404 && status != 422 {
					t.Fatalf("unreadable agreement copied: %d %s", status, body)
				}
				return
			}
			if status != 200 {
				t.Fatalf("copy: %d %s", status, body)
			}
			if _, err := os.Stat(filepath.Join(f.proj, ".hergy/teams/copied/agreement.md")); !os.IsNotExist(err) {
				t.Fatalf("missing agreement created: %v", err)
			}
		})
	}
}

func TestResourcesFolderSaveChecksNewStops(t *testing.T) {
	for _, tc := range []struct{ name, creation string }{
		{"copy", `"base":{"place":"project","name":"demo"}`},
		{"new", `"new":true`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			if tc.name == "new" {
				f.write(t, "profiles.yaml", readFile(t, filepath.Join(f.proj, ".hergy/profiles.yaml"))+"  claude-default:\n    kind: claude\n")
			}
			status, body := put(t, f.server(t), "/api/team/copied", `{"place":"project","name":"copied","part":"folder",`+tc.creation+`,"doc":{"ops":[{"kind":"memberProfile","member":"lead","profile":"absent"}]}}`)
			if status != 422 {
				t.Fatalf("invalid folder saved: %d %s", status, body)
			}
			if _, err := os.Stat(filepath.Join(f.proj, ".hergy/teams/copied")); !os.IsNotExist(err) {
				t.Fatalf("folder created: %v", err)
			}
		})
	}
}

func TestResourcesRenamePreflightsUnreadableReferences(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.proj, ".hergy/roles/builder.md")
	before := readFile(t, path)
	if err := os.Chmod(f.teamYAML(), 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.teamYAML(), 0644) })
	status, body := put(t, f.server(t), "/api/role/builder", `{"place":"project","original":"builder","rename_users":true,"hash":`+strconvQuote(source.Hash(before))+`,"doc":{"title":"","name":"maker","text":`+strconvQuote(before)+`}}`)
	if status != 422 {
		t.Fatalf("rename ignored unreadable definition: %d %s", status, body)
	}
	if !strings.Contains(body, "team.yaml") {
		t.Fatalf("missing failed reference: %s", body)
	}
	if got := readFile(t, path); got != before {
		t.Fatal("source changed")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "maker.md")); !os.IsNotExist(err) {
		t.Fatalf("target written: %v", err)
	}
}

func TestResourcesGlobalUsageAndRenameScanRegisteredProjects(t *testing.T) {
	c := newCopyFixture(t)
	writeUnder(t, c.global, "roles/lead.md", "# global lead\n")
	// The other project reads the global lead; the current project shadows it.
	if err := os.Remove(filepath.Join(c.other, ".hergy/roles/lead.md")); err != nil {
		t.Fatal(err)
	}
	s := c.twoProjects(t)
	_, users, found := s.roleTargets("global", "lead")
	if !found || users != 1 {
		t.Errorf("global role readers=%d found=%v, want 1", users, found)
	}
	before := readFile(t, filepath.Join(c.global, "roles/lead.md"))
	status, body := put(t, s, "/api/role/lead", `{"place":"global","original":"lead","rename_users":true,"hash":`+strconvQuote(source.Hash(before))+`,"doc":{"title":"","name":"coordinator","text":`+strconvQuote(before)+`}}`)
	if status != 200 {
		t.Fatalf("rename: %d %s", status, body)
	}
	text := readFile(t, filepath.Join(c.other, ".hergy/teams/other-dev/team.yaml"))
	if !strings.Contains(text, "role: coordinator") {
		t.Errorf("registered project reference unchanged: %s", text)
	}
	if !strings.Contains(readFile(t, c.teamYAML()), "role: lead") {
		t.Fatal("shadowed local role changed")
	}
}

func TestResourcesGlobalProfilesIncludeMembersAndWorkersAcrossProjects(t *testing.T) {
	c := newCopyFixture(t)
	global := filepath.Join(c.global, "profiles.yaml")
	writeUnder(t, c.global, "profiles.yaml", "version: 1\nprofiles:\n  shared:\n    kind: codex\n  kept:\n    kind: claude\n")
	otherTeam := filepath.Join(c.other, ".hergy/teams/other-dev/team.yaml")
	text := strings.ReplaceAll(readFile(t, otherTeam), "profile: planner", "profile: shared") + "workers:\n  - from: lead\n    purpose: build\n    profile: shared\n"
	writeUnder(t, c.other, ".hergy/teams/other-dev/team.yaml", text)
	s := c.twoProjects(t)
	_, users, found := s.profileTargets("global", "shared")
	if !found || users != 2 {
		t.Fatalf("global profile users=%d found=%v, want both member and worker", users, found)
	}
	before := readFile(t, global)
	status, body := put(t, s, "/api/profile/shared", `{"place":"global","original":"shared","rename_users":true,"hash":`+strconvQuote(source.Hash(before))+`,"doc":{"name":"renamed","kind":"codex","args":[],"env":[]}}`)
	if status != 200 {
		t.Fatalf("global profile rename: %d %s", status, body)
	}
	after := readFile(t, otherTeam)
	if strings.Count(after, "profile: renamed") != 2 || strings.Contains(after, "profile: shared") {
		t.Fatalf("member/worker references: %s", after)
	}
	if !strings.Contains(readFile(t, global), "kept:") {
		t.Fatal("renaming one profile discarded another profile in the same file")
	}
}

func TestResourcesProjectRenameLeavesSharedDefinitionsIntact(t *testing.T) {
	for _, kind := range []string{"role", "profile"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			globalTeam := filepath.Join(f.global, "teams/shared/team.yaml")
			writeUnder(t, f.global, "teams/shared/team.yaml", readFile(t, f.teamYAML()))
			beforeGlobal := readFile(t, globalTeam)
			var payload string
			if kind == "role" {
				text := readFile(t, filepath.Join(f.proj, ".hergy/roles/builder.md"))
				payload = `{"place":"project","original":"builder","rename_users":true,"hash":` + strconvQuote(source.Hash(text)) + `,"doc":{"name":"maker","title":"","text":` + strconvQuote(text) + `}}`
			} else {
				text := readFile(t, filepath.Join(f.proj, ".hergy/profiles.yaml"))
				payload = `{"place":"project","original":"coder","rename_users":true,"hash":` + strconvQuote(source.Hash(text)) + `,"doc":{"name":"renamed","kind":"claude","args":["--x"],"env":[{"key":"A","value":"one"}]}}`
			}
			status, body := put(t, f.server(t), "/api/"+kind+"/original", payload)
			if status != 200 {
				t.Fatalf("project rename: %d %s", status, body)
			}
			if got := readFile(t, globalTeam); got != beforeGlobal {
				t.Fatalf("project rename changed a shared definition: %s", got)
			}
			if readFile(t, f.teamYAML()) == beforeGlobal {
				t.Fatal("project definition was not updated")
			}
		})
	}
}

// Saving profiles has always rejected every stopping problem in the resulting
// file, including one already in its baseline. The shared sequence preserves it.
func TestResourcesProfilesPreserveExistingRefusal(t *testing.T) {
	f := newFixture(t)
	text := "version: 1\nprofiles:\n  Bad Name:\n    kind: codex\n"
	f.write(t, "profiles.yaml", text)
	status, body := put(t, f.server(t), "/api/profile/yaml", `{"place":"project","text":`+strconvQuote(text)+`,"hash":`+strconvQuote(source.Hash(text))+`}`)
	if status != 422 {
		t.Fatalf("profile file with existing stopping problem saved: %d %s", status, body)
	}
}

func TestResourcesTeamYAMLSaveKeepsErrorText(t *testing.T) {
	f := newFixture(t)
	before := readFile(t, f.teamYAML())
	text := strings.Replace(before, "profile: coder", "profile: absent", 1)
	status, body := put(t, f.server(t), "/api/team/demo", `{"place":"project","part":"team","hash":`+strconvQuote(source.Hash(before))+`,"text":`+strconvQuote(text)+`}`)
	if status != 422 {
		t.Fatalf("invalid YAML saved: %d %s", status, body)
	}
	var refusal struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(body), &refusal); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(refusal.Text, "members[0].profile:") {
		t.Fatalf("YAML refusal changed its existing text: %s", refusal.Text)
	}
}
