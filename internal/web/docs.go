package web

import (
	"strings"

	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// ruleCard is one rule of a document, as a card shows it: its id, the first
// clause beside it so a card never stands for its id alone (bar.md E1), and the
// sentences check writes about its text (設計 3.8). The whole of the rule is
// what the card's own field writes.
type ruleCard struct {
	ID       string
	Clause   string
	Href     string
	Findings []string
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

// sectionsOf keeps the document's sections, each with its heading, its cards,
// and where a rule added at its end goes (設計 3.8). warnings is the sentence
// check writes about each rule's text, by rule id.
func (s *Server) sectionsOf(doc *rules.Document, href func(string) string, add func(int) string, warnings map[string][]string) []ruleSection {
	sections := make([]ruleSection, 0, len(doc.Sections))
	for i, section := range doc.Sections {
		entry := ruleSection{Heading: section.Heading}
		if add != nil {
			entry.AddHref = add(i)
		}
		for _, rule := range section.Rules {
			entry.Rules = append(entry.Rules, ruleCard{
				ID:       rule.ID,
				Clause:   firstClause(rule.Text),
				Href:     href(rule.ID),
				Findings: warnings[rule.ID],
			})
		}
		sections = append(sections, entry)
	}
	return sections
}

// ruleSection is one heading of a document with its rules, and the card a rule
// added at its end is written with.
type ruleSection struct {
	Heading string
	Rules   []ruleCard
	AddHref string
}

// cutFirstLine is the first line of a text, with the rest.
func cutFirstLine(text string) (string, string, bool) {
	line, rest, ok := strings.Cut(text, "\n")
	return strings.TrimSpace(line), rest, ok
}
