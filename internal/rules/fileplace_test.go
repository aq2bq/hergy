package rules

import (
	"strings"
	"testing"
)

// TestAWholeFileProblemNamesTheFile: a problem about the file as a whole says
// where the file is, not an English place-word inside a Japanese sentence
// (d-46b9).
func TestAWholeFileProblemNamesTheFile(t *testing.T) {
	doc, problems := ReadAll("version: 1\nmission: x\nsections:\n  - rules:\n      - text: a rule\n", "roles/lead.yaml")
	if doc == nil || len(problems) == 0 {
		t.Fatalf("the unknown key was not read as a problem: %+v", problems)
	}
	text := problems[0].Text
	if strings.Contains(text, "the file") {
		t.Errorf("the problem says %q, want the file's place", text)
	}
	if !strings.Contains(text, "roles/lead.yaml") {
		t.Errorf("the problem %q does not name the file", text)
	}
}
