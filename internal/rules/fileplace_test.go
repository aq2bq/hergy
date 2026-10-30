package rules

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// TestAWholeFileProblemNamesTheFile: a problem about the file as a whole says
// where the file is once, not again inside the sentence and not with an English
// place-word in a Japanese sentence (d-46b9).
func TestAWholeFileProblemNamesTheFile(t *testing.T) {
	cases := []struct{ name, text string }{
		{"an unknown key", "version: 1\ndescription: x\nsections:\n  - rules:\n      - text: a rule\n"},
		{"a key twice", "version: 1\nversion: 1\nsections:\n  - rules:\n      - text: a rule\n"},
		{"not a mapping", "- a rule\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := ReadAll(tc.text, "roles/lead.yaml")
			if len(problems) == 0 {
				t.Fatal("the problem was not read")
			}
			text := problems[0].Text
			if strings.Contains(text, "the file") {
				t.Errorf("the problem says %q, want the file's place once", text)
			}
			if got := strings.Count(text, "roles/lead.yaml"); got != 1 {
				t.Errorf("the problem %q names the place %d times, want once", text, got)
			}
		})
	}
}

// TestAProblemIsReadInTheLanguageInUse: a held reading is a sentence of the
// language it was read in, so the same text read again in another language is
// read again rather than answered from the first reading.
func TestAProblemIsReadInTheLanguageInUse(t *testing.T) {
	text := "version: 1\ndescription: x\nsections:\n  - rules:\n      - text: a rule\n"
	_, ja := ReadAll(text, "roles/lead.yaml")
	if err := i18n.Use("en"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = i18n.Use("ja") })
	_, en := ReadAll(text, "roles/lead.yaml")
	if !strings.Contains(ja[0].Text, "このファイル") {
		t.Errorf("the Japanese reading says %q", ja[0].Text)
	}
	if strings.Contains(en[0].Text, "このファイル") || !strings.Contains(en[0].Text, "this file") {
		t.Errorf("the English reading says %q", en[0].Text)
	}
}
