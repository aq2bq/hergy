package cheat

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/options"
)

// TestCheatIsEveryCommandsLineAndSummary: both cheatsheets are the table's own
// lines, a command's synopsis and under it what it does, in the table's order.
// A command added to the table shows up here without a hand.
func TestCheatIsEveryCommandsLineAndSummary(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		want := ""
		for _, kind := range options.Kinds {
			if options.Summary(kind, lang) == "" {
				t.Errorf("command %d has no %s summary", kind, lang)
			}
			want += options.Synopsis(kind, lang) + "\n    " + options.Summary(kind, lang) + "\n"
		}
		if got := commandLines(lang, ""); got != want {
			t.Errorf("[%s] the command lines:\n got %q\nwant %q", lang, got, want)
		}
		if !strings.HasSuffix(BuildPeople(lang), "\n\n"+want) {
			t.Errorf("[%s] hergy cheat does not end with the command lines", lang)
		}
	}
	for _, kind := range options.Kinds {
		if line := "\n  " + options.Synopsis(kind, "en") + "\n      " + options.Summary(kind, "en") + "\n"; !strings.Contains(Machine, line) {
			t.Errorf("hergy cheat --ai does not hold %q", line)
		}
	}
	if !strings.HasPrefix(Machine, "hergy cheat --ai\n\n  hergy up") {
		t.Errorf("hergy cheat --ai does not open with its name and the first command: %q", Machine[:40])
	}
}
