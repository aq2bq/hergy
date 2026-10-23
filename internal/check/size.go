package check

import (
	"regexp"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// The size of one guide, counted the way d-81c1 and d-00a9 decide: only the
// prose a person writes and can cut. A team definition counts the part of its
// rules that goes into the prompt, and the template's own words; a role
// definition counts the part of its document that goes into the prompt. The
// roster, the turns, the workers, the hire command and hergy's own sentences are
// copies or fixed words, and none of them counts: adding a member must not raise
// a warning about a rule to cut.
//
// The limits are provisional (d-81c1, 2026-10-23): they have no measured basis.
// They move as teams use them and cut what they can; change them with a gy
// record that says what failed and how much human work went up or down, the way
// d-f2bc reviews the number it set.
const (
	// RoleCharLimit is how many characters one role document's prompt part may
	// hold.
	RoleCharLimit = 3000
	// TeamCharLimit is how many characters one team definition's rules and
	// templates may hold.
	TeamCharLimit = 3000
)

// placeholder is the %{name} a template writes to ask hergy for one of its
// parts. The placeholder is not prose a person wrote to be read, so it does not
// count: only the words around it do.
var placeholder = regexp.MustCompile(`%\{[^}]*\}`)

// templateChars is the prose of one template: its words with every %{name}
// taken out.
func templateChars(template string) int {
	return rules.CharCount(placeholder.ReplaceAllString(template, ""))
}

// teamSizeChars is one team definition's size: the Markdown its own agreement
// draws into the prompt, plus the words of the templates a person wrote (the
// team's prompt: and each member's prompt:). A template the team did not write
// is hergy's own, and does not count.
func teamSizeChars(cat *catalog.Catalog, def *definition.Definition) int {
	chars := 0
	if def.HasAgreement {
		if text, err := cat.Reader().Read(def.Agreement); err == nil {
			if doc, err := rules.Parse(text, cat.Display(def.Agreement.Path)); err == nil {
				chars += rules.CharCount(doc.Markdown())
			}
		}
	}
	chars += templateChars(def.Prompt)
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		if member.HasPrompt {
			chars += templateChars(member.Prompt)
		}
	}
	return chars
}

// TeamSizeChars is one team definition's size, for a caller that shows the
// number beside the provisional limit (hergy validate). It counts what
// teamSizeChars does: the prompt part of the team's own rules and the words of
// the templates the team wrote (d-00a9).
func TeamSizeChars(cat *catalog.Catalog, def *definition.Definition) int {
	return teamSizeChars(cat, def)
}

// teamSizeFinding is the warning a team definition over the provisional limit
// gets, or nil. It belongs to the definition's own check, so the row, the panel
// and the YAML head count it with the rest of that definition's findings.
func teamSizeFinding(cat *catalog.Catalog, def *definition.Definition) []Finding {
	chars := teamSizeChars(cat, def)
	if chars <= TeamCharLimit {
		return nil
	}
	return []Finding{{
		Kind: KindSize,
		Text: i18n.Tf("size.team_chars", chars, TeamCharLimit),
	}}
}

// roleSizeProblems is the warning a role document over the provisional limit
// gets. It belongs to the role definition's own check, so the role row and
// panel count it and no team is named.
func roleSizeProblems(chars int) []RoleProblem {
	if chars <= RoleCharLimit {
		return nil
	}
	return []RoleProblem{{
		Sev: rules.SevWarning, Section: -1, Rule: -1,
		Text: i18n.Tf("size.role_chars", chars, RoleCharLimit),
	}}
}
