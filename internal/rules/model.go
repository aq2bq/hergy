package rules

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aq2bq/hergy/internal/profile"
)

// A role document is meant to read the same whatever model runs it: a rule's
// body that names the model (claude, opus, gpt-4o) ties the rule to one engine,
// and hergy validate warns about it (C8, validate.rule_model_name). The words
// hergy looks for are the agent kinds a screen offers (profile.Kinds) and the
// model families behind them, so a document that names either is reported.

// modelFamilies are the model families a kind may run: the names a person writes
// when they mean the model rather than the engine.
var modelFamilies = []string{
	"opus", "sonnet", "haiku", "fable",
	"gpt", "deepseek", "qwen", "llama", "mistral", "gemma", "o1", "o3", "o4",
}

// modelTokenRE is one run of a text a model name can start in: letters and
// digits, then the letters, digits, dots and dashes a model name runs with
// (gpt-4o, deepseek-v4.1).
var modelTokenRE = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_.-]*`)

// ModelWords is the model names in a text, as they are written, in first-seen
// order and without repeats. A name is found at a word boundary and without
// regard to case: "claude", "Claude-3" and "gpt-4o" are found; "clause",
// "gptx" and "opus2x" are not.
func ModelWords(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, token := range modelTokenRE.FindAllString(text, -1) {
		word := strings.TrimRight(token, ".-")
		if modelBase(word) == "" {
			continue
		}
		key := strings.ToLower(word)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, word)
	}
	return out
}

// modelBase is the model family a written word names, in lowercase, or "" when
// the word names none. A word names a family when it is the family's name, or
// begins with it and goes on with the end of the word or a separator: gpt-4o and
// claude-3 name gpt and claude; gptx, clause and opus2x do not.
func modelBase(word string) string {
	lower := strings.ToLower(word)
	for _, name := range modelNames() {
		if !strings.HasPrefix(lower, name) {
			continue
		}
		if len(lower) == len(name) {
			return name
		}
		next, _ := utf8.DecodeRuneInString(lower[len(name):])
		if !unicode.IsLetter(next) && !unicode.IsDigit(next) {
			return name
		}
	}
	return ""
}

// modelNames is every base name hergy looks for: the model families, then the
// agent kinds a screen offers.
func modelNames() []string {
	names := make([]string, 0, len(modelFamilies)+8)
	names = append(names, modelFamilies...)
	names = append(names, profile.Kinds()...)
	return names
}
