package cheat

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/options"
)

// TestEveryCommandAndFlagHasItsWords: the cheatsheet for an agent writes what a
// command does and what a flag means by hand; the command line and the commands
// that take a flag come from the table. A command or flag added to the table
// without those words fails here.
func TestEveryCommandAndFlagHasItsWords(t *testing.T) {
	commands := map[options.Kind]string{}
	for i, kind := range options.Kinds {
		commands[kind] = options.Commands()[i]
		text, ok := behavior[kind]
		if !ok || strings.TrimSpace(text) == "" {
			t.Errorf("hergy %s has no description in behavior (internal/cheat/cheat.go)", options.Commands()[i])
		}
		for line := range strings.SplitSeq(strings.TrimRight(text, "\n"), "\n") {
			if !strings.HasPrefix(line, "      ") {
				t.Errorf("hergy %s: a line of its description does not start with six spaces: %q", options.Commands()[i], line)
			}
		}
	}
	for kind := range behavior {
		if _, ok := commands[kind]; !ok {
			t.Errorf("behavior describes a command kind (%d) that is not in the table", kind)
		}
	}

	flags := map[string]bool{}
	for _, sum := range options.FlagSummaries() {
		flags[sum.Name] = true
		if strings.TrimSpace(flagMeaning[sum.Name]) == "" {
			t.Errorf("--%s has no meaning in flagMeaning (internal/cheat/cheat.go)", sum.Name)
		}
	}
	for name := range flagMeaning {
		if !flags[name] {
			t.Errorf("flagMeaning describes --%s, which hergy does not have", name)
		}
	}
}

// TestMachineLinesComeFromTheTable: each command's line is the Synopsis of its
// row, so a flag added to or taken from the table shows up here without a hand.
func TestMachineLinesComeFromTheTable(t *testing.T) {
	for _, kind := range options.Kinds {
		want := "\n  " + options.Synopsis(kind, "en") + "\n"
		if !strings.Contains(Machine, want) {
			t.Errorf("cheat --ai does not hold the command line %q", strings.TrimSpace(want))
		}
	}
	for _, sum := range options.FlagSummaries() {
		if !strings.Contains(Machine, "\n  "+sum.Label+" ") {
			t.Errorf("cheat --ai has no Flags: line for %q", sum.Label)
		}
		if len(sum.Commands) > 0 && !strings.Contains(Machine, "Commands: "+strings.Join(sum.Commands, ", ")+".\n") {
			t.Errorf("cheat --ai does not list the commands that take --%s: %v", sum.Name, sum.Commands)
		}
	}
}
