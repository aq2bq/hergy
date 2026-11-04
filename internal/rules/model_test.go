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

func mustParse(t *testing.T, text string) *Document {
	t.Helper()
	doc, err := Parse(text, "rules.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return doc
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
