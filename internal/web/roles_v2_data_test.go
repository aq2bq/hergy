package web

import (
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
	island := r2DocOf(saved, "hash", text)
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
