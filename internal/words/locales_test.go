package words

import (
	"fmt"
	"strings"
	"testing"
)

func TestLocaleMessagesExpandVocabularyAndKeepFormatSlots(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		messages := LocaleMessages(lang)
		if got := messages["status.worker_suffix"]; !strings.Contains(got, TermWorker.Text(lang)) {
			t.Fatalf("%s: worker term missing from %q", lang, got)
		}
		if got := fmt.Sprintf(messages["status.unchanged"], 7); !strings.Contains(got, "7") || strings.Contains(got, "%!") || !strings.Contains(got, TermWorking.Text(lang)) {
			t.Fatalf("%s: invalid formatted output: %q", lang, got)
		}
		for key := range messages {
			if !GeneratedLocaleKey(key) {
				t.Errorf("%s is not generated", key)
			}
		}
		rule := Find("refresh.no-pane")
		if messages[rule.ErrorKey] != rule.Pattern(lang) {
			t.Fatal("rule projection changed")
		}
		messages["status.worker_suffix"] = "changed by caller"
		if LocaleMessages(lang)["status.worker_suffix"] == "changed by caller" {
			t.Fatal("caller changed the catalog")
		}
	}
}

func TestLocaleProjectionRejectsUnknownVocabulary(t *testing.T) {
	localeMessages["test.unknown-term"] = bilingual("{{unknown-term}}", "{{unknown-term}}")
	defer delete(localeMessages, "test.unknown-term")
	defer func() {
		if got := recover(); got != "unknown term: unknown-term" {
			t.Fatalf("got %v", got)
		}
	}()
	LocaleMessages("en")
}
