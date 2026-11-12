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

func People() string { return i18n.T("cheat.people") }

// Machine is always English. Conditions and flag meanings are projections of
// the same records help, translated errors and the documents read.
var Machine = buildMachine()

func buildMachine() string {
	var b strings.Builder
	b.WriteString("hergy cheat --ai\n\nCommands:\n")
	for _, kind := range options.Kinds {
		fmt.Fprintf(&b, "  %s\n%s", options.Synopsis(kind, "en"), indented(options.Description(kind, "en")))
	}
	b.WriteString("\nFlags:\n")
	b.WriteString(flagLines("en"))
	b.WriteString("\n")
	b.WriteString(words.Reference("en"))
	return b.String()
}

func indented(text string) string {
	return "      " + strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n      ") + "\n"
}

func flagLines(lang string) string {
	var b strings.Builder
	summaries := options.FlagSummaries()
	width := 0
	for _, flag := range summaries {
		width = max(width, i18n.Width(flag.Label))
	}
	for _, flag := range summaries {
		meaning := options.FlagDescriptions(flag.Name, lang)
		if len(flag.Commands) > 0 {
			meaning += " Commands: " + strings.Join(flag.Commands, ", ") + "."
		}
		fmt.Fprintf(&b, "  %s%s  %s\n", flag.Label, strings.Repeat(" ", width-i18n.Width(flag.Label)), meaning)
	}
	return b.String()
}

// BuildPeople is the locale generator's projection, separate from People so
// generating the locale never reads its old generated contents.
func BuildPeople(lang string) string {
	var b strings.Builder
	b.WriteString(words.Label("people.steps")[lang])
	for _, kind := range options.Kinds {
		fmt.Fprintf(&b, "%s\n%s\n\n", options.Synopsis(kind, lang), options.Description(kind, lang))
	}
	b.WriteString(flagLines(lang))
	b.WriteString("\n")
	b.WriteString(strings.ReplaceAll(words.Reference(lang), "\n  ", "\n    "))
	return b.String()
}
