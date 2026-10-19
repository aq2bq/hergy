package rules

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func parse(t *testing.T, text string) *Document {
	t.Helper()
	doc, err := Parse(text, "rules.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return doc
}

func TestParseReadsTheWholeShape(t *testing.T) {
	doc := parse(t, `version: 1
title: TEAM PROTOCOL
revision: 4
sections:
  - heading: 通信
    level: 3
    intro: 前置きである。
    style: numbered
    rules:
      - id: P-TURN-1
        text: |-
          本文である。
          続きである。
        why: 理由である。
        send_why: false
        note: 保守する人のメモ
        report: 監査.scope
        tests: [T-1, T-2]
        origin:
          - kind: incident
            date: 2026-09-25
            refs: ["#6100", "#6081"]
            note: 覚え書き
          - kind: derived
            refs: [I7]
reports:
  監査:
    where: 記録のコメント
    fields:
      scope: 変更の範囲
      who: 書いた人
`)

	if doc.Version != 1 || doc.Title != "TEAM PROTOCOL" {
		t.Errorf("version/title = %d/%q", doc.Version, doc.Title)
	}
	if doc.Revision == nil || *doc.Revision != 4 {
		t.Errorf("revision = %v, want 4", doc.Revision)
	}
	if len(doc.Sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(doc.Sections))
	}
	section := doc.Sections[0]
	if section.Heading != "通信" || section.Level != 3 || section.Intro != "前置きである。" || section.Style != StyleNumbered {
		t.Errorf("section = %+v", section)
	}
	rule := doc.Rules()[0]
	if rule.ID != "P-TURN-1" || !strings.Contains(rule.Text, "続きである。") {
		t.Errorf("rule = %+v", rule)
	}
	if rule.Why != "理由である。" || rule.SendWhy == nil || *rule.SendWhy {
		t.Errorf("why/send_why = %q/%v", rule.Why, rule.SendWhy)
	}
	if rule.Report != "監査.scope" || strings.Join(rule.Tests, ",") != "T-1,T-2" {
		t.Errorf("report/tests = %q/%v", rule.Report, rule.Tests)
	}
	if len(rule.Origin) != 2 {
		t.Fatalf("origin = %+v, want 2", rule.Origin)
	}
	incident := rule.Origin[0]
	if incident.Kind != KindIncident || incident.Date != "2026-09-25" || strings.Join(incident.Refs, ",") != "#6100,#6081" {
		t.Errorf("incident = %+v", incident)
	}
	if derived := rule.Origin[1]; derived.Kind != KindDerived || strings.Join(derived.Refs, ",") != "I7" {
		t.Errorf("derived = %+v", derived)
	}
	if len(doc.Reports) != 1 || doc.Reports[0].Name != "監査" || doc.Reports[0].Where != "記録のコメント" {
		t.Fatalf("reports = %+v", doc.Reports)
	}
	if fields := doc.Reports[0].Fields; len(fields) != 2 || fields[0].Name != "scope" || fields[1].Description != "書いた人" {
		t.Errorf("fields = %+v", fields)
	}
}

// TestParseKeepsWhatItMayLeaveOut: the optional keys are optional, and an
// unquoted date is still the text the file wrote.
func TestParseKeepsWhatItMayLeaveOut(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: |-\n          x\n        origin:\n          - kind: instruction\n            date: 2026-09-24\n            note: 指示\n")
	if doc.Title != "" || doc.Revision != nil || len(doc.Reports) != 0 {
		t.Errorf("empty optional values were not left empty: %+v", doc)
	}
	rule := doc.Rules()[0]
	if rule.SendWhy != nil || rule.Why != "" || rule.Report != "" || rule.Tests != nil {
		t.Errorf("rule = %+v, want only id, text and origin", rule)
	}
	if date := rule.Origin[0].Date; date != "2026-09-24" {
		t.Errorf("date = %q, want 2026-09-24", date)
	}
}

// TestParseAtNamesOneListOrOneString: a key that takes a list of strings also
// takes one string.
func TestParseAtNamesOneListOrOneString(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n        tests: T-1\n        origin:\n          - kind: derived\n            refs: I7\n")
	rule := doc.Rules()[0]
	if strings.Join(rule.Tests, ",") != "T-1" {
		t.Errorf("tests = %v, want one item", rule.Tests)
	}
	if strings.Join(rule.Origin[0].Refs, ",") != "I7" {
		t.Errorf("refs = %v, want one item", rule.Origin[0].Refs)
	}
}

// TestParseRefusesBreaks is the 5th section of the design: each way a rule file
// breaks stops the parse and names what it did not like.
func TestParseRefusesBreaks(t *testing.T) {
	section := "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n"
	cases := []struct {
		name string
		text string
		want string
	}{
		{"an empty file", "", "empty"},
		{"a file that is not a mapping", "- one\n- two\n", "mapping"},
		{"a file YAML cannot read", "version: 1\nsections: [\n", "yaml"},
		{"an unknown key at the top", "version: 1\nsections: [{rules: []}]\nkind: role\n", "kind"},
		{"a key twice at the top", "version: 1\ntitle: a\ntitle: b\nsections: [{rules: []}]\n", "title"},
		{"no version", "sections:\n  - rules:\n      - id: R1\n        text: x\n", "version"},
		{"a version that is not 1", "version: 2\nsections:\n  - rules:\n      - id: R1\n        text: x\n", "version"},
		{"no sections", "version: 1\ntitle: T\n", "sections"},
		{"sections empty", "version: 1\nsections: []\n", "sections"},
		{"sections not a list", "version: 1\nsections: {}\n", "list"},
		{"a section that is not a mapping", "version: 1\nsections:\n  - one\n", "mapping"},
		{"an unknown key in a section", "version: 1\nsections:\n  - heading: H\n    kinds: x\n", "kinds"},
		{"a key twice in a section", "version: 1\nsections:\n  - heading: H\n    heading: H2\n", "heading"},
		{"a level with no heading", "version: 1\nsections:\n  - rules: []\n    level: 3\n", "level"},
		{"a level outside 2..4", "version: 1\nsections:\n  - heading: H\n    level: 5\n", "5"},
		{"a level that is not an integer", "version: 1\nsections:\n  - heading: H\n    level: three\n", "integer"},
		{"a style that does not exist", "version: 1\nsections:\n  - style: table\n", "table"},
		{"rules not a list", "version: 1\nsections:\n  - rules: one\n", "list"},
		{"a rule with no id", "version: 1\nsections:\n  - rules:\n      - text: x\n", "id"},
		{"a rule with no text", "version: 1\nsections:\n  - rules:\n      - id: R1\n", "text"},
		{"a rule id that is empty", "version: 1\nsections:\n  - rules:\n      - id: ''\n        text: x\n", "id"},
		{"a rule id that does not match", "version: 1\nsections:\n  - rules:\n      - id: r1\n        text: x\n", "r1"},
		{"a rule id that is not a string", "version: 1\nsections:\n  - rules:\n      - id: 5\n        text: x\n", "string"},
		{"a text that is blank", "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: '   '\n", "blank"},
		{"a text that is not a string", "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: 5\n", "string"},
		{"an unknown key in a rule", "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n        incident: y\n", "incident"},
		{"send_why that is not a boolean", section + "        send_why: maybe\n", "false"},
		{"origin not a list", section + "        origin: {kind: incident}\n", "list"},
		{"an unknown kind", section + "        origin:\n          - kind: accident\n", "accident"},
		{"a missing kind", section + "        origin:\n          - note: x\n", "kind"},
		{"a date that is not YYYY-MM-DD", section + "        origin:\n          - kind: incident\n            date: 2026/09/25\n", "2026/09/25"},
		{"an unknown key in an origin", section + "        origin:\n          - kind: incident\n            issue: 1\n", "issue"},
		{"derived without refs", section + "        origin:\n          - kind: derived\n", "refs"},
		{"tests not a list of strings", section + "        tests:\n          - 5\n", "string"},
		{"reports not a mapping", "version: 1\nsections:\n  - rules: []\nreports: [one]\n", "mapping"},
		{"a report that is not a mapping", "version: 1\nsections:\n  - rules: []\nreports:\n  A: one\n", "mapping"},
		{"an unknown key in a report", "version: 1\nsections:\n  - rules: []\nreports:\n  A:\n    place: there\n", "place"},
		{"fields not a mapping", "version: 1\nsections:\n  - rules: []\nreports:\n  A:\n    fields: [one]\n", "mapping"},
		{"a field name twice", "version: 1\nsections:\n  - rules: []\nreports:\n  A:\n    fields:\n      scope: d\n      scope: e\n", "scope"},
		{"a field description that is not a string", "version: 1\nsections:\n  - rules: []\nreports:\n  A:\n    fields:\n      scope: 5\n", "string"},
		{"a report name twice", "version: 1\nsections:\n  - rules: []\nreports:\n  A:\n    where: x\n  A:\n    where: y\n", "A"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.text, "rules.yaml")
			if err == nil {
				t.Fatal("a broken rule file did not stop the parse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

func TestMarkdownRendersATitleHeadingsAndReasons(t *testing.T) {
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
	want := "# T\n\n## H\n\nI\n\none because\n\ntwo\n\n### H2\n\nthree\n"
	if got := doc.Markdown(); got != want {
		t.Errorf("Markdown() =\n%q\nwant\n%q", got, want)
	}
}

// TestMarkdownRendersEachStyle: the four ways a section lays its rules out, with
// the continuation lines of a multi-line body kept inside the item.
func TestMarkdownRendersEachStyle(t *testing.T) {
	cases := []struct {
		style string
		want  string
	}{
		{"paragraphs", "one\n\ntwo\n"},
		{"bullets", "- one\n- two\n"},
		{"numbered", "1. one\n2. two\n"},
		{"ids", "- **R1** one\n- **R2** two\n"},
	}
	for _, tc := range cases {
		t.Run(tc.style, func(t *testing.T) {
			doc := parse(t, "version: 1\nsections:\n  - style: "+tc.style+"\n    rules:\n      - id: R1\n        text: one\n      - id: R2\n        text: two\n")
			if got := doc.Markdown(); got != tc.want {
				t.Errorf("Markdown() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMarkdownKeepsAMultiLineBodyInsideItsItem(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - style: bullets\n    rules:\n      - id: R1\n        text: |-\n          a\n          b\n        why: |-\n          c\n          d\n")
	want := "- a\n   b c\n   d\n"
	if got := doc.Markdown(); got != want {
		t.Errorf("Markdown() = %q, want %q", got, want)
	}
}

// TestMarkdownRendersReports: the report shapes come last, under their heading,
// with each report's name, where it is written, and its fields in order.
func TestMarkdownRendersReports(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - rules: []\nreports:\n  A:\n    where: W\n    fields:\n      f: d\n      g: e\n")
	got := doc.Markdown()
	if !strings.HasPrefix(got, "## "+i18n.T("rules.reports_heading")+"\n") {
		t.Errorf("the report heading does not come first:\n%q", got)
	}
	for _, want := range []string{"**A** (W)", "- `f`: d", "- `g`: e"} {
		if !strings.Contains(got, want) {
			t.Errorf("Markdown() is missing %q:\n%q", want, got)
		}
	}
	if !strings.HasSuffix(got, "- `g`: e\n") {
		t.Errorf("the last field does not come last:\n%q", got)
	}
}

// TestMarkdownOfNothing is empty: a file whose sections hold no rule renders to
// nothing rather than to a lone newline.
func TestMarkdownOfNothing(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - rules: []\n")
	if got := doc.Markdown(); got != "" {
		t.Errorf("Markdown() = %q, want empty", got)
	}
}

func TestCheckRefusesABreakAcrossDocuments(t *testing.T) {
	report := "reports:\n  A:\n    where: W\n    fields:\n      f: d\n"
	one := oneRuleFile("")
	cases := []struct {
		name string
		docs []string
		want string
	}{
		{"a rule id that stands twice", []string{one, one}, "R1"},
		{"a report name that stands twice", []string{oneRuleFile(report), oneRuleFile(report)}, "A"},
		{"a report that is not there", []string{oneRuleFile("        report: A.absent\n")}, "A.absent"},
		{"a report with no field", []string{oneRuleFile("        report: A\n")}, "A"},
		{"a derived rule that is not there", []string{oneRuleFile("        origin:\n          - kind: derived\n            refs: [R9]\n")}, "R9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docs := make([]*Document, len(tc.docs))
			for i, text := range tc.docs {
				doc, err := Parse(text, fmt.Sprintf("doc%d.yaml", i))
				if err != nil {
					t.Fatal(err)
				}
				docs[i] = doc
			}
			err := Check(docs)
			if err == nil {
				t.Fatal("a break across the documents did not stop the check")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestCheckAcceptsPointersThatResolve: a report of one document may be named by
// a rule of another, and a derived rule may name a rule of another, when the
// member reads both.
func TestCheckAcceptsPointersThatResolve(t *testing.T) {
	docs := []*Document{
		parseAt(t, "doc1.yaml", "version: 1\nsections:\n  - rules:\n      - id: I7\n        text: x\nreports:\n  A:\n    where: W\n    fields:\n      f: d\n"),
		parseAt(t, "doc2.yaml", "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n        report: A.f\n        origin:\n          - kind: derived\n            refs: [I7]\n"),
	}
	if err := Check(docs); err != nil {
		t.Errorf("Check refused pointers that resolve: %v", err)
	}
	if err := Check(nil); err != nil {
		t.Errorf("Check(nil) = %v, want nil", err)
	}
}

// oneRuleFile is a rule file with one rule, R1; tail is added under it.
func oneRuleFile(tail string) string {
	return "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n" + tail
}

func parseAt(t *testing.T, path, text string) *Document {
	t.Helper()
	doc, err := Parse(text, path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
