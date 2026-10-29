package rules

import (
	"github.com/aq2bq/hergy/internal/i18n"
)

// Check refuses the one break that spans the documents one member reads: a rule
// id that stands twice. The documents are the ones that go into that member's
// prompt, so the same two documents are checked once for each member that reads
// them.
func Check(docs []*Document) error {
	problems := CheckAll(docs)
	if len(problems) == 0 {
		return nil
	}
	return problems[0].err
}

// CheckAll reads the breaks that span the documents one member reads the way a
// screen needs them: not stopping at the first, but gathering every one with the
// document, the section and the rule it stands on (C3). The order is the one
// Check has always taken, so its first problem is the sentence Check returns.
// Section and Rule are the rule's place in its own document, or -1 when the
// problem is not about one rule.
func CheckAll(docs []*Document) []Problem {
	set := newSet()
	var problems []Problem
	for _, doc := range docs {
		problems = append(problems, set.addAll(doc)...)
	}
	return problems
}

// set indexes the rule ids of one member's documents, each with the file it is
// in so an error can name both sides.
type set struct {
	ids map[string]string
}

func newSet() *set {
	return &set{ids: map[string]string{}}
}

// addAll indexes one document, filing a name that is already taken and going on
// with the names that are not.
func (s *set) addAll(doc *Document) []Problem {
	var problems []Problem
	for si := range doc.Sections {
		rules := doc.Sections[si].Rules
		for ri := range rules {
			// A rule with no id yet is not a duplicate of another: hergy gives
			// it one when it writes the file (d-4aab).
			if rules[ri].ID == "" {
				continue
			}
			if first, ok := s.ids[rules[ri].ID]; ok {
				problem := newProblem(doc.Path, si, ri, "id", 0,
					i18n.Errorf("error.rules_duplicate_id", doc.Path, rules[ri].ID, first))
				problem.ruleID = rules[ri].ID
				problems = append(problems, problem)
				continue
			}
			s.ids[rules[ri].ID] = doc.Path
		}
	}
	return problems
}
