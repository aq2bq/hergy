package rules

import (
	"fmt"
	"strings"
	"testing"
)

// docsAt parses one document per text, each with its own path, so a test can
// say which document a break is found in.
func docsAt(t *testing.T, texts ...string) []*Document {
	t.Helper()
	out := make([]*Document, len(texts))
	for i, text := range texts {
		out[i] = parseAt(t, fmt.Sprintf("doc%d.yaml", i), text)
	}
	return out
}

// TestCheckAllGathersEveryBreakWithItsPlace: CheckAll does not stop at the first
// break across the documents one member reads. It files every one with the
// document and the rule it stands on, in the order Check has always taken (C3).
func TestCheckAllGathersEveryBreakWithItsPlace(t *testing.T) {
	doc := parseAt(t, "roles/lead.yaml", "version: 1\nsections:\n  - rules:\n"+
		"      - id: R1\n        text: x\n"+
		"      - id: R1\n        text: x\n        report: A.absent\n        origin:\n          - kind: derived\n            refs: [R9]\n")
	problems := CheckAll([]*Document{doc})
	want := []struct {
		Field   string
		Section int
		Rule    int
	}{
		{"id", 0, 1},
		{"report", 0, 1},
		{"origin.0.refs.0", 0, 1},
	}
	if len(problems) != len(want) {
		t.Fatalf("problems = %+v, want %d", problems, len(want))
	}
	for i, w := range want {
		p := problems[i]
		if p.Sev != SevError || p.Path != "roles/lead.yaml" || p.Field != w.Field || p.Section != w.Section || p.Rule != w.Rule {
			t.Errorf("problem %d = %+v, want field %q at %d, %d", i, p, w.Field, w.Section, w.Rule)
		}
	}
	if got := problems[0].RuleID(); got != "R1" {
		t.Errorf("the repeated id's RuleID = %q, want R1", got)
	}
	if got := problems[1].RuleID(); got != "R1" {
		t.Errorf("the missing report's RuleID = %q, want R1", got)
	}
}

// TestCheckSeesARuleKeptOutOfThePrompt: a rule a file keeps out of the prompt is
// a rule still (d-f187). Two rules that share an id are one break even
// when one of them is out of the prompt, and a rule derived from one that is out
// of the prompt is not broken.
func TestCheckSeesARuleKeptOutOfThePrompt(t *testing.T) {
	same := parseAt(t, "roles/lead.yaml", "version: 1\nsections:\n  - rules:\n"+
		"      - id: R1\n        text: x\n        in_prompt: false\n"+
		"      - id: R1\n        text: y\n")
	problems := CheckAll([]*Document{same})
	if len(problems) != 1 || problems[0].Field != "id" || problems[0].Rule != 1 {
		t.Errorf("two rules share an id, one of them out of the prompt: problems = %+v, want the one repeated id at rule 1", problems)
	}

	derived := parseAt(t, "roles/lead.yaml", "version: 1\nsections:\n  - rules:\n"+
		"      - id: R1\n        text: x\n        in_prompt: false\n"+
		"      - id: R2\n        text: y\n        origin:\n          - kind: derived\n            refs: [R1]\n")
	if problems := CheckAll([]*Document{derived}); len(problems) != 0 {
		t.Errorf("a rule derived from one that is out of the prompt is called broken: %+v", problems)
	}
}

// TestCheckAllNamesTheDocumentANameStandsInTwice: a name taken in one document
// and written again in another is one break, and the sentence names both sides
// so a person can find them (C3).
func TestCheckAllNamesTheDocumentANameStandsInTwice(t *testing.T) {
	one := oneRuleFile("")
	problems := CheckAll(docsAt(t, one, one))
	if len(problems) != 1 {
		t.Fatalf("problems = %+v, want the one repeated id", problems)
	}
	for _, path := range []string{"doc0.yaml", "doc1.yaml"} {
		if !strings.Contains(problems[0].Text, path) {
			t.Errorf("the sentence %q does not name %s", problems[0].Text, path)
		}
	}
}

// TestCheckAllFirstProblemIsTheSentenceCheckReturns: the command line reads its
// one sentence from the same reading a screen reads every one from (C3).
func TestCheckAllFirstProblemIsTheSentenceCheckReturns(t *testing.T) {
	report := "reports:\n  A:\n    where: W\n    fields:\n      f: d\n"
	cases := []struct {
		name string
		docs []string
	}{
		{"a rule id that stands twice", []string{oneRuleFile(""), oneRuleFile("")}},
		{"a report name that stands twice", []string{oneRuleFile(report), oneRuleFile(report)}},
		{"a report that is not there", []string{oneRuleFile("        report: A.absent\n")}},
		{"a report with no field", []string{oneRuleFile("        report: A\n")}},
		{"a derived rule that is not there", []string{oneRuleFile("        origin:\n          - kind: derived\n            refs: [R9]\n")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docs := docsAt(t, tc.docs...)
			problems := CheckAll(docs)
			if len(problems) == 0 {
				t.Fatal("CheckAll found no break")
			}
			err := Check(docs)
			if err == nil {
				t.Fatal("Check accepted the documents")
			}
			if err.Error() != problems[0].Text {
				t.Errorf("Check = %q, CheckAll's first = %q", err, problems[0].Text)
			}
		})
	}
}

// TestCheckAllAcceptsPointersThatResolve: a report or a derived ref one document
// names may stand in another when the member reads both, and a reading with
// nothing to say gives no problem at all (C3).
func TestCheckAllAcceptsPointersThatResolve(t *testing.T) {
	docs := docsAt(t,
		"version: 1\nsections:\n  - rules:\n      - id: I7\n        text: x\nreports:\n  A:\n    where: W\n    fields:\n      f: d\n",
		"version: 1\nsections:\n  - rules:\n      - id: R1\n        text: x\n        report: A.f\n        origin:\n          - kind: derived\n            refs: [I7]\n",
	)
	if problems := CheckAll(docs); len(problems) != 0 {
		t.Errorf("CheckAll refused pointers that resolve: %+v", problems)
	}
	if problems := CheckAll(nil); problems != nil {
		t.Errorf("CheckAll(nil) = %+v, want none", problems)
	}
}
