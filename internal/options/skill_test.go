package options

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// hergyCommand finds a command mention. A command is written as "hergy <word>"
// at the start of a line, after two or more spaces, or right after a backtick:
// those are where the cheatsheet and a skill's code put one. Prose such as
// "hergy when unset" stays out, so the test checks names, not English.
var hergyCommand = regexp.MustCompile("(?m)(?:^| {2,}|`)hergy ([a-z][a-z0-9_-]*)")

// hergyFlag finds a long flag. The short -C and -h are not written as "--x",
// and the test treats --dir, --help and the --dry-run alias as existing.
var hergyFlag = regexp.MustCompile(`--[a-z][a-z0-9-]*`)

// TestSkillAndCheatNamesExist walks the names the bundled skill and the
// people's cheat put in front of a person or an agent. A name hergy does not
// have is a promise it cannot keep, so the test prints the name and fails.
func TestSkillAndCheatNamesExist(t *testing.T) {
	sources := map[string]string{}

	skill, err := os.ReadFile("../../skills/hergy/SKILL.md")
	if err != nil {
		t.Fatalf("read the bundled skill: %v", err)
	}
	sources["skills/hergy/SKILL.md"] = string(skill)

	for _, lang := range []string{"en", "ja"} {
		raw, err := os.ReadFile("../../internal/i18n/locales/" + lang + ".json")
		if err != nil {
			t.Fatalf("read locale %s: %v", lang, err)
		}
		var messages map[string]string
		if err := json.Unmarshal(raw, &messages); err != nil {
			t.Fatalf("locale %s: %v", lang, err)
		}
		sources["locales/"+lang+".json (cheat.people)"] = messages["cheat.people"]
	}

	commands := map[string]bool{}
	for _, name := range Commands() {
		commands[name] = true
	}

	flags := map[string]bool{}
	for _, kind := range Kinds {
		for _, name := range Flags(kind) {
			flags[name] = true
		}
	}
	flags["dir"] = true     // -C / --dir
	flags["help"] = true    // -h / --help
	flags["dry-run"] = true // an alias of --dry

	for name, text := range sources {
		for _, m := range hergyCommand.FindAllStringSubmatch(text, -1) {
			if !commands[m[1]] {
				t.Errorf("%s names the command %q, which hergy does not have", name, m[1])
			}
		}
		for _, f := range hergyFlag.FindAllString(text, -1) {
			if !flags[strings.TrimPrefix(f, "--")] {
				t.Errorf("%s names the flag %q, which hergy does not have", name, f)
			}
		}
	}
}
