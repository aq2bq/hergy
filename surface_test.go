package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/cheat"
	"github.com/aq2bq/hergy/internal/options"
)

// This file holds what every page a person or an agent reads says about which
// command takes which flag to the table in internal/options/spec.go. The table
// decides; a page that says otherwise fails here, and the failure names the
// page and the line.

// surface is one piece of text that pairs commands with flags.
type surface struct {
	name string
	text string
}

// surfaces are the pages that show a command and its flags: the cheatsheet for
// an agent, the short cheatsheet and the full list in both languages, the two
// READMEs and the bundled skill.
func surfaces(t *testing.T) []surface {
	t.Helper()
	out := []surface{{"hergy cheat --ai", cheat.Machine}}
	for _, lang := range []string{"en", "ja"} {
		raw, err := os.ReadFile("internal/i18n/locales/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var messages map[string]string
		if err := json.Unmarshal(raw, &messages); err != nil {
			t.Fatalf("locale %s: %v", lang, err)
		}
		out = append(out,
			surface{"hergy cheat (" + lang + ")", messages["cheat.people"]},
			surface{"hergy --help (" + lang + ")", messages["usage"]})
	}
	for _, path := range []string{"README.md", "README.ja.md", "skills/hergy/SKILL.md"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, surface{path, string(raw)})
	}
	return out
}

// A command is written "hergy <word>" at the start of a line, after two or more
// spaces, or right after a backtick: where the cheatsheets, the READMEs and the
// skill put one. Prose such as "hergy keeps no record" stays out.
var commandMention = regexp.MustCompile("(?:^| {2,}|`)hergy ([a-z][a-z0-9_-]*)")

var flagMention = regexp.MustCompile(`--[a-z][a-z0-9-]*`)

type mention struct {
	at   int
	name string
}

func kindNamed(name string) (options.Kind, bool) {
	for i, command := range options.Commands() {
		if command == name {
			return options.Kinds[i], true
		}
	}
	return 0, false
}

func acceptsFlag(kind options.Kind, flag string) bool {
	return slices.Contains(options.Flags(kind), flag)
}

// TestSurfacesPairACommandOnlyWithItsOwnFlags: wherever a line writes
// "hergy <command>" and "--flag" together, the command takes the flag. A flag
// that stands after one or more commands belongs to the one just before it; a
// flag ahead of every command is checked against all the commands on the line.
// --lang and --help, which every command has, are left alone.
func TestSurfacesPairACommandOnlyWithItsOwnFlags(t *testing.T) {
	var failures []string
	for _, s := range surfaces(t) {
		for n, line := range strings.Split(s.text, "\n") {
			var commands []mention
			for _, m := range commandMention.FindAllStringSubmatchIndex(line, -1) {
				if _, ok := kindNamed(line[m[2]:m[3]]); ok {
					commands = append(commands, mention{m[2], line[m[2]:m[3]]})
				}
			}
			if len(commands) == 0 {
				continue
			}
			for _, m := range flagMention.FindAllStringIndex(line, -1) {
				flag := strings.TrimPrefix(line[m[0]:m[1]], "--")
				if flag == "dry-run" {
					flag = "dry"
				}
				if flag == "lang" || flag == "help" {
					continue
				}
				var bound []string
				for _, c := range commands {
					if c.at < m[0] {
						bound = []string{c.name}
					}
				}
				if bound == nil {
					for _, c := range commands {
						bound = append(bound, c.name)
					}
				}
				ok := false
				for _, name := range bound {
					kind, _ := kindNamed(name)
					ok = ok || acceptsFlag(kind, flag)
				}
				if !ok {
					failures = append(failures, fmt.Sprintf("%s, line %d: --%s with hergy %s, which does not take it:\n    %s",
						s.name, n+1, flag, strings.Join(bound, "/"), strings.TrimSpace(line)))
				}
			}
		}
	}
	if len(failures) > 0 {
		t.Fatalf("%d command and flag pair(s) the table does not have:\n  %s", len(failures), strings.Join(failures, "\n  "))
	}
}

// TestCheatAICommandLinesNameEveryFlag: each command has one line in the
// cheatsheet for an agent, "  hergy <command> ...", and it names every flag the
// command has of its own (-C counts for --dir). --lang and --help, which every
// command has, are not repeated on each line.
func TestCheatAICommandLinesNameEveryFlag(t *testing.T) {
	for _, command := range options.Commands() {
		kind, _ := kindNamed(command)
		line := ""
		for l := range strings.SplitSeq(cheat.Machine, "\n") {
			if l == "  hergy "+command || strings.HasPrefix(l, "  hergy "+command+" ") {
				if line != "" {
					t.Errorf("cheat --ai has two command lines for hergy %s", command)
				}
				line = l
			}
		}
		if line == "" {
			t.Errorf("cheat --ai has no command line for hergy %s", command)
			continue
		}
		for _, flag := range options.OwnFlags(kind) {
			named := strings.Contains(line, "--"+flag) || (flag == "dir" && strings.Contains(line, "-C"))
			if !named {
				t.Errorf("cheat --ai: the line of hergy %s does not name --%s, a flag it takes:\n    %s", command, flag, strings.TrimSpace(line))
			}
		}
	}
}

// flagsSection is the "Flags:" block of the cheatsheet for an agent: the lines
// that start a flag, one per flag.
func flagsSection(t *testing.T) []string {
	t.Helper()
	var lines []string
	in := false
	for line := range strings.SplitSeq(cheat.Machine, "\n") {
		switch {
		case line == "Flags:":
			in = true
		case in && line == "":
			return lines
		case in:
			lines = append(lines, line)
		}
	}
	t.Fatal("cheat --ai has no Flags: section")
	return nil
}

var flagsLabel = regexp.MustCompile(`^  (?:-[A-Za-z], )?--([a-z][a-z0-9-]*)`)

// TestCheatAIFlagsSectionNamesTheRightCommands: the Flags: section says, flag by
// flag, which commands take it ("up only", "hire.", "up/refresh/hire."). A line
// that names a command names one that takes the line's flag, and every flag has a
// line.
func TestCheatAIFlagsSectionNamesTheRightCommands(t *testing.T) {
	token := regexp.MustCompile(`[A-Za-z_-]+`)
	described := map[string]bool{}
	for _, line := range flagsSection(t) {
		m := flagsLabel.FindStringSubmatch(line)
		if m == nil {
			t.Errorf("cheat --ai, Flags: a line that is not a flag:\n    %s", line)
			continue
		}
		flag := m[1]
		described[flag] = true
		for _, word := range token.FindAllString(line[len(m[0]):], -1) {
			kind, isCommand := kindNamed(word)
			if isCommand && !acceptsFlag(kind, flag) {
				t.Errorf("cheat --ai, Flags: the line of --%s names hergy %s, which does not take it:\n    %s", flag, word, strings.TrimSpace(line))
			}
		}
	}
	var missing []string
	for _, kind := range options.Kinds {
		for _, flag := range options.Flags(kind) {
			if !described[flag] {
				missing = append(missing, flag)
			}
		}
	}
	sort.Strings(missing)
	for i, flag := range missing {
		if i == 0 || flag != missing[i-1] {
			t.Errorf("cheat --ai: the Flags: section has no line for --%s", flag)
		}
	}
}
