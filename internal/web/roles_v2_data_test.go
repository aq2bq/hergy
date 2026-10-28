package web

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/rules"
)

// TestRolesDraftKeepsARuleOutOfThePrompt: the role screen offers no switch for
// in_prompt yet (d-f187), so the value travels in the draft: the island carries
// it, the document a save writes carries it back, and the file keeps the line
// (d-f187).
func TestRolesDraftKeepsARuleOutOfThePrompt(t *testing.T) {
	const text = `version: 1
sections:
  - heading: 進め方
    rules:
      - id: R1
        text: x
        in_prompt: false
      - id: R2
        text: y
`
	saved, err := rules.Parse(text, "roles/lead.yaml")
	if err != nil {
		t.Fatal(err)
	}
	island := r2DocOf(saved)
	if len(island.Sections) != 1 || len(island.Sections[0].Rules) != 2 {
		t.Fatalf("the island holds %+v", island)
	}
	if island.Sections[0].Rules[0].InPrompt {
		t.Error("the island says a rule kept out of the prompt is in it")
	}
	if !island.Sections[0].Rules[1].InPrompt {
		t.Error("the island says a rule in the prompt is out of it")
	}
	// The screen sends its draft back with each rule's place, the way the page
	// does, and edits one rule.
	for i := range island.Sections[0].Rules {
		island.Sections[0].Rules[i].From = &r2From{Section: 0, Rule: i}
	}
	island.Sections[0].Rules[1].Text = "書き換えた本文。"
	out, err := rules.Rewrite([]byte(text), saved, rules.Draft{Doc: island.document(), From: island.froms()})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "in_prompt: false") {
		t.Errorf("the save dropped in_prompt: false:\n%s", out)
	}
	back, err := rules.Parse(string(out), "roles/lead.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if back.Sections[0].Rules[0].Included() {
		t.Error("the saved rule came back into the prompt")
	}
	if back.Sections[0].Rules[1].Text != "書き換えた本文。" {
		t.Errorf("the save did not write the edit: %q", back.Sections[0].Rules[1].Text)
	}
}

// TestRoleCheckAnswersInTheBoundarysShape: a role check answers what the
// boundary's table says (spec-boundary 5): the problems, each standing on its
// own field of the document, the Markdown a person reads, how the change moved
// it, and the people the document reaches. A text is checked whole, so its
// problem stands on its line instead.
func TestRoleCheckAnswersInTheBoundarysShape(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)

	resp, decoded := f.apiJSON(t, server, http.MethodPost, "/api/role/check", map[string]any{
		"place": "project", "original": "lead",
		"doc": map[string]any{
			"name": "lead", "title": "進め方", "revision": 1,
			"sections": []any{map[string]any{
				"heading": "進め方", "level": 2, "intro": "", "rules": []any{
					map[string]any{"id": "LEAD-1", "text": "lead は最初に動く。", "in_prompt": true},
					map[string]any{"id": "", "text": "id の無い規則。", "in_prompt": true},
				},
			}},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the draft check answered %d: %v", resp.StatusCode, decoded)
	}
	problems, _ := decoded["problems"].([]any)
	if len(problems) == 0 {
		t.Fatalf("the draft check found nothing: %v", decoded)
	}
	onField := regexp.MustCompile(`^sections\.\d+\.rules\.\d+`)
	placed := false
	for _, one := range problems {
		problem, _ := one.(map[string]any)
		at, _ := problem["at"].(string)
		switch {
		case onField.MatchString(at):
			placed = true
		case at == "line":
		default:
			t.Errorf("a draft's problem stands nowhere: %v", problem)
		}
	}
	if !placed {
		t.Errorf("no draft problem stands on its field: %v", problems)
	}
	if _, ok := decoded["preview"]; !ok {
		t.Errorf("the check carries no Markdown preview: %v", decoded)
	}
	effect, _ := decoded["effect"].(map[string]any)
	if effect == nil {
		t.Fatalf("the check carries no effect: %v", decoded)
	}
	if _, ok := effect["diff"]; !ok {
		t.Errorf("the effect carries no diff: %v", effect)
	}
	if _, ok := effect["turns"]; !ok {
		t.Errorf("the effect carries no turns: %v", effect)
	}
	if readers, _ := decoded["readers"].([]any); len(readers) == 0 {
		t.Errorf("the check does not say who reads the document: %v", decoded)
	}

	resp, decoded = f.apiJSON(t, server, http.MethodPost, "/api/role/check", map[string]any{
		"place": "project", "name": "lead",
		"text": "version: 1\nsections:\n  - rules:\n      - id: LEAD-1\n        text: one\n      - id: LEAD-1\n        text: two\n",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the text check answered %d: %v", resp.StatusCode, decoded)
	}
	problems, _ = decoded["problems"].([]any)
	if len(problems) == 0 {
		t.Fatalf("the text check found nothing: %v", decoded)
	}
	onLine := false
	for _, one := range problems {
		problem, _ := one.(map[string]any)
		if at, _ := problem["at"].(string); at == "line" {
			if line, _ := problem["line"].(float64); line > 0 {
				onLine = true
			}
		}
	}
	if !onLine {
		t.Errorf("a text's problem stands on no line: %v", problems)
	}
}
