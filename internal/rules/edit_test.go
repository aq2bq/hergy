package rules

import (
	"strings"
	"testing"
)

// twoSections is a document with two sections: two rules in the first and one
// in the second, so an edit can be seen to touch only what it names.
func twoSections(t *testing.T) *Document {
	t.Helper()
	return parseAt(t, "rules.yaml", `version: 1
sections:
  - heading: A
    rules:
      - id: A1
        text: a1
      - id: A2
        text: a2
  - heading: B
    rules:
      - id: B1
        text: b1
`)
}

func ids(d *Document) string {
	names := make([]string, 0, len(d.Rules()))
	for _, rule := range d.Rules() {
		names = append(names, rule.ID)
	}
	return strings.Join(names, ",")
}

func TestAddRulePutsItAtTheEndOfOneSection(t *testing.T) {
	doc := twoSections(t)
	if err := doc.AddRule(1, Rule{ID: "B2", Text: "b2"}); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	if got := ids(doc); got != "A1,A2,B1,B2" {
		t.Errorf("rules = %s, want A1,A2,B1,B2", got)
	}
	if got := len(doc.Sections[0].Rules); got != 2 {
		t.Errorf("the first section changed: %d rules", got)
	}
	if got := doc.Sections[1].Rules[1]; got.ID != "B2" || got.Text != "b2" {
		t.Errorf("the added rule = %+v", got)
	}
	if _, err := Parse(mustEmit(t, doc), "rules.yaml"); err != nil {
		t.Errorf("the edited document does not read back: %v", err)
	}
}

func TestSetRuleReplacesOnlyThatRule(t *testing.T) {
	doc := twoSections(t)
	next := Rule{ID: "A2-NEW", Text: "changed", Note: "because"}
	if err := doc.SetRule(0, 1, next); err != nil {
		t.Fatalf("SetRule: %v", err)
	}
	if got := ids(doc); got != "A1,A2-NEW,B1" {
		t.Errorf("rules = %s, want A1,A2-NEW,B1", got)
	}
	if got := doc.Sections[0].Rules[1]; got.ID != "A2-NEW" || got.Note != "because" {
		t.Errorf("the replaced rule = %+v", got)
	}
}

func TestDeleteRuleKeepsTheRest(t *testing.T) {
	doc := twoSections(t)
	if err := doc.DeleteRule(0, 0); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	if got := ids(doc); got != "A2,B1" {
		t.Errorf("rules = %s, want A2,B1", got)
	}
	if err := doc.DeleteRule(1, 0); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	if got := ids(doc); got != "A2" {
		t.Errorf("rules = %s, want A2", got)
	}
	if len(doc.Sections[1].Rules) != 0 {
		t.Errorf("the emptied section still holds %d rules", len(doc.Sections[1].Rules))
	}
}

// TestEditsRefuseAPlaceThatIsNotThere: the screen points by place, and a place
// the document does not have is refused rather than writing somewhere else.
func TestEditsRefuseAPlaceThatIsNotThere(t *testing.T) {
	cases := []struct {
		name string
		edit func(doc *Document) error
	}{
		{"a section past the end", func(doc *Document) error { return doc.AddRule(2, Rule{ID: "X", Text: "x"}) }},
		{"a section before the start", func(doc *Document) error { return doc.AddRule(-1, Rule{ID: "X", Text: "x"}) }},
		{"a rule past the end", func(doc *Document) error { return doc.SetRule(0, 2, Rule{ID: "X", Text: "x"}) }},
		{"a rule before the start", func(doc *Document) error { return doc.DeleteRule(0, -1) }},
		{"a rule in a section that is not there", func(doc *Document) error { return doc.SetRule(9, 0, Rule{ID: "X", Text: "x"}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.edit(twoSections(t)); err == nil {
				t.Fatal("an edit at a place that is not there was not refused")
			}
		})
	}
}

func TestSuggestRuleIDStepsPastTheTakenOnes(t *testing.T) {
	doc := twoSections(t)
	if got := doc.SuggestRuleID("C1"); got != "C1" {
		t.Errorf("SuggestRuleID(C1) = %q, want C1", got)
	}
	if got := doc.SuggestRuleID("A1"); got != "A1-2" {
		t.Errorf("SuggestRuleID(A1) = %q, want A1-2", got)
	}
	if err := doc.AddRule(1, Rule{ID: "A1-2", Text: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := doc.SuggestRuleID("A1"); got != "A1-3" {
		t.Errorf("SuggestRuleID(A1) = %q, want A1-3", got)
	}
}

// TestSectionMarkdownIsOneSection: the screen shows the section a rule stands
// in while the person edits, so the section alone is what comes back.
func TestSectionMarkdownIsOneSection(t *testing.T) {
	doc := parseAt(t, "rules.yaml", "version: 1\ntitle: T\nsections:\n  - heading: H\n    intro: I\n    style: bullets\n    rules:\n      - id: R1\n        text: one\n  - heading: K\n    rules:\n      - id: R2\n        text: two\n")
	got, err := doc.SectionMarkdown(0)
	if err != nil {
		t.Fatalf("SectionMarkdown: %v", err)
	}
	if want := "## H\n\nI\n\n- **R1** one\n"; got != want {
		t.Errorf("SectionMarkdown(0) = %q, want %q", got, want)
	}
	if got, err := doc.SectionMarkdown(1); err != nil || got != "## K\n\n- **R2** two\n" {
		t.Errorf("SectionMarkdown(1) = %q, %v", got, err)
	}
	if _, err := doc.SectionMarkdown(2); err == nil {
		t.Error("a section that is not there was not refused")
	}
	empty := parseAt(t, "rules.yaml", "version: 1\nsections:\n  - rules: []\n")
	if got, err := empty.SectionMarkdown(0); err != nil || got != "" {
		t.Errorf("SectionMarkdown of an empty section = %q, %v", got, err)
	}
}

func mustEmit(t *testing.T, doc *Document) string {
	t.Helper()
	written, err := Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	return written
}
