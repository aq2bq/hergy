// Package cheat projects command syntax and words' public facts for people and
// agents. It reads no team settings and does not contact herdr.
package cheat

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/words"
)

// People reads the generated cheatsheet in the selected language.
func People() string { return i18n.T("cheat.people") }

// Machine is always English: every command's line, with the flags it takes,
// and what it does in one line. A command's flags are explained by its --help.
var Machine = "hergy cheat --ai\n\n" + commandLines("en", "  ")

func commandLines(lang, indent string) string {
	var b strings.Builder
	for _, kind := range options.Kinds {
		fmt.Fprintf(&b, "%s%s\n%s    %s\n", indent, options.Synopsis(kind, lang), indent, options.Summary(kind, lang))
	}
	return b.String()
}

// BuildPeople is the locale generator's projection, separate from People so
// generating the locale never reads its old generated contents.
func BuildPeople(lang string) string {
	return words.Label("people.steps")[lang] + commandLines(lang, "")
}
