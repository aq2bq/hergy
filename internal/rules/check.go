package rules

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Check refuses the breaks that span the documents one member reads: a rule id
// that stands twice, a report name that stands twice, a report a rule points at
// that is not there, and a derived rule that points at a rule that is not there.
// The documents are the ones that go into that member's prompt, so the same two
// documents are checked once for each member that reads them.
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
	problems = append(problems, set.refProblems(docs)...)
	return problems
}

// set indexes the ids and the report names of one member's documents, each with
// the file it is in so an error can name both sides.
type set struct {
	ids     map[string]string
	reports map[string]*reportIndex
}

type reportIndex struct {
	path   string
	fields map[string]bool
}

func newSet() *set {
	return &set{ids: map[string]string{}, reports: map[string]*reportIndex{}}
}

// addAll indexes one document, filing a name that is already taken and going on
// with the names that are not.
func (s *set) addAll(doc *Document) []Problem {
	var problems []Problem
	for _, report := range doc.Reports {
		if first, ok := s.reports[report.Name]; ok {
			problems = append(problems, newProblem(doc.Path, -1, -1, "reports."+report.Name, 0,
				i18n.Errorf("error.rules_duplicate_report", doc.Path, report.Name, first.path)))
			continue
		}
		index := &reportIndex{path: doc.Path, fields: map[string]bool{}}
		for _, field := range report.Fields {
			index.fields[field.Name] = true
		}
		s.reports[report.Name] = index
	}
	for si := range doc.Sections {
		rules := doc.Sections[si].Rules
		for ri := range rules {
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

// refProblems checks the pointers of every rule: a report, and, for a rule
// derived from others, the rules it came from.
func (s *set) refProblems(docs []*Document) []Problem {
	var problems []Problem
	for _, doc := range docs {
		for si := range doc.Sections {
			rules := doc.Sections[si].Rules
			for ri := range rules {
				rule := &rules[ri]
				if rule.Report != "" && !s.hasReport(rule.Report) {
					problem := newProblem(doc.Path, si, ri, "report", 0,
						i18n.Errorf("error.rules_report_missing", doc.Path, rule.ID, rule.Report))
					problem.ruleID = rule.ID
					problems = append(problems, problem)
				}
				for oi := range rule.Origin {
					origin := &rule.Origin[oi]
					if origin.Kind != KindDerived {
						continue
					}
					for _, ref := range origin.Refs {
						if _, ok := s.ids[ref]; ok {
							continue
						}
						problem := newProblem(doc.Path, si, ri, fmt.Sprintf("origin.%d", oi), 0,
							i18n.Errorf("error.rules_derived_missing", doc.Path, rule.ID, ref))
						problem.ruleID = rule.ID
						problems = append(problems, problem)
					}
				}
			}
		}
	}
	return problems
}

// hasReport reports whether "name.field" names a field of a report the member's
// documents hold.
func (s *set) hasReport(ref string) bool {
	name, field, ok := strings.Cut(ref, ".")
	if !ok {
		return false
	}
	index, ok := s.reports[name]
	if !ok {
		return false
	}
	return index.fields[field]
}
