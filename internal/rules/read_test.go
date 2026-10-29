package rules

import (
	"strings"
	"testing"
)

// TestReadAllGathersEveryProblemAndTheDocumentSoFar: ReadAll does not stop at
// the first problem. It files every one in the written order, and gives the
// document as far as it read, so a screen can show every problem at once (C2).
func TestReadAllGathersEveryProblemAndTheDocumentSoFar(t *testing.T) {
	text := `version: 1
title: 5
sections:
  - heading: H
    level: 5
    rules:
      - id: r1
        text: '   '
      - id: OK-1
        text: fine
`
	doc, problems := ReadAll(text, "rules.yaml")
	if doc == nil {
		t.Fatal("ReadAll gave no document")
	}
	// The document is read as far as it goes: the good rule comes through, the
	// value the shape refused is left out, and the rule the shape refused keeps
	// the id it did read.
	if doc.Version != 1 {
		t.Errorf("version = %d, want 1", doc.Version)
	}
	if doc.Title != "" {
		t.Errorf("title = %q, want the refused value left out", doc.Title)
	}
	if len(doc.Sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(doc.Sections))
	}
	if doc.Sections[0].Level != 0 {
		t.Errorf("level = %d, want the refused level left out", doc.Sections[0].Level)
	}
	ids := make([]string, len(doc.Sections[0].Rules))
	for i, rule := range doc.Sections[0].Rules {
		ids[i] = rule.ID
	}
	if strings.Join(ids, ",") != "r1,OK-1" {
		t.Errorf("rules = %v, want the whole list", ids)
	}

	var fields []string
	for _, p := range problems {
		fields = append(fields, p.Field)
		if p.Sev != SevError {
			t.Errorf("problem %+v is not an error; a rule file's own shape gives errors", p)
		}
		if p.Text == "" || p.err == nil {
			t.Errorf("problem %+v has no sentence to give", p)
		}
	}
	if want := "title,sections[0].level,text"; strings.Join(fields, ",") != want {
		t.Errorf("fields = %v, want %s", fields, want)
	}
}

// TestReadAllTakesARuleWithNoID: an id is hergy's to give, so a rule written
// without one (or with one in an older shape) is read as it stands and is not a
// problem: the next write gives it an id (d-4aab).
func TestReadAllTakesARuleWithNoID(t *testing.T) {
	text := "version: 1\ntitle: T\nsections:\n  - heading: H\n    rules:\n      - text: no id yet\n      - id: C1\n        text: an older shape\n"
	doc, problems := ReadAll(text, "rules.yaml")
	if len(problems) != 0 {
		t.Fatalf("problems = %+v, want none", problems)
	}
	rules := doc.Rules()
	if rules[0].ID != "" || rules[1].ID != "C1" {
		t.Errorf("ids = %q, %q; want the file's own", rules[0].ID, rules[1].ID)
	}
}

// TestReadAllFirstProblemIsTheSentenceParseReturns: the command line reads its
// one sentence from the same reading a screen reads every one from, so they
// cannot say different sentences (C2).
func TestReadAllFirstProblemIsTheSentenceParseReturns(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"an empty file", ""},
		{"a file that is not a mapping", "- one\n- two\n"},
		{"a file YAML cannot read", "version: 1\nsections: [\n"},
		{"a file with several problems", "version: 1\ntitle: 5\nsections:\n  - heading: H\n    level: 5\n"},
		{"no version", "sections:\n  - rules: []\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := ReadAll(tc.text, "rules.yaml")
			if len(problems) == 0 {
				t.Fatal("ReadAll found no problem in a file with one")
			}
			_, err := Parse(tc.text, "rules.yaml")
			if err == nil {
				t.Fatal("Parse accepted the file")
			}
			if err.Error() != problems[0].Text {
				t.Errorf("Parse = %q, ReadAll's first = %q", err, problems[0].Text)
			}
		})
	}
}

// TestReadAllOfAFileThatDoesNotReadGivesNoDocument: a file hergy cannot read at
// all gives its one problem and no document, so a screen has nothing to draw
// from it.
func TestReadAllOfAFileThatDoesNotReadGivesNoDocument(t *testing.T) {
	doc, problems := ReadAll("", "roles/lead.yaml")
	if doc != nil {
		t.Errorf("doc = %+v, want none", doc)
	}
	if len(problems) != 1 || problems[0].Path != "roles/lead.yaml" {
		t.Errorf("problems = %+v, want the file's one", problems)
	}
}

// TestReadAllOfAFileThatIsNotAMappingStillGivesADocument: a file that is a
// list, not a mapping, has no keys to read, and still hands back the document
// it is, so the caller never gets a nil document it has to guard (C2).
func TestReadAllOfAFileThatIsNotAMappingStillGivesADocument(t *testing.T) {
	doc, problems := ReadAll("- one\n- two\n", "roles/lead.yaml")
	if doc == nil || doc.Path != "roles/lead.yaml" {
		t.Fatalf("doc = %+v, want the document with its path", doc)
	}
	if len(problems) != 1 || problems[0].Line == 0 {
		t.Errorf("problems = %+v, want the one problem on its line", problems)
	}
	if len(doc.Sections) != 0 {
		t.Errorf("doc.Sections = %+v, want none", doc.Sections)
	}
}

// TestReadAllWarnsAboutALeftoverStyle: the layout of a section is one way now,
// so a style: line is read, left alone, and named once as a warning that asks
// for the line to be deleted (d-f4f6). The file still reads, and the sentence
// names the file and the section: its heading, or the place the file writes it
// in when the section has no heading.
func TestReadAllWarnsAboutALeftoverStyle(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{
			"a section with a heading",
			"version: 1\nsections:\n  - heading: 約束\n    style: ids\n    rules:\n      - id: R1\n        text: one\n",
			"約束",
		},
		{
			"a section with no heading",
			"version: 1\nsections:\n  - style: bullets\n    rules: []\n",
			"sections[0]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, problems := ReadAll(tc.text, "roles/lead.yaml")
			if doc == nil || len(doc.Sections) != 1 {
				t.Fatalf("the file with a style: did not read: %+v", doc)
			}
			if len(problems) != 1 {
				t.Fatalf("problems = %+v, want one warning", problems)
			}
			p := problems[0]
			if p.Sev != SevWarning || p.Section != 0 || p.Rule != -1 || p.Field != "sections[0].style" || p.Line == 0 {
				t.Errorf("problem = %+v, want a warning about the section's style on its line", p)
			}
			if !strings.Contains(p.Text, "roles/lead.yaml") || !strings.Contains(p.Text, tc.want) {
				t.Errorf("the warning does not name the file and %q: %q", tc.want, p.Text)
			}
			if _, err := Parse(tc.text, "roles/lead.yaml"); err != nil {
				t.Errorf("Parse stopped on a style: line: %v", err)
			}
		})
	}
}

// TestReadAllWarnsOnceForEachSection: every section that writes a style: gets
// its own warning, in the written order, and the file reads without one.
func TestReadAllWarnsOnceForEachSection(t *testing.T) {
	text := "version: 1\nsections:\n  - heading: A\n    style: ids\n    rules:\n      - id: R1\n        text: one\n  - heading: B\n    rules:\n      - id: R2\n        text: two\n      - id: R3\n        text: three\n"
	_, problems := ReadAll(text, "roles/lead.yaml")
	if len(problems) != 1 || problems[0].Sev != SevWarning {
		t.Fatalf("problems = %+v, want one warning", problems)
	}
	if _, err := Parse(text, "roles/lead.yaml"); err != nil {
		t.Errorf("Parse stopped: %v", err)
	}
}
