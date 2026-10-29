package web

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// TestAYAMLSaveGivesRuleIDsToEveryFile: a screen's save writes the text the
// person typed and then gives every rule without hergy's id one; the ids are
// written in the file that holds the rule and in the texts that name an old id,
// in every document (d-4aab).
func TestAYAMLSaveGivesRuleIDsToEveryFile(t *testing.T) {
	f := rbFixtureRoles(t)
	server := f.server(t)
	aPath := filepath.Join(f.proj, ".hergy", "roles", "ids-a.yaml")
	bPath := filepath.Join(f.proj, ".hergy", "roles", "ids-b.yaml")
	aText := "version: 1\nsections:\n  - rules:\n      - id: OLD1\n        text: see OLD1 as well.\n      - id: B2\n        text: another rule.\n"
	bText := "version: 1\nsections:\n  - rules:\n      - text: OLD1 matters here\n"
	f.write(t, filepath.Join("roles", "ids-a.yaml"), aText)
	f.write(t, filepath.Join("roles", "ids-b.yaml"), bText)

	resp, body := f.api(t, server, http.MethodPut, "/api/role/ids-a", map[string]any{
		"place": catalog.PlaceProject, "name": "ids-a", "hash": source.Hash(aText), "text": aText,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the YAML save answers %d: %s", resp.StatusCode, body)
	}
	aDoc, problems := rules.ReadAll(readFile(t, aPath), aPath)
	if len(problems) != 0 {
		t.Fatalf("the saved file does not read: %+v", problems)
	}
	newID := aDoc.Rules()[0].ID
	if !rules.RuleIDPattern.MatchString(newID) || newID == "OLD1" {
		t.Fatalf("the rule's id = %q, want an id hergy gave", newID)
	}
	if !strings.Contains(aDoc.Rules()[0].Text, newID) {
		t.Errorf("the rule's own text = %q, want %q", aDoc.Rules()[0].Text, newID)
	}
	if strings.Contains(aDoc.Rules()[0].Text, "OLD1") {
		t.Errorf("the old id stayed in the text: %q", aDoc.Rules()[0].Text)
	}
	bDoc, _ := rules.ReadAll(readFile(t, bPath), bPath)
	if !strings.Contains(bDoc.Rules()[0].Text, newID) {
		t.Errorf("the other document's text = %q, want the new id %q", bDoc.Rules()[0].Text, newID)
	}
}

// TestAPanelSaveAnswersTheIDsItGave: the answer a screen reads back holds the
// ids the rules got, so the screen shows them without making one (d-4aab).
func TestAPanelSaveAnswersTheIDsItGave(t *testing.T) {
	f := ruFixtureRoles(t)
	server := f.server(t)
	draft := r2NewRole("ids-new")
	draft.Sections[0].Rules[0].ID = ""
	draft.Sections[0].Rules[0].Text = "a rule the screen wrote."
	resp, decoded := f.apiJSON(t, server, http.MethodPut, "/api/role/ids-new", map[string]any{
		"place": catalog.PlaceProject, "hash": "", "doc": draft,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the save answers %d: %v", resp.StatusCode, decoded)
	}
	doc, ok := decoded["doc"].(map[string]any)
	if !ok {
		t.Fatalf("the answer carries no doc: %v", decoded)
	}
	sections, _ := doc["sections"].([]any)
	if len(sections) == 0 {
		t.Fatalf("the answer's doc holds no section: %v", doc)
	}
	first, _ := sections[0].(map[string]any)
	rows, _ := first["rules"].([]any)
	if len(rows) == 0 {
		t.Fatalf("the answer's doc holds no rule: %v", doc)
	}
	rule, _ := rows[0].(map[string]any)
	id, _ := rule["id"].(string)
	if !rules.RuleIDPattern.MatchString(id) {
		t.Errorf("the answer's rule id = %q, want the id hergy gave", id)
	}
}
