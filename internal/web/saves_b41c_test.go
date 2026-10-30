package web

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/source"
)

// TestROLES_B41C_TheAnswerIsTheFileTheWriteLeft: a save that gives a rule its id
// answers with the text the write left on the disk, so the screen's next save
// carries the file that is there now (r-b41c). The master's own steps are the
// test: add a rule, save, add another rule, save.
func TestROLES_B41C_TheAnswerIsTheFileTheWriteLeft(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	path := ruRolePath(f, catalog.PlaceProject, "builder")
	draft, text := ruLoadDraft(t, path)
	hash := source.Hash(text)
	for i, body := range []string{"足した規則 1。", "足した規則 2。", "足した規則 3。"} {
		draft.Sections[0].Rules = append(draft.Sections[0].Rules, r2Rule{Text: body})
		resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/builder", map[string]any{
			"place": catalog.PlaceProject, "original": "builder", "hash": hash, "doc": draft,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("save %d answers %d, want 200: %v", i+1, resp.StatusCode, decoded["text"])
		}
		disk := readFile(t, path)
		got, _ := decoded["text"].(string)
		if got != disk {
			t.Errorf("save %d answers a text the disk does not hold", i+1)
		}
		if want := source.Hash(disk); decoded["hash"] != want {
			t.Errorf("save %d answers the hash %v, want the one the file has now", i+1, decoded["hash"])
		}
		hash, _ = decoded["hash"].(string)
		// The draft stands on the file the save left, the way the screen's own
		// draft does (its toDoc takes the froms from the saved document).
		draft, _ = ruLoadDraft(t, path)
	}
	if !strings.Contains(readFile(t, path), "足した規則 3。") {
		t.Errorf("the third save did not write its rule")
	}
}

// TestTEAMS_B41C_TheAgreementAnswerIsTheFileTheWriteLeft: the same for a team's
// own agreement, whose save gives its rules their ids too (r-b41c).
func TestTEAMS_B41C_TheAgreementAnswerIsTheFileTheWriteLeft(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	before := readFile(t, path)
	hash := source.Hash(before)
	doc := map[string]any{"name": "", "title": "demo チームの約束", "revision": 1,
		"sections": []map[string]any{{"heading": "使命", "rules": []map[string]any{
			{"id": "", "text": "足した約束。", "in_prompt": true},
		}}}}
	for i := 1; i <= 3; i++ {
		resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/team/demo", map[string]any{
			"place": "project", "part": "agreement", "hash": hash,
			"doc": map[string]any{"ops": []map[string]any{}, "doc": doc},
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("save %d answers %d, want 200: %v", i, resp.StatusCode, decoded["text"])
		}
		disk := readFile(t, path)
		if got, _ := decoded["text"].(string); got != disk {
			t.Errorf("save %d answers a text the disk does not hold", i)
		}
		if want := source.Hash(disk); decoded["hash"] != want {
			t.Errorf("save %d answers the hash %v, want the one the file has now", i, decoded["hash"])
		}
		hash, _ = decoded["hash"].(string)
	}
}
