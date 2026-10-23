package rules

import (
	"os"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		panic(err)
	}
	if err := i18n.Use("ja"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func parseAt(t *testing.T, path, text string) *Document {
	t.Helper()
	doc, err := Parse(text, path)
	if err != nil {
		t.Fatalf("Parse(%s): %v", path, err)
	}
	return doc
}

func mustParse(t *testing.T, text string) *Document {
	t.Helper()
	doc, err := Parse(text, "rules.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return doc
}

// TestReadsAndWritesTheNewShape: a rule holds its id, body, on/off switch, the
// day it was switched off and a note for people; Emit writes that shape back.
func TestReadsAndWritesTheNewShape(t *testing.T) {
	doc := mustParse(t, `version: 1
title: T
revision: 2
sections:
  - heading: H
    intro: I
    rules:
      - id: R1
        text: 本文。
        note: 人のためのメモ。
      - id: R2
        text: 止めた本文。
        in_prompt: false
        off_since: 2026-10-23
        note: 様子見。
`)
	if len(doc.Sections) != 1 || len(doc.Sections[0].Rules) != 2 {
		t.Fatalf("document = %+v", doc)
	}
	first := doc.Sections[0].Rules[0]
	if first.ID != "R1" || first.Text != "本文。" || !first.Included() || first.Note != "人のためのメモ。" {
		t.Errorf("first = %+v", first)
	}
	second := doc.Sections[0].Rules[1]
	if second.Included() || second.OffSince != "2026-10-23" || second.Note != "様子見。" {
		t.Errorf("second = %+v", second)
	}
	written, err := Emit(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"id: R1", "text: 本文。", "note: 人のためのメモ。", "in_prompt: false", "off_since:", "2026-10-23"} {
		if !strings.Contains(written, want) {
			t.Errorf("the write is missing %q:\n%s", want, written)
		}
	}
	again := mustParse(t, written)
	if !sameRule(again.Rules()[1], second) || !sameRule(again.Rules()[0], first) {
		t.Errorf("the document changed: %+v", again)
	}
}

// TestInPromptTrueIgnoresAnOldOffDay: a rule that is on is read as having no
// off day, and hergy does not write one (d-9f30).
func TestInPromptTrueIgnoresAnOldOffDay(t *testing.T) {
	doc := mustParse(t, "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n        off_since: 2026-10-23\n")
	if got := doc.Rules()[0].OffSince; got != "" {
		t.Errorf("off_since = %q, want it ignored while in_prompt is true", got)
	}
	written, err := Emit(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(written, "off_since") {
		t.Errorf("the write kept an off day on a rule that is on:\n%s", written)
	}
}

// TestOldRuleItemsStopWithTheFileAndRule: a document with an old item stops, and
// the sentence names the file, the rule and where the content goes (d-85dc,
// r-81be).
func TestOldRuleItemsStopWithTheFileAndRule(t *testing.T) {
	cases := []struct {
		name, key, advice string
	}{
		{"why", "why: 理由", "本文"},
		{"send_why", "send_why: false", "メモ"},
		{"origin", "origin:\n          - kind: incident", "メモ"},
		{"report", "report: 監査.scope", "消して"},
		{"tests", "tests: [T-1]", "消して"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n        " + tc.key + "\n"
			_, problems := ReadAll(text, "rules.yaml")
			found := false
			for _, p := range problems {
				if strings.Contains(p.Text, "R1") && strings.Contains(p.Text, tc.advice) {
					found = true
				}
			}
			if !found {
				t.Errorf("problems = %+v, want R1 and the advice %q", problems, tc.advice)
			}
			if _, err := Parse(text, "rules.yaml"); err == nil {
				t.Error("an old item did not stop the file")
			}
		})
	}
}

// TestReportsStopByName: the document-level reports block is gone and is named
// with its item, not silently dropped.
func TestReportsStopByName(t *testing.T) {
	text := "version: 1\nsections:\n  - rules: []\nreports:\n  監査:\n    where: W\n"
	_, problems := ReadAll(text, "rules.yaml")
	if len(problems) != 1 || !strings.Contains(problems[0].Text, "reports.監査") {
		t.Errorf("problems = %+v, want reports.監査 named", problems)
	}
}

// TestCheckRefusesARepeatedRuleID: ids are unique among the documents one
// member reads; the other report and derived checks are gone.
func TestCheckRefusesARepeatedRuleID(t *testing.T) {
	one := mustParse(t, "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n")
	two := mustParse(t, "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: y\n")
	problems := CheckAll([]*Document{one, two})
	if len(problems) != 1 || problems[0].RuleID() != "R1" {
		t.Errorf("problems = %+v, want the repeated R1", problems)
	}
}

// TestMarkdownAndLines: the body is the Markdown a member reads; an excluded
// rule is one struck line in the preview and none in the prompt.
func TestMarkdownAndLines(t *testing.T) {
	doc := mustParse(t, `version: 1
title: T
sections:
  - heading: H
    intro: I
    rules:
      - id: R1
        text: one
      - id: R2
        text: two
        in_prompt: false
`)
	if got, want := doc.Markdown(), "# T\n\n## H\n\nI\n\n- **R1** one\n"; got != want {
		t.Errorf("Markdown = %q, want %q", got, want)
	}
	if len(doc.IncludedRules()) != 1 || doc.ExcludedCount() != 1 {
		t.Errorf("included/excluded = %d/%d", len(doc.IncludedRules()), doc.ExcludedCount())
	}
	excluded := 0
	for _, line := range doc.PreviewLines() {
		if line.Part == PartExcluded {
			excluded++
		}
	}
	if excluded != 1 {
		t.Errorf("preview excluded lines = %d, want 1", excluded)
	}
}

// FuzzEmit: whatever a rule file holds, what hergy writes back reads as the
// same document or Emit says why it cannot write it.
func FuzzEmit(f *testing.F) {
	f.Add("version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n")
	f.Add("version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n        in_prompt: false\n        off_since: 2026-10-23\n        note: n\n")
	f.Add("version: 1\ntitle: T\nrevision: 1\nsections:\n  - heading: H\n    level: 3\n    intro: I\n    style: ids\n    rules:\n      - id: R1\n        text: |-\n          a\n          b\n")
	f.Fuzz(func(t *testing.T, text string) {
		doc, err := Parse(text, "rules.yaml")
		if err != nil {
			return
		}
		written, err := Emit(doc)
		if err != nil {
			return
		}
		again, err := Parse(written, "rules.yaml")
		if err != nil {
			t.Fatalf("the file Emit wrote does not read back: %v\n%s", err, written)
		}
		if !sameDocument(doc, again) {
			t.Fatalf("the document changed:\ngot  %+v\nwant %+v\nfile:\n%s", again, doc, written)
		}
	})
}

func sameDocument(a, b *Document) bool {
	if a.Version != b.Version || a.Title != b.Title || (a.Revision == nil) != (b.Revision == nil) {
		return false
	}
	if a.Revision != nil && *a.Revision != *b.Revision {
		return false
	}
	if len(a.Sections) != len(b.Sections) {
		return false
	}
	for i := range a.Sections {
		as, bs := a.Sections[i], b.Sections[i]
		if as.Heading != bs.Heading || as.Level != bs.Level || as.Intro != bs.Intro || len(as.Rules) != len(bs.Rules) {
			return false
		}
		for j := range as.Rules {
			if !sameRule(as.Rules[j], bs.Rules[j]) {
				return false
			}
		}
	}
	return true
}

// TestFillOffDatesWritesAndClearsTheDay: a save that turns a rule off writes the
// day; a save that turns it on clears it; a rule that was already off keeps the
// day it has, even when it has none (d-9f30).
func TestFillOffDatesWritesAndClearsTheDay(t *testing.T) {
	on := mustParse(t, "version: 1\nsections:\n  - rules:\n      - id: A\n        text: a\n")
	draft := mustParse(t, "version: 1\nsections:\n  - rules:\n      - id: A\n        text: a\n        in_prompt: false\n")
	FillOffDates(on, draft, "2026-10-23")
	if got := draft.Rules()[0].OffSince; got != "2026-10-23" {
		t.Errorf("off_since = %q, want today", got)
	}

	off := mustParse(t, "version: 1\nsections:\n  - rules:\n      - id: A\n        text: a\n        in_prompt: false\n")
	back := mustParse(t, "version: 1\nsections:\n  - rules:\n      - id: A\n        text: a\n")
	FillOffDates(off, back, "2026-10-23")
	if got := back.Rules()[0].OffSince; got != "" {
		t.Errorf("off_since = %q, want it cleared", got)
	}

	manual := mustParse(t, "version: 1\nsections:\n  - rules:\n      - id: A\n        text: a\n        in_prompt: false\n")
	FillOffDates(off, manual, "2026-10-23")
	if got := manual.Rules()[0].OffSince; got != "" {
		t.Errorf("off_since = %q, a manual off day was written", got)
	}
}
