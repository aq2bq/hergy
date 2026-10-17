package web

import (
	"errors"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// readDefinition reads one team definition by name, the one way a name is looked
// up (the project's first). The layer is never named: this package does not know
// one (I11).
func (s *Server) readDefinition(name string) (*definition.Definition, error) {
	return s.cat.Read(name)
}

// findings is what hergy validate would say about one definition, as the list
// of 〔field, sentence〕 the screens place beside the field they belong to. The
// call passes no layer, so the judgement is the one the screens show (spec/37
// T1).
func (s *Server) findings(name string) []check.Finding {
	return check.Definition(check.Input{Catalog: s.cat, Name: name})
}

// breakLine writes one broken settings file as one line: where it is, as a
// person writes it, and why.
func (s *Server) breakLine(path string, err error) string {
	return check.BreakLine(s.cat.Display, path, err)
}

// readErrorPath is the file a failed read names, or the fallback when the reason
// does not carry one.
func readErrorPath(err error, fallback string) string {
	var readErr *catalog.ReadError
	if errors.As(err, &readErr) && readErr.Path != "" {
		return readErr.Path
	}
	return fallback
}

// ruleDoc reads one rule document through the catalog's reader: the text the
// screens show is the text the model reads, so a draft laid over the catalog
// would be read here too.
func (s *Server) ruleDoc(doc source.Doc) (*rules.Document, error) {
	text, err := s.cat.Reader().Read(doc)
	if err != nil {
		return nil, err
	}
	return rules.Parse(text, doc.Path)
}

// runningTeams is the teams running on this machine, as the Port reports them.
// A herdr that cannot be read is one line in the activity, not an error that
// blanks the other screens.
func (s *Server) runningTeams() ([]effect.Running, error) {
	return s.port.Running()
}

// runningByDefinition is the running teams by the definition they were booted
// from, so the definitions list marks the ones in use.
func runningByDefinition(running []effect.Running) map[string][]effect.Running {
	out := map[string][]effect.Running{}
	for _, entry := range running {
		out[entry.Definition] = append(out[entry.Definition], entry)
	}
	return out
}

// fieldTexts is every sentence about one field, in the order the check returned
// them.
func fieldTexts(findings []check.Finding, field string) []string {
	var out []string
	for _, finding := range findings {
		if finding.Field == field {
			out = append(out, finding.Text)
		}
	}
	return out
}

// stops is the first sentence that stops a save, and whether there was one.
func stops(findings []check.Finding, field string) (string, bool) {
	for _, finding := range findings {
		if finding.Stops && (field == "" || finding.Field == field) {
			return finding.Text, true
		}
	}
	return "", false
}
