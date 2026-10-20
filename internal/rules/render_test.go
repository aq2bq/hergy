package rules

import (
	"strings"
	"testing"
)

// TestLinesAreTheLinesOfTheMarkdown: the preview a screen draws (Lines) and the
// prompt a member is handed (Markdown) cannot drift apart. Joining the lines
// with one line break and adding the last one is the Markdown itself (C1).
func TestLinesAreTheLinesOfTheMarkdown(t *testing.T) {
	texts := []string{
		"version: 1\nsections:\n  - rules: []\n",
		"version: 1\ntitle: T\nsections:\n  - heading: H\n    intro: I\n    rules:\n      - id: R1\n        text: one\n        why: because\n  - heading: H2\n    rules:\n      - id: R2\n        text: two\nreports:\n  A:\n    where: W\n    fields:\n      f: d\n",
		"version: 1\nsections:\n  - style: bullets\n    rules:\n      - id: R1\n        text: |-\n          a\n          b\n      - id: R2\n        text: two\n",
	}
	for _, text := range texts {
		doc := parse(t, text)
		var parts []string
		for _, line := range doc.Lines() {
			if strings.Contains(line.Text, "\n") {
				t.Errorf("a line holds a line break: %q", line.Text)
			}
			parts = append(parts, line.Text)
		}
		got := ""
		if len(parts) > 0 {
			got = strings.Join(parts, "\n") + "\n"
		}
		if got != doc.Markdown() {
			t.Errorf("Lines joined = %q, Markdown() = %q", got, doc.Markdown())
		}
	}
}

// TestLinesMarkEachPartAndPlace: every line says what it stands for and where it
// stands, with a blank line between blocks, and the reason a send_why: false
// rule holds sits beside its last line without entering the text (C1, M2).
func TestLinesMarkEachPartAndPlace(t *testing.T) {
	doc := parse(t, `version: 1
title: T
sections:
  - heading: H
    intro: I
    rules:
      - id: R1
        text: one
        why: because
      - id: R2
        text: two
        why: hidden
        send_why: false
  - heading: H2
    level: 3
    rules:
      - id: R3
        text: three
`)
	want := []MDLine{
		{Text: "# T", Part: PartTitle, Section: -1, Rule: -1},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "## H", Part: PartHeading, Section: 0, Rule: -1},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "I", Part: PartIntro, Section: 0, Rule: -1},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "- **R1** one because", Part: PartRule, Section: 0, Rule: 0},
		{Text: "- **R2** two", Part: PartRule, Section: 0, Rule: 1, Held: "hidden"},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "### H2", Part: PartHeading, Section: 1, Rule: -1},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "- **R3** three", Part: PartRule, Section: 1, Rule: 0},
	}
	got := doc.Lines()
	if len(got) != len(want) {
		t.Fatalf("lines = %d, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestLinesLeaveOutARuleKeptOutOfThePrompt: a rule the file keeps out of the
// prompt (in_prompt: false) has no line, and the rules around it keep the place
// they stand in, so a line that does stand points at the rule it is
// (d-f187, C6).
func TestLinesLeaveOutARuleKeptOutOfThePrompt(t *testing.T) {
	doc := parse(t, `version: 1
title: T
sections:
  - heading: H
    rules:
      - id: R1
        text: one
      - id: R2
        text: two
        why: because
        in_prompt: false
      - id: R3
        text: three
`)
	want := []MDLine{
		{Text: "# T", Part: PartTitle, Section: -1, Rule: -1},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "## H", Part: PartHeading, Section: 0, Rule: -1},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "- **R1** one", Part: PartRule, Section: 0, Rule: 0},
		{Text: "- **R3** three", Part: PartRule, Section: 0, Rule: 2},
	}
	got := doc.Lines()
	if len(got) != len(want) {
		t.Fatalf("lines = %d, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if want := "# T\n\n## H\n\n- **R1** one\n- **R3** three\n"; doc.Markdown() != want {
		t.Errorf("Markdown = %q, want %q", doc.Markdown(), want)
	}
}

// TestLinesOfAListKeepTheItemsInOneBlock: the rules of a section are one block
// of list items, with no blank line between them, and the continuation lines of
// a multi-line body stay in that one block (C1). A style: a file still writes
// makes no difference.
func TestLinesOfAListKeepTheItemsInOneBlock(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - style: bullets\n    rules:\n      - id: R1\n        text: |-\n          a\n          b\n      - id: R2\n        text: two\n")
	want := []MDLine{
		{Text: "- **R1** a", Part: PartRule, Section: 0, Rule: 0},
		{Text: "   b", Part: PartRule, Section: 0, Rule: 0},
		{Text: "- **R2** two", Part: PartRule, Section: 0, Rule: 1},
	}
	got := doc.Lines()
	if len(got) != len(want) {
		t.Fatalf("lines = %d, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestLinesHoldNothingWithoutAReason: a rule whose send_why is false but whose
// why is empty holds nothing, and a rule whose reason is sent keeps it in its
// text (C1).
func TestLinesHoldNothingWithoutAReason(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: one\n        send_why: false\n")
	lines := doc.Lines()
	if len(lines) != 1 {
		t.Fatalf("lines = %+v, want the one rule line", lines)
	}
	if lines[0].Held != "" || lines[0].Text != "- **R1** one" {
		t.Errorf("line = %+v, want the text with no held reason", lines[0])
	}
}

// TestLinesOfNothingIsEmpty: a document whose sections hold no rule and which
// has no title renders to no line at all, the way Markdown renders to nothing.
func TestLinesOfNothingIsEmpty(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - rules: []\n")
	if got := doc.Lines(); len(got) != 0 {
		t.Errorf("Lines() = %+v, want empty", got)
	}
}

// TestPreviewLinesKeepAnExcludedRuleInItsPlace: the preview a screen draws
// (PreviewLines) holds a rule the file keeps out of the prompt where the rule
// stands, as one line with no line number, while Lines and Markdown leave it out
// (spec-teams C1).
func TestPreviewLinesKeepAnExcludedRuleInItsPlace(t *testing.T) {
	doc := parse(t, `version: 1
title: T
sections:
  - heading: H
    rules:
      - id: R1
        text: one
      - id: R2
        text: two
        why: because
        in_prompt: false
      - id: R3
        text: |-
          three
          more
`)
	want := []MDLine{
		{Text: "# T", Part: PartTitle, Section: -1, Rule: -1},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "## H", Part: PartHeading, Section: 0, Rule: -1},
		{Part: PartBlank, Section: -1, Rule: -1},
		{Text: "- **R1** one", Part: PartRule, Section: 0, Rule: 0},
		{Text: "R2 two", Part: PartExcluded, Section: 0, Rule: 1},
		{Text: "- **R3** three", Part: PartRule, Section: 0, Rule: 2},
		{Text: "   more", Part: PartRule, Section: 0, Rule: 2},
	}
	got := doc.PreviewLines()
	if len(got) != len(want) {
		t.Fatalf("preview lines = %d, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := doc.Lines(); len(got) != len(want)-1 {
		t.Errorf("Lines() = %d lines, want the excluded rule left out", len(got))
	}
	if want := "# T\n\n## H\n\n- **R1** one\n- **R3** three\n   more\n"; doc.Markdown() != want {
		t.Errorf("Markdown() = %q, want %q", doc.Markdown(), want)
	}
}
