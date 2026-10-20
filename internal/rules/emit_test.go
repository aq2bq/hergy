package rules

import (
	"reflect"
	"strings"
	"testing"
)

// TestEmitWritesTheOneFormat pins the shape of a written file: the keys in the
// order of spec/01 section 3, the default send_why left out, a multi-line value
// as a literal block, a single-line value quoted only where yaml.v3 must, and
// the reports last.
func TestEmitWritesTheOneFormat(t *testing.T) {
	doc := parse(t, `version: 1
title: T
revision: 2
sections:
  - heading: H
    level: 3
    intro: I
    rules:
      - id: R1
        text: one
        why: because
      - id: R2
        text: |-
          two
          lines
        send_why: false
        note: n
        origin:
          - kind: incident
            date: 2026-09-25
            refs: ["#1"]
            note: o
          - kind: derived
            refs: [R1]
        report: A.f
        tests: [T-1, T-2]
  - rules:
      - id: R3
        text: '5'
reports:
  A:
    where: W
    fields:
      f: d
`)
	want := `version: 1
title: T
revision: 2
sections:
  - heading: H
    level: 3
    intro: I
    rules:
      - id: R1
        text: one
        why: because
      - id: R2
        text: |-
          two
          lines
        send_why: false
        note: n
        origin:
          - kind: incident
            date: "2026-09-25"
            refs:
              - '#1'
            note: o
          - kind: derived
            refs:
              - R1
        report: A.f
        tests:
          - T-1
          - T-2
  - rules:
      - id: R3
        text: "5"
reports:
  A:
    where: W
    fields:
      f: d
`
	got, err := Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if got != want {
		t.Errorf("Emit() =\n%s\nwant\n%s", got, want)
	}
}

// TestEmitLeavesOutALeftoverStyle: hergy writes no style:, so the first save of
// a file that still writes one leaves the line out (d-f4f6).
func TestEmitLeavesOutALeftoverStyle(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - heading: H\n    style: ids\n    rules:\n      - id: R1\n        text: one\n")
	written, err := Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if strings.Contains(written, "style") {
		t.Errorf("Emit wrote a style: back:\n%s", written)
	}
}

// TestEmitIsTheSameDocumentReadBack: what hergy writes, hergy reads back as the
// same document, and the Markdown a member reads is unchanged. Emit is
// idempotent, so writing a file twice gives the same bytes.
func TestEmitIsTheSameDocumentReadBack(t *testing.T) {
	cases := []struct{ name, text string }{
		{"the whole shape", `version: 1
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
`},
		{"the optional keys left out", "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n"},
		{"a section with no heading and no rules", "version: 1\nsections:\n  - heading: H\n    rules: []\n  - rules: []\n"},
		{"a style: line of each kind left out", "version: 1\nsections:\n  - style: paragraphs\n    rules:\n      - id: R1\n        text: a\n  - style: bullets\n    rules:\n      - id: R2\n        text: b\n  - style: numbered\n    rules:\n      - id: R3\n        text: c\n  - style: ids\n    rules:\n      - id: R4\n        text: d\n"},
		{"a body that keeps its last line break", "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: |\n          a\n          b\n"},
		{"a body that would read as something else", "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: '5'\n      - id: R2\n        text: 'true'\n      - id: R3\n        text: '# no comment'\n      - id: R4\n        text: '- not a list'\n"},
		{"one string where a list is taken", "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n        tests: T-1\n        origin:\n          - kind: derived\n            refs: I7\n"},
		{"a report with no fields", "version: 1\nsections:\n  - rules: []\nreports:\n  A:\n    where: W\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Parse(tc.text, "rules.yaml")
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			written, err := Emit(doc)
			if err != nil {
				t.Fatalf("Emit: %v", err)
			}
			again, err := Parse(written, "rules.yaml")
			if err != nil {
				t.Fatalf("the file Emit wrote does not read back: %v\n%s", err, written)
			}
			if !reflect.DeepEqual(canonical(doc), canonical(again)) {
				t.Errorf("the document changed:\ngot  %+v\nwant %+v\nfile:\n%s", again, doc, written)
			}
			if got, want := again.Markdown(), doc.Markdown(); got != want {
				t.Errorf("the Markdown changed:\ngot  %q\nwant %q", got, want)
			}
			second, err := Emit(again)
			if err != nil {
				t.Fatalf("Emit: %v", err)
			}
			if second != written {
				t.Errorf("Emit is not idempotent:\nfirst  %q\nsecond %q", written, second)
			}
		})
	}
}

// canonical is a document with the one value hergy does not write folded into
// the empty one: send_why true, written out or left out, means the same thing,
// and Emit leaves it out. Empty lists become nil too, so a file that wrote
// tests: [] is the same document as one that left tests out. Two documents with
// the same canonical form are the same document.
func canonical(d *Document) Document {
	out := *d
	out.Sections = append([]Section(nil), d.Sections...)
	for i, section := range d.Sections {
		section.Rules = append([]Rule(nil), section.Rules...)
		for j, rule := range section.Rules {
			if rule.SendWhy != nil && *rule.SendWhy {
				rule.SendWhy = nil
			}
			rule.Origin = append([]Origin(nil), rule.Origin...)
			for k, origin := range rule.Origin {
				origin.Refs = append([]string(nil), origin.Refs...)
				rule.Origin[k] = origin
			}
			rule.Tests = append([]string(nil), rule.Tests...)
			section.Rules[j] = rule
		}
		out.Sections[i] = section
	}
	out.Reports = append([]Report(nil), d.Reports...)
	for i, report := range d.Reports {
		report.Fields = append([]ReportField(nil), report.Fields...)
		out.Reports[i] = report
	}
	return out
}

// FuzzEmit is the nightly fuzz target: whatever a rule file holds, what hergy
// writes back either reads as the same document or Emit says why it cannot
// write it. The seeds run with go test; tools/check/nightly.yml runs them with
// -fuzz.
func FuzzEmit(f *testing.F) {
	f.Add("version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n")
	f.Add("version: 1\ntitle: T\nrevision: 1\nsections:\n  - heading: H\n    level: 3\n    intro: I\n    style: ids\n    rules:\n      - id: R1\n        text: |-\n          a\n          b\n        why: w\n        send_why: false\n        note: n\n        report: A.f\n        tests: [T-1]\n        origin:\n          - kind: derived\n            refs: [R1]\nreports:\n  A:\n    where: W\n    fields:\n      f: d\n")
	f.Add("version: 1\nsections:\n  - rules:\n      - id: R1\n        text: |\n          a\n")
	f.Add("version: 1\nsections:\n  - rules:\n      - id: R1\n        text: '5'\n")
	f.Add("version: 2\n")
	f.Add("sections: [")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		doc, err := Parse(text, "rules.yaml")
		if err != nil {
			return // a file hergy refuses to read is not this test's business
		}
		written, err := Emit(doc)
		if err != nil {
			t.Fatalf("Emit refused a document hergy read: %v\n%s", err, text)
		}
		again, err := Parse(written, "rules.yaml")
		if err != nil {
			t.Fatalf("the file Emit wrote does not read back: %v\nwrote:\n%s", err, written)
		}
		if !reflect.DeepEqual(canonical(doc), canonical(again)) {
			t.Fatalf("the document changed:\ngot  %+v\nwant %+v\nwrote:\n%s", again, doc, written)
		}
		if got, want := again.Markdown(), doc.Markdown(); got != want {
			t.Fatalf("the Markdown changed:\ngot  %q\nwant %q", got, want)
		}
		second, err := Emit(again)
		if err != nil {
			t.Fatalf("Emit is not stable: %v", err)
		}
		if second != written {
			t.Fatalf("Emit is not idempotent:\nfirst  %q\nsecond %q", written, second)
		}
	})
}
