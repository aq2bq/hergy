package check

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/topology"
)

// AgreementInput is what one team's agreement check needs (C6). It mirrors
// RoleInput, but the document is the team's own agreement.yaml and the only
// readers are the members of this definition.
type AgreementInput struct {
	Catalog *catalog.Catalog // the team.yaml draft is already overlaid by the caller
	Name    string           // the definition name
	Place   string           // source.LayerLocal ("project") or source.LayerGlobal ("global")
	Draft   *rules.Document  // the agreement as the screen has it, or nil with Text
	Text    string           // the agreement's YAML as the YAML tab types it
}

// Agreement answers the rules tab with the team definition's one check (d-a575):
// it lays the agreement's draft over the team.yaml the caller has already
// overlaid, checks the definition with check.Team, and shows the part of that
// answer that stands in agreement.yaml. The panel therefore counts the same
// findings as the row and the topology panel; nothing is checked a second way by
// the screen a person reached it through.
//
// The Markdown and the diff draw the draft. Turns shows the agreement's arrows
// judged against this team's turns (the team's own document, so this is the
// team's check); Readers shows who reads it, by name only (d-8bbf).
func Agreement(in AgreementInput) DocReport {
	cat := in.Catalog
	if cat == nil {
		return DocReport{}
	}
	path := agreementPath(cat, in.Place, in.Name)
	saved := readRoleDoc(cat, path)

	text := agreementText(in)
	display := cat.Display(path)
	doc, _ := rules.ReadAll(text, display)
	if doc == nil {
		doc = &rules.Document{Path: display}
	}
	lines := rules.LinesOf(text, display)
	overlay := cat.With(path, text)

	report := DocReport{
		Markdown: RoleMarkdown{Lines: doc.PreviewLines(), Chars: rules.CharCount(doc.Markdown())},
		Diff:     roleDiff(saved, doc),
	}
	index := ruleIndex(doc)
	for _, finding := range Team(TeamInput{Catalog: overlay, Name: in.Name, Layer: in.Place}).Problems {
		// Only the findings that stand in this file: a role document's rule, the
		// team figure's fields and the shared agreement are the other screens'
		// parts of the same check.
		if finding.Doc != display {
			continue
		}
		section, rule := index.place(finding.Field, display)
		sev := rules.SevWarning
		if finding.Stops {
			sev = rules.SevError
		}
		report.Problems = append(report.Problems, RoleProblem{
			Sev: sev, Section: section, Rule: rule, Field: finding.At,
			Line: lines.At(section, rule, finding.At),
			Text: withoutOwnName(finding.Text, display),
		})
	}

	def, err := overlay.Definition(in.Name, in.Place)
	if err == nil {
		report.Turns = agreementTurns(overlay, def, display, index)
		report.Readers = agreementReaders(def)
	}
	report.Problems = orderProblems(dedupeProblems(report.Problems))
	return report
}

// agreementPath is where one layer keeps a team's own agreement.
func agreementPath(cat *catalog.Catalog, place, name string) string {
	return filepath.Join(cat.LayerDir(place), "teams", name, "agreement.yaml")
}

// agreementText is the YAML the draft stands for: the panel's document written
// back out, or the YAML tab's own text. With neither, the text is empty, which is
// the file a person emptied.
func agreementText(in AgreementInput) string {
	if in.Draft != nil {
		if text, err := rules.Emit(in.Draft); err == nil {
			return text
		}
	}
	return in.Text
}

// agreementReaders is the members that read this agreement, by name only: who
// uses it, not how much each is handed (d-8bbf).
func agreementReaders(def *definition.Definition) []RoleReader {
	out := make([]RoleReader, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		out = append(out, RoleReader{Definition: def.Name, Member: def.Name + "-" + member.Name})
	}
	return out
}

// agreementTurns is every arrow of this agreement's rule texts, judged for this
// definition: whether the definition's draft turns hold it. It is roleTurns for
// the team's own agreement, where the document under check is the agreement
// itself rather than a role document.
func agreementTurns(cat *catalog.Catalog, def *definition.Definition, path string, index roleIndex) []RoleTurn {
	parsed := parseRoleDoc(cat, source.Doc{Path: path})
	if parsed == nil {
		return nil
	}
	members := topologyMembers(def)
	doc := topology.Doc{Key: path, Label: parsed.Title}
	for _, rule := range parsed.Rules() {
		doc.Rules = append(doc.Rules, topology.Rule{ID: rule.ID, Text: rule.Text})
	}
	reading := topology.Read(members, []topology.Doc{doc})
	var out []RoleTurn
	for _, edge := range reading.Edges {
		section, rule := index.place("rule:"+path+"#"+edge.RuleID, path)
		out = append(out, RoleTurn{
			Section: section, Rule: rule, Arrow: edgeText(edge, members),
			Definition: def.Name, OK: edgeHeld(def, edge, members),
		})
	}
	for _, unread := range reading.Unread {
		section, rule := index.place("rule:"+path+"#"+unread.RuleID, path)
		out = append(out, RoleTurn{
			Section: section, Rule: rule, Arrow: unread.Text,
			Definition: def.Name, OK: false,
		})
	}
	return out
}
