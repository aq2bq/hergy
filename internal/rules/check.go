package rules

import (
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// Check refuses the breaks that span the documents one member reads: a rule id
// that stands twice, a report name that stands twice, a report a rule points at
// that is not there, and a derived rule that points at a rule that is not there.
// The documents are the ones that go into that member's prompt, so the same two
// documents are checked once for each member that reads them.
func Check(docs []*Document) error {
	set := newSet()
	for _, doc := range docs {
		if err := set.add(doc); err != nil {
			return err
		}
	}
	return set.refs(docs)
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

// add indexes one document, refusing a name that is already taken.
func (s *set) add(doc *Document) error {
	for _, report := range doc.Reports {
		if first, ok := s.reports[report.Name]; ok {
			return i18n.Errorf("error.rules_duplicate_report", doc.Path, report.Name, first.path)
		}
		index := &reportIndex{path: doc.Path, fields: map[string]bool{}}
		for _, field := range report.Fields {
			index.fields[field.Name] = true
		}
		s.reports[report.Name] = index
	}
	for _, rule := range doc.Rules() {
		if first, ok := s.ids[rule.ID]; ok {
			return i18n.Errorf("error.rules_duplicate_id", doc.Path, rule.ID, first)
		}
		s.ids[rule.ID] = doc.Path
	}
	return nil
}

// refs checks the pointers of every rule: a report, and, for a rule derived
// from others, the rules it came from.
func (s *set) refs(docs []*Document) error {
	for _, doc := range docs {
		for _, rule := range doc.Rules() {
			if rule.Report != "" && !s.hasReport(rule.Report) {
				return i18n.Errorf("error.rules_report_missing", doc.Path, rule.ID, rule.Report)
			}
			if err := s.checkDerived(doc.Path, rule); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *set) checkDerived(path string, rule Rule) error {
	for _, origin := range rule.Origin {
		if origin.Kind != KindDerived {
			continue
		}
		for _, ref := range origin.Refs {
			if _, ok := s.ids[ref]; !ok {
				return i18n.Errorf("error.rules_derived_missing", path, rule.ID, ref)
			}
		}
	}
	return nil
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
