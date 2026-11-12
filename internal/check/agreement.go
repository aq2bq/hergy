package check

import (
	"path/filepath"

	domainname "github.com/aq2bq/hergy/internal/domain/name"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
)

// AgreementInput is what one team's agreement check needs (C6). It mirrors
// RoleInput, but the document is the team's own agreement.md and the only
// readers are the members of this definition.
type AgreementInput struct {
	Catalog *catalog.Catalog // the team.yaml draft is already overlaid by the caller
	Name    string           // the definition name
	Place   string           // source.LayerLocal ("project") or source.LayerGlobal ("global")
	Text    string           // the agreement's YAML as the YAML tab types it
}

// Agreement answers the rules tab with the team definition's one check (d-a575):
// it lays the agreement's draft over the team.yaml the caller has already
// overlaid, checks the definition with check.Team, and shows the part of that
// answer that stands in agreement.md. The panel therefore counts the same
// findings as the row and the topology panel; nothing is checked a second way by
// the screen a person reached it through.
//
// The Markdown and the diff draw the draft. Turns shows the agreement's arrows
// judged against this team's turns (the team's own document, so this is the
// team's check); Readers shows who reads it, by name only (d-8bbf).
func Agreement(in AgreementInput) DocReport {
	if in.Catalog == nil {
		return DocReport{}
	}
	path := agreementPath(in.Catalog, in.Place, in.Name)
	report := proseReport(in.Catalog, path, in.Text)
	overlay := in.Catalog.With(path, in.Text)
	if def, err := overlay.Definition(in.Name, in.Place); err == nil {
		report.Readers = agreementReaders(def)
		report.TemplateChars = TeamSizeChars(overlay, def) - report.Markdown.Chars
		for _, finding := range teamSizeFinding(overlay, def) {
			report.Problems = append(report.Problems, RoleProblem{Sev: "warn", Field: "size", Text: finding.Text})
		}
	}
	report.Limit = TeamCharLimit
	return report
}

// agreementPath is where one layer keeps a team's own agreement.
func agreementPath(cat *catalog.Catalog, place, name string) string {
	return filepath.Join(cat.LayerDir(place), "teams", name, "agreement.md")
}

// agreementReaders is the members that read this agreement, by name only: who
// uses it, not how much each is handed (d-8bbf).
func agreementReaders(def *definition.Definition) []RoleReader {
	out := make([]RoleReader, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		out = append(out, RoleReader{Definition: def.Name.String(), Member: domainname.PreviewName(def.Name.String(), member.Name.String())})
	}
	return out
}
