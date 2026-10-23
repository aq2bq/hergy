package check

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
	"github.com/aq2bq/hergy/internal/topology"
)

// RoleInput is what one role document's check needs (C6): the catalog to read
// through, where the document stands, the draft a screen holds, and the name it
// is saved under.
type RoleInput struct {
	// Catalog is what the check reads through. A screen hands a catalog with
	// whatever else it is editing laid over it (C5, C3b); the role document
	// itself is named by Place and Name here, so the caller leaves it out.
	Catalog *catalog.Catalog
	// Place is the layer the document stands in: source.LayerLocal ("project")
	// or source.LayerGlobal ("global").
	Place string
	// Name is the role's name: the file's name without .yaml.
	Name string
	// Draft is the document as a screen has it before it is written: the role
	// tab's fields, read as a document. It is written back out with Emit to be
	// checked as if it were saved, and it draws the Markdown preview. When it is
	// empty, Text is the draft instead.
	Draft *rules.Document
	// Text is the YAML the YAML tab is typing, read as the draft: a problem it
	// finds carries the YAML line, so the tab can mark the line. It is read when
	// Draft is empty; an empty Text is a document with no lines, which is the
	// error.file_empty a person deleting the whole file gets.
	Text string
	// Original is the name the document is saved under when the draft renames
	// it; empty means the name has not changed. It is where the saved file is
	// read for the diff, and the file the draft replaces.
	Original string
}

// RoleProblem is one thing wrong with one role document, as a screen shows it
// (C6). It carries the document's own C2 problems, the C3 problems of each
// definition that reads it, the turn warnings, the model-name warnings and the
// size warnings, told apart by Definition where a definition is what it is
// about. Section and Rule are the rule's place in the document, or -1; Line is
// the YAML line (0 when there is none), which the YAML tab marks.
type RoleProblem struct {
	Sev        string `json:"sev"`
	Section    int    `json:"section"`
	Rule       int    `json:"rule"`
	Field      string `json:"field"`
	Definition string `json:"definition"`
	Line       int    `json:"line"`
	Text       string `json:"text"`
}

// RoleMarkdown is the Markdown a member reads from the document, cut into lines
// as a screen draws them (C1), with the character count of the whole text.
type RoleMarkdown struct {
	Lines []rules.MDLine `json:"lines"`
	Chars int            `json:"chars"`
}

// RoleDiff is how the Markdown a screen shows changed from the saved document
// (C6): the indexes of the lines added, the indexes a removed saved line stood
// before, and whether a removed saved line stood after the last line.
type RoleDiff struct {
	Added         []int `json:"added"`
	RemovedBefore []int `json:"removedBefore"`
	RemovedAtEnd  bool  `json:"removedAtEnd"`
}

// RoleTurn is one arrow a rule's text writes, judged for one definition that
// reads the document (C6): the arrow as a person reads it, the definition, and
// whether the definition's turns: holds it. Every definition that reads the
// document gives one entry for every arrow, whether it holds or not.
type RoleTurn struct {
	Section    int    `json:"section"`
	Rule       int    `json:"rule"`
	Arrow      string `json:"arrow"`
	Definition string `json:"definition"`
	OK         bool   `json:"ok"`
}

// RoleReader is one member that reads the document: the team definition it
// plays the role in and its name. It says who uses the role (a reason to take it
// to another project), not how much it is handed: the amount is what the prompt
// preview shows, and no team-dependent amount is part of this check (d-8bbf).
type RoleReader struct {
	Definition string `json:"definition"`
	Member     string `json:"member"`
}

// DocReport is what one rule document's check found: the document's own
// problems, the Markdown a screen draws (C1), how that changed from the saved
// document, the arrows the document's rules write (for a team's agreement, whose
// turns are the team's), and the members that read it by name. It is the shape
// both a role document's check (Role) and a team's agreement check (Agreement,
// spec-teams C6) answer with, so the two screens share one part (spec-teams 7).
type DocReport struct {
	Problems []RoleProblem `json:"problems"`
	Markdown RoleMarkdown  `json:"markdown"`
	Diff     RoleDiff      `json:"diff"`
	Turns    []RoleTurn    `json:"turns"`
	Readers  []RoleReader  `json:"readers"`
	// Limit is the provisional size budget of this unit, in characters, for a
	// screen that shows the document's size against it (d-8bbf).
	Limit int `json:"limit,omitempty"`
}

// RoleReport is the old name of DocReport, kept so a caller that reads one role
// document keeps its name (spec-teams 7).
type RoleReport = DocReport

// Role checks one role document the way the role screen needs it (C6, d-a575):
// the document's own problems read to the end (C2), an id or a report name it
// repeats, the model names its rules write (C8), the size of the part of it that
// goes into the prompt (d-00a9), and the Markdown a member reads, with how that
// changed from the saved document.
//
// The answer does not depend on which team definitions use the role (d-8bbf): a
// rule that names a team's turn is the team definition's check (TurnFindings),
// and the documents one member reads are checked together there
// (betweenFindings). Readers carries only who uses the role, by name, apart from
// the check.
//
// The draft is checked as if it were saved: the document is read through a
// catalog that carries the draft in its place (C5), so the readers see it.
func Role(in RoleInput) DocReport {
	cat := in.Catalog
	if cat == nil {
		return DocReport{}
	}
	path := rolePath(cat, in.Place, in.Name)
	savedName := in.Original
	if savedName == "" {
		savedName = in.Name
	}
	savedPath := rolePath(cat, in.Place, savedName)

	// The saved document is read before the draft is laid over the catalog, so
	// the diff is against what is on disk (C6).
	saved := readRoleDoc(cat, savedPath)

	// The text the draft stands for: the role tab's document written back out,
	// or the YAML tab's own text. ReadAll gathers every problem and, with it,
	// the document as far as it reads (C2). The document is parsed under the path
	// a person reads it by, so a sentence that names it names it the way the
	// panel's own head does.
	text := draftText(in)
	display := cat.Display(path)
	doc, problems := rules.ReadAll(text, display)
	if doc == nil {
		doc = &rules.Document{Path: display}
	}
	lines := rules.LinesOf(text, display)

	// Everything else reads through a catalog that carries the draft (C5).
	overlay := cat.With(path, text)
	if savedPath != path {
		overlay = overlay.Without(savedPath)
	}
	if in.Place == source.LayerGlobal {
		// The project's own document of this name wins the one way a name is
		// looked up (K1); hide it so this document is the one read.
		overlay = overlay.Without(rolePath(cat, source.LayerLocal, in.Name))
	}

	report := DocReport{
		Markdown: RoleMarkdown{Lines: doc.PreviewLines(), Chars: rules.CharCount(doc.Markdown())},
		Diff:     roleDiff(saved, doc),
		Limit:    RoleCharLimit,
	}
	for _, p := range problems {
		report.Problems = append(report.Problems, RoleProblem{
			Sev: p.Sev, Section: p.Section, Rule: p.Rule, Field: p.Field,
			Line: p.Line, Text: withoutOwnName(p.Text, display),
		})
	}
	index := ruleIndex(doc)
	for _, p := range localRuleProblems(doc) {
		report.Problems = append(report.Problems, RoleProblem{
			Sev: p.Sev, Section: p.Section, Rule: p.Rule, Field: p.Field,
			Line: lines.At(p.Section, p.Rule, p.Field),
			Text: withoutOwnName(p.Text, display),
		})
	}
	report.Problems = append(report.Problems, roleModelProblems(display, doc, index, lines)...)
	report.Problems = append(report.Problems, roleSizeProblems(rules.CharCount(doc.Markdown()))...)

	for _, def := range roleDefinitions(overlay, in.Name) {
		report.Readers = append(report.Readers, roleReaders(def, in.Name)...)
	}
	report.Problems = orderProblems(dedupeProblems(report.Problems))
	return report
}

// orderProblems gives every problem the line a screen points at and puts them in
// the file's order (spec-roles 6 C6). A problem about the file as a whole — the
// amount a member reads, the title, the revision — stands on the first line, the
// way the Markdown preview marks the first line.
func orderProblems(problems []RoleProblem) []RoleProblem {
	for i := range problems {
		if problems[i].Line == 0 {
			problems[i].Line = 1
		}
	}
	sort.SliceStable(problems, func(i, j int) bool { return problems[i].Line < problems[j].Line })
	return problems
}

// draftText is the YAML the draft stands for: the role tab's document written
// back out in hergy's one format, or the YAML tab's own text. With neither, the
// text is empty, which is the file a person emptied.
func draftText(in RoleInput) string {
	if in.Draft != nil {
		if text, err := rules.Emit(in.Draft); err == nil {
			return text
		}
	}
	return in.Text
}

// rolePath is where one layer keeps a role document.
func rolePath(cat *catalog.Catalog, place, name string) string {
	return filepath.Join(cat.LayerDir(place), "roles", name+".yaml")
}

// readRoleDoc is one role document as it is on disk, or nil when it is not there
// or does not read. It is what the diff is against: a document that does not read
// has no Markdown to compare with.
func readRoleDoc(cat *catalog.Catalog, path string) *rules.Document {
	text, err := cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return nil
	}
	parsed, err := rules.Parse(text, path)
	if err != nil {
		return nil
	}
	return parsed
}

// roleDefinitions is every readable team definition that plays one role, once
// each, in the order a name is looked up (the project's own first).
func roleDefinitions(cat *catalog.Catalog, role string) []*definition.Definition {
	seen := map[string]bool{}
	var out []*definition.Definition
	for _, ref := range cat.RoleUsers(role) {
		if seen[ref.Definition] {
			continue
		}
		seen[ref.Definition] = true
		def, err := cat.Definition(ref.Definition, "")
		if err != nil {
			continue
		}
		out = append(out, def)
	}
	return out
}

// RoleIndex is the place of every rule id in a document, the first occurrence
// winning, so a finding's field can say where it stands.
type roleIndex struct {
	places map[string][2]int
}

// ruleIndex indexes one document's rule ids.
func ruleIndex(doc *rules.Document) roleIndex {
	index := roleIndex{places: map[string][2]int{}}
	for si := range doc.Sections {
		for ri := range doc.Sections[si].Rules {
			id := doc.Sections[si].Rules[ri].ID
			if id == "" {
				continue
			}
			if _, ok := index.places[id]; !ok {
				index.places[id] = [2]int{si, ri}
			}
		}
	}
	return index
}

// place is the section and rule a field names, or -1, -1 when it names no rule
// of the document (a document-level problem, or one whose rule is not there).
func (x roleIndex) place(field, path string) (int, int) {
	prefix := "rule:" + path + "#"
	if !strings.HasPrefix(field, prefix) {
		return -1, -1
	}
	if at, ok := x.places[strings.TrimPrefix(field, prefix)]; ok {
		return at[0], at[1]
	}
	return -1, -1
}

// roleModelProblems is one document's model-name warnings (C8): one per rule
// whose text names a model, in the written order.
func roleModelProblems(path string, doc *rules.Document, index roleIndex, lines rules.Lines) []RoleProblem {
	var out []RoleProblem
	for _, finding := range modelDocFindings(path, doc) {
		section, rule := index.place(finding.Field, path)
		out = append(out, RoleProblem{
			Sev: rules.SevWarning, Section: section, Rule: rule,
			Field: "text", Line: lines.At(section, rule, "text"), Text: finding.Text,
		})
	}
	return out
}

// parseRoleDoc reads one rule document through the catalog, or nil when it does
// not read. It is parsed under the path a person reads it by (Catalog.Display):
// the problems between the documents one member reads name a file each, and a
// screen draws those names under the panel's own head (C3, X1).
func parseRoleDoc(cat *catalog.Catalog, doc source.Doc) *rules.Document {
	text, err := cat.Reader().Read(doc)
	if err != nil {
		return nil
	}
	parsed, err := rules.Parse(text, cat.Display(doc.Path))
	if err != nil {
		return nil
	}
	return parsed
}

// edgeHeld is whether a definition's turns: holds an arrow: for "→" the one
// pair, and for "⇄" both ways. An arrow with an end that is no node of the graph
// is held by nothing.
func edgeHeld(def *definition.Definition, edge topology.Edge, members []topology.Member) bool {
	pairs := edgePairs(edge, members)
	if len(pairs) == 0 {
		return false
	}
	for _, pair := range pairs {
		if !graphHasTurn(def.Graph, pair[0], pair[1]) {
			return false
		}
	}
	return true
}

// roleReaders is the members that read the document, by the definition they
// play it in and their name: who uses the role, not how much each is handed
// (d-8bbf).
func roleReaders(def *definition.Definition, role string) []RoleReader {
	var out []RoleReader
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		if member.Role != role {
			continue
		}
		out = append(out, RoleReader{Definition: def.Name, Member: def.Name + "-" + member.Name})
	}
	return out
}

// dedupeProblems is the problems with an identical one shown once, so a C3
// problem two definitions read the same way (spec-roles 1.6) is one sentence
// beside the rule rather than one per definition. The first stands, with the
// definition it was found for.
func dedupeProblems(problems []RoleProblem) []RoleProblem {
	seen := make(map[string]bool, len(problems))
	out := make([]RoleProblem, 0, len(problems))
	for _, p := range problems {
		key := fmt.Sprintf("%s\x00%d\x00%d\x00%s\x00%s", p.Sev, p.Section, p.Rule, p.Field, p.Text)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

// roleDiff is how the Markdown a screen draws changed from the saved document
// (C6): the lines that are new, where a saved line was removed, and whether the
// removals stand after the last line.
func roleDiff(saved, draft *rules.Document) RoleDiff {
	if saved == nil {
		return RoleDiff{}
	}
	return lineDiff(mdTexts(saved.Lines()), mdTexts(draft.Lines()))
}

// LineDiff is how draft differs from saved, by line: the same alignment the
// Markdown preview uses, exported so the team screen's YAML preview marks the
// lines one edit would change the same way (spec-teams B7).
func LineDiff(saved, draft []string) RoleDiff { return lineDiff(saved, draft) }

// mdTexts is the text of the Markdown lines, for a diff by line.
func mdTexts(lines []rules.MDLine) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = line.Text
	}
	return out
}

// lineDiff is how draft differs from saved, by line: the indexes of the draft
// lines added, the indexes a removed saved line stood before (or RemovedAtEnd,
// for the ones after the last), found by aligning the two. It is what the
// Markdown preview marks (spec-roles 1.2.6).
func lineDiff(saved, draft []string) RoleDiff {
	common := lcsLengths(saved, draft)
	var diff RoleDiff
	i, j := 0, 0
	added := map[int]bool{}
	removed := map[int]bool{}
	for i < len(saved) && j < len(draft) {
		switch {
		case saved[i] == draft[j]:
			i++
			j++
		case common[i+1][j] >= common[i][j+1]:
			// A saved line is gone before draft[j].
			if j < len(draft) {
				removed[j] = true
			} else {
				diff.RemovedAtEnd = true
			}
			i++
		default:
			added[j] = true
			j++
		}
	}
	for ; i < len(saved); i++ {
		diff.RemovedAtEnd = true
	}
	for ; j < len(draft); j++ {
		added[j] = true
	}
	// A removed saved line whose place a draft line took is marked on the first
	// line after the added run (spec-roles 1.2.6): the mock draws the strike
	// under the block that replaced it, not on the same line as the new text.
	for j := range draft {
		if !removed[j] || !added[j] {
			continue
		}
		n := j
		for n < len(draft) && added[n] {
			n++
		}
		delete(removed, j)
		if n < len(draft) {
			removed[n] = true
		} else {
			diff.RemovedAtEnd = true
		}
	}
	for j := range draft {
		if added[j] {
			diff.Added = append(diff.Added, j)
		}
		if removed[j] {
			diff.RemovedBefore = append(diff.RemovedBefore, j)
		}
	}
	return diff
}

// lcsLengths is the table of the longest common subsequence of two line lists:
// common[i][j] is the length for saved[i:] and draft[j:].
func lcsLengths(saved, draft []string) [][]int {
	common := make([][]int, len(saved)+1)
	for i := range common {
		common[i] = make([]int, len(draft)+1)
	}
	for i, s := range slices.Backward(saved) {
		for j := len(draft) - 1; j >= 0; j-- {
			switch {
			case s == draft[j]:
				common[i][j] = common[i+1][j+1] + 1
			case common[i+1][j] >= common[i][j+1]:
				common[i][j] = common[i+1][j]
			default:
				common[i][j] = common[i][j+1]
			}
		}
	}
	return common
}
