package rules

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// handWritten is a rule document as a person writes it: a blank line, a comment
// above a section, a named title, a single-quoted reason, a quoted date, a flow
// sequence for one rule's refs, and a multi-line text. A write must leave all of
// that alone except the rule it changes (段 4, d-7b58).
const handWritten = `version: 1
title: 手で書いた文書

# 進め方
sections:
  - heading: 進め方
    rules:
      - id: T1
        text: |-
          1 行目の本文。
          2 行目の本文。
        why: '理由はシングルクォートで書いてある'
        origin:
          - kind: incident
            date: "2026-10-15"
            refs: [d-1, d-2]
      - id: T2
        text: T2 の本文。
        note: T2 のメモ

      # T3 は最後の規則
      - id: T3
        text: T3 の本文。
        origin:
          - kind: instruction
            date: "2026-10-16"
            refs: [d-3]
`

// parseHand is the document, read.
func parseHand(t *testing.T) *Document {
	t.Helper()
	doc, err := Parse(handWritten, "agreement.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestWriteWritesOnlyTheChangedRulesLines is 段 4 の golden: changing one rule
// writes that rule's own lines, in Emit's form, and every other line — the blank
// line, the comment, T1's quoted reason and its flow refs, T3's comment — comes
// back byte for byte.
func TestWriteWritesOnlyTheChangedRulesLines(t *testing.T) {
	doc := parseHand(t)
	next := doc.Sections[0].Rules[1]
	next.Note = "書き換えたメモ"
	if err := doc.SetRule(0, 1, next); err != nil {
		t.Fatal(err)
	}
	out, err := Write([]byte(handWritten), doc, Edit{Section: 0, Rule: 1})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(handWritten,
		"      - id: T2\n        text: T2 の本文。\n        note: T2 のメモ\n",
		"      - id: T2\n        text: T2 の本文。\n        note: 書き換えたメモ\n", 1)
	if string(out) != want {
		t.Errorf("Write wrote\n%q\nwant\n%q", string(out), want)
	}
	// The rule the write did not touch keeps the shape the person wrote: its
	// reason stays single-quoted and its refs stay a flow sequence.
	for _, kept := range []string{"why: '理由はシングルクォートで書いてある'", "refs: [d-1, d-2]", "# T3 は最後の規則", "\n\n# 進め方"} {
		if !strings.Contains(string(out), kept) {
			t.Errorf("the write changed %q:\n%s", kept, out)
		}
	}
}

// TestWriteAChangedRulersFlowRefsBecomeBlockForm is the one cost of the write
// (d-7b58): the rule that is written comes out in Emit's form, so its own flow
// refs become a block sequence while the other rules keep theirs.
func TestWriteAChangedRulersFlowRefsBecomeBlockForm(t *testing.T) {
	doc := parseHand(t)
	next := doc.Sections[0].Rules[0]
	next.Text = "書き換えた本文。"
	if err := doc.SetRule(0, 0, next); err != nil {
		t.Fatal(err)
	}
	out, err := Write([]byte(handWritten), doc, Edit{Section: 0, Rule: 0})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"        text: 書き換えた本文。\n",
		"        why: 理由はシングルクォートで書いてある\n",
		"            refs:\n              - d-1\n              - d-2\n",
		"      - id: T2\n        text: T2 の本文。\n        note: T2 のメモ\n",
		"            refs: [d-3]\n",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the write does not hold %q:\n%s", want, out)
		}
	}
}

// TestWriteAddsAtTheEndOfTheSection: 足す puts the rule after the section's last
// item, and nothing above it moves. The blank line and the comment that stood
// before the last rule stay with it.
func TestWriteAddsAtTheEndOfTheSection(t *testing.T) {
	doc := parseHand(t)
	added := Rule{ID: "T4", Text: "足した本文。", Origin: []Origin{{Kind: KindInstruction, Date: "2026-10-17", Refs: []string{"d-4"}}}}
	if err := doc.AddRule(0, added); err != nil {
		t.Fatal(err)
	}
	out, err := Write([]byte(handWritten), doc, Edit{Section: 0, Rule: 3, Added: true})
	if err != nil {
		t.Fatal(err)
	}
	want := handWritten + "      - id: T4\n        text: 足した本文。\n        origin:\n          - kind: instruction\n            date: \"2026-10-17\"\n            refs:\n              - d-4\n"
	if string(out) != want {
		t.Errorf("Write wrote\n%q\nwant\n%q", string(out), want)
	}
}

// TestWriteAddsUnderASectionWithNoRules: a section that has no rules: key gets
// one, under the keys it does write, and the rule under it.
func TestWriteAddsUnderASectionWithNoRules(t *testing.T) {
	const text = "version: 1\nsections:\n  - heading: 規則\n"
	doc, err := Parse(text, "x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	added := Rule{ID: "R1", Text: "最初の規則。"}
	if err := doc.AddRule(0, added); err != nil {
		t.Fatal(err)
	}
	out, err := Write([]byte(text), doc, Edit{Section: 0, Rule: 0, Added: true})
	if err != nil {
		t.Fatal(err)
	}
	want := "version: 1\nsections:\n  - heading: 規則\n    rules:\n      - id: R1\n        text: 最初の規則。\n"
	if string(out) != want {
		t.Errorf("Write wrote\n%q\nwant\n%q", string(out), want)
	}
	// And the document that had no rule reads back with the one that was added.
	back, err := Parse(string(out), "x.yaml")
	if err != nil {
		t.Fatalf("the written document does not read: %v", err)
	}
	if len(back.Rules()) != 1 || back.Rules()[0].ID != "R1" {
		t.Errorf("the written document holds %+v", back.Rules())
	}
}

// TestWriteTakesTheRuleOutWithItsComment: 消す takes the rule's own lines, with
// the comment the file wrote above it, and the blank line around it stays.
func TestWriteTakesTheRuleOutWithItsComment(t *testing.T) {
	doc := parseHand(t)
	if err := doc.DeleteRule(0, 2); err != nil {
		t.Fatal(err)
	}
	out, err := Write([]byte(handWritten), doc, Edit{Section: 0, Rule: 2, Removed: true})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(handWritten,
		"\n      # T3 は最後の規則\n      - id: T3\n        text: T3 の本文。\n        origin:\n          - kind: instruction\n            date: \"2026-10-16\"\n            refs: [d-3]\n", "\n", 1)
	if string(out) != want {
		t.Errorf("Write wrote\n%q\nwant\n%q", string(out), want)
	}
}

// TestWriteWithTheSameValuesChangesNothing: a document hergy wrote itself, read
// and written with the same rule, comes back byte for byte.
func TestWriteWithTheSameValuesChangesNothing(t *testing.T) {
	doc := parseHand(t)
	emitted, err := Emit(doc)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(emitted, "agreement.yaml")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Write([]byte(emitted), again, Edit{Section: 0, Rule: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != emitted {
		t.Errorf("a write with the same values changed the file:\n%q\nwant\n%q", string(out), emitted)
	}
}

// TestWriteRefusesAShapeItCannotWrite: a document whose sections are one flow
// sequence has no line of one rule's own, so the write is refused with
// error.edit_shape rather than written somewhere else. The sentence names the
// document, not team.yaml (spec/42 R2).
func TestWriteRefusesAShapeItCannotWrite(t *testing.T) {
	const text = "version: 1\nsections: [{rules: [{id: T1, text: x}]}]\n"
	doc, err := Parse(text, "roles/lead.yaml")
	if err != nil {
		t.Fatal(err)
	}
	next := doc.Sections[0].Rules[0]
	next.Text = "y"
	if err := doc.SetRule(0, 0, next); err != nil {
		t.Fatal(err)
	}
	_, err = Write([]byte(text), doc, Edit{Section: 0, Rule: 0})
	if err == nil {
		t.Fatal("a flow-shaped document was written")
	}
	if want := i18n.Tf("error.edit_shape", "roles/lead.yaml", "sections"); err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

// TestWriteWithTheSameValuesKeepsAHandWrittenShape is spec/42 R1: a rule written
// by hand (a quoted text, a flow sequence for its refs) is left as the file
// wrote it when the values the write means are the ones the file holds.
func TestWriteWithTheSameValuesKeepsAHandWrittenShape(t *testing.T) {
	doc := parseHand(t)
	out, err := Write([]byte(handWritten), doc, Edit{Section: 0, Rule: 0})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != handWritten {
		t.Errorf("Write rewrote a hand-written document:\n%q\nwant\n%q", string(out), handWritten)
	}
}

// TestWriteWithAMultiLineText: a rule whose text runs over several lines is
// written whole — the lines under its key are the rule's own — when it is
// written over, and when one is added after it.
func TestWriteWithAMultiLineText(t *testing.T) {
	const text = "version: 1\nsections:\n  - rules:\n      - id: T1\n        text: |-\n          1 行目。\n          2 行目。\n"
	doc, err := Parse(text, "x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	next := doc.Sections[0].Rules[0]
	next.Text = "1 行目。\n2 行目。\n3 行目。"
	if err := doc.SetRule(0, 0, next); err != nil {
		t.Fatal(err)
	}
	out, err := Write([]byte(text), doc, Edit{Section: 0, Rule: 0})
	if err != nil {
		t.Fatal(err)
	}
	want := "version: 1\nsections:\n  - rules:\n      - id: T1\n        text: |-\n          1 行目。\n          2 行目。\n          3 行目。\n"
	if string(out) != want {
		t.Errorf("Write wrote\n%q\nwant\n%q", string(out), want)
	}

	// The same, adding a rule whose text runs over lines after one that does.
	added := Rule{ID: "T2", Text: "足した 1 行目。\n足した 2 行目。"}
	if err := doc.AddRule(0, added); err != nil {
		t.Fatal(err)
	}
	out, err = Write([]byte(want), doc, Edit{Section: 0, Rule: 1, Added: true})
	if err != nil {
		t.Fatal(err)
	}
	want += "      - id: T2\n        text: |-\n          足した 1 行目。\n          足した 2 行目。\n"
	if string(out) != want {
		t.Errorf("Write wrote\n%q\nwant\n%q", string(out), want)
	}
}
