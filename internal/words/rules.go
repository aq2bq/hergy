package words

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Text is a template in each shipped language. {{term-id}} is resolved only
// from the vocabulary. Renderers cannot substitute another name for a term.
type Text map[string]string

// RuleID identifies one public operation fact.
type RuleID string

// Rule owns one public fact and its recovery. Slots supply neutral subjects
// when explaining a precondition; an operation supplies actual subjects on refusal.
// ErrorKey is only a compatibility key for existing translated presentations.
type Rule struct {
	Detail    bool // shown under the usage line, rather than only in flag help/reference
	ID        RuleID
	Commands  []string
	Statement Text
	Recovery  Text
	Slots     []TermID
	ErrorKey  string
}

func bilingual(en, ja string) Text { return Text{"en": en, "ja": ja} }

var ruleToken = regexp.MustCompile(`\[\[([a-z][a-z0-9.-]+)\]\]`)

var termToken = regexp.MustCompile(`\{\{([a-z-]+)\}\}`)

func expand(s, lang string) string {
	s = ruleToken.ReplaceAllStringFunc(s, func(token string) string { return Find(RuleID(token[2 : len(token)-2])).Explain(lang) })
	return termToken.ReplaceAllStringFunc(s, func(token string) string {
		return TermID(token[2 : len(token)-2]).Text(lang)
	})
}

// In resolves vocabulary and shared-rule references in a template.
func (text Text) In(lang string) string { return expand(text[lang], lang) }

// Label reads a presentation label from the vocabulary-backed catalog.
func Label(id string) Text {
	text, ok := labels[id]
	if !ok {
		panic("unknown label: " + id)
	}
	return bilingual(text.In("en"), text.In("ja"))
}

// Find returns the canonical record, failing on an unknown programmer-supplied ID.
func Find(id RuleID) Rule {
	for _, rule := range rules {
		if rule.ID == id {
			return rule
		}
	}
	panic("unknown rule: " + string(id))
}

// Rules returns public records for projections and consistency checks.
func Rules() []Rule { return append([]Rule(nil), rules...) }

// Pattern joins the single cause and recovery, retaining runtime format slots.
func (r Rule) Pattern(lang string) string {
	parts := []string{r.Statement.In(lang)}
	if recovery := r.Recovery.In(lang); recovery != "" {
		parts = append(parts, recovery)
	}
	return strings.Join(parts, " ")
}

// Explain supplies neutral vocabulary names for a precondition description.
func (r Rule) Explain(lang string) string {
	args := make([]any, len(r.Slots))
	for i, slot := range r.Slots {
		args[i] = "<" + slot.Text(lang) + ">"
	}
	text := r.Pattern(lang)
	if len(args) > 0 {
		text = fmt.Sprintf(text, args...)
	}
	if r.ErrorKey != "" {
		return Label("rule.refusal")[lang] + text
	}
	return text
}

// Describe projects a sequence of rule records in one language.
func Describe(ids []RuleID, lang string) string {
	var parts []string
	for _, id := range ids {
		parts = append(parts, Find(id).Explain(lang))
	}
	return strings.Join(parts, "\n")
}

// ForCommand is the single association of commands with operation facts.
func ForCommand(command string) []RuleID {
	var out []RuleID
	for _, rule := range rules {
		if rule.Detail && slices.Contains(rule.Commands, command) {
			out = append(out, rule.ID)
		}
	}
	return out
}

// Refusal keeps the reason machine-readable through wrapping and adapters.
type Refusal struct {
	Rule RuleID
	Lang string
	Args []any
}

func (e *Refusal) Error() string { return fmt.Sprintf(Find(e.Rule).Pattern(e.Lang), e.Args...) }

// Errorf attaches an operation reason and its runtime subjects to a refusal.
func Errorf(rule RuleID, lang string, args ...any) error {
	return &Refusal{Rule: rule, Lang: lang, Args: args}
}

// GeneratedLocaleKey distinguishes projected messages from handwritten messages.
func GeneratedLocaleKey(key string) bool {
	if key == "cheat.people" || key == "usage" {
		return true
	}
	for _, rule := range rules {
		if rule.ErrorKey == key {
			return true
		}
	}
	return false
}

// Sentences supplies a shared rule wherever help describes an argument or flag.
func Sentences(ids ...RuleID) Text { return bilingual(Describe(ids, "en"), Describe(ids, "ja")) }
