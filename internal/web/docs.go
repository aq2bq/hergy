package web

import (
	"strings"

	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// ruleCard is one rule of a document, as a card shows it: its id, the first
// clause beside it so a card never stands for its id alone (bar.md E1), and the
// whole of the rule when the card is open.
type ruleCard struct {
	ID     string
	Clause string
	Text   string
	Why    string
	Origin string
	Check  string
	Note   string
	Href   string
}

// ruleCount is how many rules one document holds. A document that cannot be read
// counts none.
func (s *Server) ruleCount(path string) int {
	doc, err := s.ruleDoc(source.Doc{Path: path})
	if err != nil {
		return 0
	}
	return len(doc.Rules())
}

// sectionsOf keeps the document's sections, each with its heading and its cards.
func (s *Server) sectionsOf(doc *rules.Document, href func(string) string) []ruleSection {
	sections := make([]ruleSection, 0, len(doc.Sections))
	for _, section := range doc.Sections {
		entry := ruleSection{Heading: section.Heading}
		for _, rule := range section.Rules {
			entry.Rules = append(entry.Rules, ruleCard{
				ID:     rule.ID,
				Clause: firstClause(rule.Text),
				Text:   rule.Text,
				Why:    rule.Why,
				Origin: originText(rule.Origin),
				Check:  checkText(rule),
				Note:   rule.Note,
				Href:   href(rule.ID),
			})
		}
		if len(entry.Rules) > 0 {
			sections = append(sections, entry)
		}
	}
	return sections
}

// ruleSection is one heading of a document with its rules.
type ruleSection struct {
	Heading string
	Rules   []ruleCard
}

// originText writes a rule's origins as one line, with the kinds said the way
// validate counts them (設計 3.8、spec/39 N6).
func originText(origins []rules.Origin) string {
	parts := make([]string, 0, len(origins))
	for _, origin := range origins {
		text := originKind(origin.Kind)
		if origin.Date != "" {
			text += " " + origin.Date
		}
		if origin.Note != "" {
			text += " " + origin.Note
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, t("list.separator"))
}

// originKind is one origin's kind as a person reads it, the same words validate
// uses for its tally.
func originKind(kind string) string {
	switch kind {
	case rules.KindIncident:
		return t("web.origin.incident")
	case rules.KindInstruction:
		return t("web.origin.instruction")
	case rules.KindDerived:
		return t("web.origin.derived")
	default:
		return kind
	}
}

// checkText writes a rule's check: the report field it names and the tests it
// carries.
func checkText(rule rules.Rule) string {
	parts := make([]string, 0, 1+len(rule.Tests))
	if rule.Report != "" {
		parts = append(parts, rule.Report)
	}
	parts = append(parts, rule.Tests...)
	return strings.Join(parts, t("list.separator"))
}

// cutFirstLine is the first line of a text, with the rest.
func cutFirstLine(text string) (string, string, bool) {
	line, rest, ok := strings.Cut(text, "\n")
	return strings.TrimSpace(line), rest, ok
}
