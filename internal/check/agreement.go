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

// Agreement checks one team's agreement.yaml and returns the same shape as a
// role document's check (C6), so the two screens share one part.
//
// The caller overlays the ops-applied team.yaml, so the arrows are judged
// against the definition's draft turns: the turns the agreement's rules write
// are compared with the turns overlay.Definition reads. The document is read as
// if it were saved (catalog.With), so the members' amounts and the cross-document
// checks see the draft. A definition that does not read still answers with the
// document's own problems, the Markdown and the diff.
func Agreement(in AgreementInput) DocReport {
	cat := in.Catalog
	if cat == nil {
		return DocReport{}
	}
	path := agreementPath(cat, in.Place, in.Name)
	saved := readRoleDoc(cat, path)

	text := agreementText(in)
	doc, problems := rules.ReadAll(text, path)
	if doc == nil {
		doc = &rules.Document{Path: path}
	}
	lines := rules.LinesOf(text, path)
	overlay := cat.With(path, text)

	report := DocReport{
		Markdown: RoleMarkdown{Lines: doc.PreviewLines(), Chars: rules.CharCount(doc.Markdown())},
		Diff:     roleDiff(saved, doc),
	}
	for _, p := range problems {
		report.Problems = append(report.Problems, RoleProblem{
			Sev: p.Sev, Section: p.Section, Rule: p.Rule, Field: p.Field,
			Line: p.Line, Text: p.Text,
		})
	}

	def, err := overlay.Definition(in.Name, in.Place)
	if err != nil {
		report.Problems = orderProblems(dedupeProblems(report.Problems))
		return report
	}

	index := ruleIndex(doc)
	report.Problems = append(report.Problems, roleModelProblems(path, doc, index, lines)...)
	report.Problems = append(report.Problems, roleDocProblems(overlay, def, "", path, index, lines)...)
	report.Problems = append(report.Problems, roleTurnProblems(overlay, def, path, index, lines)...)
	size := SizeOf(agreementStarts(overlay, def), overlay.Reader())
	report.Problems = append(report.Problems, agreementSizeProblems(def.Name, size)...)
	report.Turns = agreementTurns(overlay, def, path, index)
	report.Readers = roleReaders(def.Name, size)
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

// agreementStarts is every member of one definition as the frames the size is
// counted from: each member reads its own role document, the agreement every
// team shares, and this team's agreement. It is roleStarts for the team's own
// agreement, where the readers are all the members rather than one role's.
// The prompt is not built: a definition whose prompt cannot be built still gives
// its readers' sizes.
func agreementStarts(cat *catalog.Catalog, def *definition.Definition) []definition.Start {
	var shared []source.Doc
	if doc, has := cat.SharedAgreement(); has {
		shared = append(shared, doc)
	}
	starts := make([]definition.Start, 0, len(def.Graph.Members))
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		read := make([]source.Doc, 0, len(shared)+1)
		read = append(read, shared...)
		if def.HasAgreement {
			read = append(read, def.Agreement)
		}
		start := definition.Start{
			Name:    def.Name + "-" + member.Name,
			Short:   member.Name,
			Role:    member.Role,
			Profile: member.Profile,
			Read:    read,
		}
		if doc, ok := cat.Role(member.Role); ok {
			start.HasRole = true
			start.RoleDoc = doc
		}
		starts = append(starts, start)
	}
	return starts
}

// agreementSizeProblems is the size warning of every member that reads the
// team's agreement and is over a provisional limit (C7). The sentence is
// validate's own, with no definition name at its head: a team's agreement
// belongs to one definition, so the panel it stands in already says which one
// (spec-teams 1.7).
func agreementSizeProblems(definitionName string, size Size) []RoleProblem {
	var out []RoleProblem
	for _, member := range size.Members {
		if member.Size.OverMemberRules() {
			out = append(out, RoleProblem{
				Sev: rules.SevWarning, Section: -1, Rule: -1,
				Definition: definitionName,
				Text:       MemberRulesWarning(member.Name, member.Size.Rules),
			})
		}
		if member.Size.OverMemberChars() {
			out = append(out, RoleProblem{
				Sev: rules.SevWarning, Section: -1, Rule: -1,
				Definition: definitionName,
				Text:       MemberCharsWarning(member.Name, member.Size.Chars),
			})
		}
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
