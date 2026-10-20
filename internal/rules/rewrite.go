package rules

import (
	"sort"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// This file writes a whole document from a screen's draft: every value the draft
// changed is written on the lines it already stands on, and every line the draft
// did not touch — a blank line, a comment, the quoting a person wrote — comes
// back byte for byte (spec-roles 1.2.9, 6 C4). A screen edits many places at
// once (a title, a section key, several rules, and a rule moved up), so the
// write is planned as the line edits it is made of and applied from the bottom
// of the file up, where a later edit cannot move an earlier one's lines.

// Place is where one rule of the saved document stands: its section, and its
// place within that section. A place whose Section is below zero is a rule the
// saved document did not hold — one the draft adds.
type Place struct {
	Section int
	Rule    int
}

// Draft is a screen's draft of a document (C4): the values as the person wrote
// them, and, for each rule of Doc, where that rule stood in the saved document
// (From is as long as Doc.Rules(), in the same order). A rule whose place says
// Section below zero is new; a rule whose place names another section was moved.
type Draft struct {
	Doc  *Document
	From []Place
}

// Rewrite writes the saved document (text, as saved holds it) the way the draft
// means it: the title and revision, a section's own keys, and the rules — each
// rewritten where it stands, one added, one taken out, or one moved. A rule that
// only moved keeps its lines, shifted to where its new section writes them. The
// written text is read back and compared with the draft (sameValuesAs); a value
// that did not land where the draft means it is refused with error.edit_shape,
// and so is a shape a line write cannot place (a flow sequence, an anchor). A
// draft whose values are the ones the file already holds is not written at all:
// the saved text comes back byte for byte.
func Rewrite(text []byte, saved *Document, draft Draft) ([]byte, error) {
	if draft.Doc == nil {
		return nil, i18n.Errorf("error.edit_shape", pathOf(saved), "draft")
	}
	path := draft.Doc.Path
	if path == "" {
		path = pathOf(saved)
	}
	root, err := readRoot(text, path)
	if err != nil {
		return nil, err
	}
	src := yamledit.New(text, path)
	ops, err := planRewrite(src, root, saved, draft, path)
	if err != nil {
		return nil, err
	}
	if len(ops) == 0 {
		// Nothing to write. The draft may still differ from the file (a value
		// hergy does not write, such as a change to the reports block): that is
		// caught here rather than written.
		if err := sameValuesAs(draft.Doc, text, path); err != nil {
			return nil, err
		}
		return text, nil
	}
	applyOps(src, ops)
	out := src.Bytes()
	if err := sameValuesAs(draft.Doc, out, path); err != nil {
		return nil, err
	}
	return out, nil
}

// pathOf is the path a document was read from, or "" when there is none.
func pathOf(doc *Document) string {
	if doc == nil {
		return ""
	}
	return doc.Path
}

// op is one line edit of a rewrite, placed on the file's own lines (1-based).
// The edits are applied from the bottom of the file up, so the line numbers
// yaml.v3 gave stay valid: an edit below one already applied is not moved by it.
type op struct {
	kind  opKind
	line  int // the line the edit is placed at; for opInsert, the line it goes after
	to    int // the last line replaced or removed (opReplace, opRemove)
	lines []string
}

type opKind int

const (
	opReplace     opKind = iota // the lines line..to become lines
	opRemove                    // the lines line..to are taken out
	opInsert                    // lines are written after line
	opReplaceLine               // the line line becomes lines
)

// applyOps applies the edits from the bottom of the file up, so an edit's own
// line numbers, taken from the tree read at the start, still name the lines the
// edit means after the edits below it have moved.
func applyOps(src *yamledit.Source, ops []op) {
	sort.SliceStable(ops, func(i, j int) bool { return ops[i].line > ops[j].line })
	for _, o := range ops {
		switch o.kind {
		case opReplace:
			src.Replace(o.line, o.to, o.lines)
		case opRemove:
			src.Remove(o.line, o.to)
		case opInsert:
			src.Insert(o.line, o.lines...)
		case opReplaceLine:
			src.ReplaceLine(o.line, o.lines)
		}
	}
}

// plan is what a rewrite is worked out on: the file's source, its tree, the
// saved values and the draft, and the places the draft's rules came from.
type plan struct {
	src   *yamledit.Source
	root  *yaml.Node
	saved []Section
	// sections is the file's sections: a node, so a rule moved out of a
	// section can be found where it stood.
	sections *yaml.Node
	draft    Draft
	path     string
	// sectionOf is the draft section each draft rule is in, rule by rule, in
	// the order the draft writes them.
	sectionOf []int
	// claimed says, for a saved rule (its section and its place), which draft
	// rule names it as its own place.
	claimed map[[2]int]int
}

// planRewrite plans the line edits that write the draft over the saved document.
func planRewrite(src *yamledit.Source, root *yaml.Node, saved *Document, draft Draft, path string) ([]op, error) {
	want := draft.Doc
	sectionsNode := yamledit.MappingValue(root, "sections")
	if sectionsNode == nil || sectionsNode.Kind != yaml.SequenceNode || sectionsNode.Style&yaml.FlowStyle != 0 {
		return nil, i18n.Errorf("error.edit_shape", path, "sections")
	}
	var savedSections []Section
	if saved != nil {
		savedSections = saved.Sections
	}
	if len(savedSections) != len(sectionsNode.Content) {
		return nil, i18n.Errorf("error.edit_shape", path, "sections")
	}

	p := &plan{src: src, root: root, saved: savedSections, sections: sectionsNode, draft: draft, path: path, claimed: map[[2]int]int{}}
	for ds, sec := range want.Sections {
		for range sec.Rules {
			p.sectionOf = append(p.sectionOf, ds)
		}
	}
	for gi, place := range draft.From {
		if place.Section >= 0 {
			p.claimed[[2]int{place.Section, place.Rule}] = gi
		}
	}

	ops, err := planTopScalars(src, root, want, path)
	if err != nil {
		return nil, err
	}
	target := sectionTargets(savedSections, want, draft)
	appendAt := 0
	if len(sectionsNode.Content) > 0 {
		appendAt = yamledit.NodeEndLine(sectionsNode.Content[len(sectionsNode.Content)-1])
	}
	for ds := range want.Sections {
		ss, ok := target[ds]
		if !ok {
			// A section the saved document did not hold: written whole, at the
			// end of the sections, the way Emit writes one.
			lines, err := sectionLines(want.Sections[ds], sectionsDash(sectionsNode), path)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op{kind: opInsert, line: appendAt, lines: lines})
			continue
		}
		more, err := p.planSection(ds, ss, sectionsNode.Content[ss])
		if err != nil {
			return nil, err
		}
		ops = append(ops, more...)
	}
	// A saved section no draft section names is taken out, rules and all.
	for ss := range savedSections {
		if !sectionUsed(target, ss) {
			node := sectionsNode.Content[ss]
			ops = append(ops, op{kind: opRemove, line: src.ItemStartLine(node), to: yamledit.NodeEndLine(node)})
		}
	}
	return ops, nil
}

// sectionTargets is the saved section each draft section is: the one most of the
// draft section's rules came from (their From places). A draft section whose
// rules came from nowhere — or which holds no rule at all — is new. A saved
// section two draft sections name is claimed by the first, so the other reads as
// new rather than moving the same place twice.
func sectionTargets(saved []Section, want *Document, draft Draft) map[int]int {
	out := map[int]int{}
	used := map[int]bool{}
	global := 0
	for ds := range want.Sections {
		votes := map[int]int{}
		for range want.Sections[ds].Rules {
			if global < len(draft.From) {
				if p := draft.From[global]; p.Section >= 0 && p.Section < len(saved) {
					votes[p.Section]++
				}
			}
			global++
		}
		best, bestN := -1, 0
		for ss := range saved {
			if n := votes[ss]; n > bestN {
				best, bestN = ss, n
			}
		}
		if best >= 0 && !used[best] {
			out[ds] = best
			used[best] = true
		}
	}
	return out
}

// sectionUsed reports whether a draft section names a saved section.
func sectionUsed(target map[int]int, ss int) bool {
	for _, got := range target {
		if got == ss {
			return true
		}
	}
	return false
}

// sectionsDash is the column a new section's "- " stands at, read from the
// sections the file already writes.
func sectionsDash(sections *yaml.Node) int {
	if len(sections.Content) > 0 {
		return sections.Content[len(sections.Content)-1].Column - 3
	}
	return 2
}

// planTopScalars plans the title and revision: the value on the line it stands
// on, a key added where hergy writes it (title after version, revision after
// title), or a key taken out when the draft empties it.
func planTopScalars(src *yamledit.Source, root *yaml.Node, want *Document, path string) ([]op, error) {
	var ops []op
	title := yamledit.MappingKeyNode(root, "title")
	switch {
	case title == nil && want.Title == "":
	case title == nil:
		ops = append(ops, op{kind: opInsert, line: keyEnd(root, "version"), lines: []string{
			scalarFieldLine("title", want.Title, yamledit.MappingIndent(root)),
		}})
	case want.Title == "":
		ops = append(ops, removeKeyOp(src, root, "title"))
	default:
		o, err := scalarReplaceOp(src, root, "title", want.Title, path)
		if err != nil {
			return nil, err
		}
		if o != nil {
			ops = append(ops, *o)
		}
	}

	revision := yamledit.MappingKeyNode(root, "revision")
	switch {
	case revision == nil && want.Revision == nil:
	case revision == nil:
		anchor := "title"
		if title == nil {
			anchor = "version"
		}
		ops = append(ops, op{kind: opInsert, line: keyEnd(root, anchor), lines: []string{
			yamledit.Spaces(yamledit.MappingIndent(root)) + "revision: " + strconv.Itoa(*want.Revision),
		}})
	case want.Revision == nil:
		ops = append(ops, removeKeyOp(src, root, "revision"))
	default:
		o, err := intReplaceOp(src, root, "revision", *want.Revision, path)
		if err != nil {
			return nil, err
		}
		if o != nil {
			ops = append(ops, *o)
		}
	}
	return ops, nil
}

// keyEnd is the last line a mapping key and its value live on, or 0 when the key
// is not there.
func keyEnd(root *yaml.Node, key string) int {
	keyNode := yamledit.MappingKeyNode(root, key)
	if keyNode == nil {
		return 0
	}
	end := yamledit.NodeEndLine(keyNode)
	if v := yamledit.MappingValue(root, key); v != nil {
		if e := yamledit.NodeEndLine(v); e > end {
			end = e
		}
	}
	return end
}

// removeKeyOp is the edit that takes a key, its value and the comment lines the
// file wrote above them out of a mapping.
func removeKeyOp(src *yamledit.Source, root *yaml.Node, key string) op {
	keyNode := yamledit.MappingKeyNode(root, key)
	end := yamledit.NodeEndLine(keyNode)
	if v := yamledit.MappingValue(root, key); v != nil {
		if e := yamledit.NodeEndLine(v); e > end {
			end = e
		}
	}
	return op{kind: opRemove, line: src.ItemStartLine(keyNode), to: end}
}

// scalarFieldLine is one key and a text value written the way hergy writes a new
// one, at an indent.
func scalarFieldLine(key, value string, indent int) string {
	return yamledit.Spaces(indent) + key + ": " + yamledit.ScalarLiteral(value, 0)
}

// scalarReplaceOp is the edit that writes a new value on the line a text key's
// value stands on, keeping the key, the indent, the quoting and any comment, or
// nil when the value the file holds is the one wanted.
func scalarReplaceOp(src *yamledit.Source, root *yaml.Node, key, value, path string) (*op, error) {
	v := yamledit.MappingValue(root, key)
	if v == nil {
		return nil, i18n.Errorf("error.edit_shape", path, key)
	}
	if v.Value == value {
		return nil, nil
	}
	if strings.ContainsAny(value, "\n\r") {
		return nil, i18n.Errorf("error.edit_shape", path, key)
	}
	if v.Anchor != "" || v.Kind != yaml.ScalarNode || v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return nil, i18n.Errorf("error.edit_shape", path, key)
	}
	line, ok := replaceScalarAt(src.Line(v.Line), v, yamledit.ScalarLiteral(value, v.Style), false)
	if !ok {
		return nil, i18n.Errorf("error.edit_shape", path, key)
	}
	return &op{kind: opReplaceLine, line: v.Line, lines: []string{line}}, nil
}

// intReplaceOp is the edit that writes a new number on the line an integer key's
// value stands on, written plain (a quoted number stays quoted), or nil when the
// number is the one the file holds.
func intReplaceOp(src *yamledit.Source, root *yaml.Node, key string, value int, path string) (*op, error) {
	v := yamledit.MappingValue(root, key)
	if v == nil {
		return nil, i18n.Errorf("error.edit_shape", path, key)
	}
	if v.Value == strconv.Itoa(value) {
		return nil, nil
	}
	if v.Kind != yaml.ScalarNode || v.Anchor != "" {
		return nil, i18n.Errorf("error.edit_shape", path, key)
	}
	literal := strconv.Itoa(value)
	if v.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0 {
		literal = yamledit.ScalarLiteral(literal, v.Style)
	}
	line, ok := replaceScalarAt(src.Line(v.Line), v, literal, false)
	if !ok {
		return nil, i18n.Errorf("error.edit_shape", path, key)
	}
	return &op{kind: opReplaceLine, line: v.Line, lines: []string{line}}, nil
}

// replaceScalarAt builds the line a scalar's value becomes: the line as written,
// with the new literal over the value's own bytes, so the key, the indent, the
// quoting and any comment after it stay.
func replaceScalarAt(line string, v *yaml.Node, literal string, flow bool) (string, bool) {
	start := yamledit.ByteColumn(line, v.Column)
	end, ok := yamledit.ScalarEnd(line, start, flow)
	if !ok {
		return "", false
	}
	return line[:start] + literal + line[end:], true
}

// sectionKeyOrder is the order hergy writes a section's own keys in, so a key
// that is added stands where Emit would write it. style is not here: hergy does
// not write one any more, and a file that still holds one loses it on the first
// save (d-f4f6).
var sectionKeyOrder = []string{"heading", "level", "intro"}

// placement is a run of rules written after one line, in order. Consecutive
// rules that go to the same place are written as one insert, so their order is
// the draft's.
type placement struct {
	anchor int
	lines  []string
}

// planSection plans one matched section: its own keys, the rules that stay, the
// rule that moves, the rule that is added, and the rule that is taken out. Rules
// are laid out in the draft's order, each after the saved line that comes before
// it.
func (p *plan) planSection(ds, ss int, node *yaml.Node) ([]op, error) {
	ops, err := p.planSectionKeys(node, p.draft.Doc.Sections[ds])
	if err != nil {
		return nil, err
	}
	rulesNode := yamledit.MappingValue(node, "rules")
	if rulesNode != nil && (rulesNode.Kind != yaml.SequenceNode || rulesNode.Style&yaml.FlowStyle != 0) {
		return nil, i18n.Errorf("error.edit_shape", p.path, "rules")
	}
	var items []*yaml.Node
	if rulesNode != nil {
		items = rulesNode.Content
	}
	if len(items) != len(p.saved[ss].Rules) {
		return nil, i18n.Errorf("error.edit_shape", p.path, "rules")
	}

	globals := p.globalRules(ds)
	if rulesNode == nil {
		// The section had no rules: the key and every rule are written together,
		// under the section's own keys, the way Emit writes them.
		return p.createRules(ops, node, globals)
	}
	dashCol := items[0].Column - 3
	headLine := yamledit.MappingKeyNode(node, "rules").Line

	// Which of the section's own rules stay on their own lines: the longest run
	// of them whose saved places rise in the draft's order (R4). A rule that
	// leaves the run moved, and every rule the draft puts here that the saved
	// section held elsewhere moved in; both have their own lines carried to
	// where the draft writes them, so the shape of a line and a comment travel
	// with the rule (spec-roles 1.2.9).
	kept := p.keptRules(ds, ss)
	moved := map[int]bool{}
	var groups []placement
	add := func(anchor int, lines []string) {
		if n := len(groups); n > 0 && groups[n-1].anchor == anchor {
			groups[n-1].lines = append(groups[n-1].lines, lines...)
			return
		}
		groups = append(groups, placement{anchor: anchor, lines: lines})
	}
	desiredAfter := headLine
	for _, gi := range globals {
		place := p.draft.From[gi]
		rule := p.ruleAt(gi)
		if place.Section == ss && kept[place.Rule] {
			if !sameRule(p.saved[ss].Rules[place.Rule], rule) {
				lines, err := ruleLines(rule, dashCol, p.path)
				if err != nil {
					return nil, err
				}
				ops = append(ops, op{kind: opReplace, line: items[place.Rule].Line, to: yamledit.NodeEndLine(items[place.Rule]), lines: lines})
			}
			desiredAfter = yamledit.NodeEndLine(items[place.Rule])
			continue
		}
		lines, removed, err := p.moveLines(place, rule, ss, node, items, dashCol)
		if err != nil {
			return nil, err
		}
		add(desiredAfter, lines)
		ops = append(ops, removed...)
		if place.Section == ss {
			moved[place.Rule] = true
		}
	}
	for _, g := range groups {
		ops = append(ops, op{kind: opInsert, line: g.anchor, lines: g.lines})
	}
	// A saved rule no draft rule of this section names is taken out — unless the
	// draft moved it within this section (handled above) or into another one
	// (that section writes it).
	ops = p.removeUnclaimed(ops, ds, ss, node, items, moved)
	return ops, nil
}

// keptRules is which of one draft section's own saved rules stay on their own
// lines: the longest increasing run of their saved places, read in the draft's
// order (R4). A draft that keeps every rule in place has every place in the run;
// one rule lifted over its neighbour leaves that neighbour out of the run, so
// the neighbour is the one whose lines move. Ties take the earlier place, so the
// rule a person moved down is the one written down.
func (p *plan) keptRules(ds, ss int) map[int]bool {
	var seq []int
	for _, gi := range p.globalRules(ds) {
		if place := p.draft.From[gi]; place.Section == ss {
			seq = append(seq, place.Rule)
		}
	}
	out := map[int]bool{}
	n := len(seq)
	if n == 0 {
		return out
	}
	dp := make([]int, n)
	prev := make([]int, n)
	best, bestAt := 0, 0
	for i := range n {
		dp[i], prev[i] = 1, -1
		for j := range i {
			if seq[j] < seq[i] && dp[j]+1 > dp[i] {
				dp[i], prev[i] = dp[j]+1, j
			}
		}
		if dp[i] > best {
			best, bestAt = dp[i], i
		}
	}
	for i := bestAt; i >= 0; i = prev[i] {
		out[seq[i]] = true
		if prev[i] < 0 {
			break
		}
	}
	return out
}

// moveLines is the lines a rule is written with where the draft puts it in a
// section: its own lines, shifted to the column that section writes its "- " at,
// when the values did not change, and Emit's lines when they did. A rule that
// moved inside this section also takes its old lines out; one that came in from
// another section is taken out by that section's own plan.
func (p *plan) moveLines(place Place, rule Rule, ss int, node *yaml.Node, items []*yaml.Node, dashCol int) ([]string, []op, error) {
	if place.Section < 0 {
		lines, err := ruleLines(rule, dashCol, p.path)
		return lines, nil, err
	}
	source := p.sections.Content[place.Section]
	sourceRules := yamledit.MappingValue(source, "rules")
	if sourceRules == nil || place.Rule >= len(sourceRules.Content) {
		lines, err := ruleLines(rule, dashCol, p.path)
		return lines, nil, err
	}
	lines, err := p.rulePlacementLines(sourceRules.Content[place.Rule], rule, dashCol)
	if err != nil {
		return nil, nil, err
	}
	if place.Section == ss {
		return lines, p.removeRuleOps(node, items, place.Rule), nil
	}
	return lines, nil, nil
}

// removeUnclaimed takes out every saved rule of a section that no draft rule of
// it names. A rule the draft moved within the section, or into another one, is
// left to the part that writes it.
func (p *plan) removeUnclaimed(ops []op, ds, ss int, node *yaml.Node, items []*yaml.Node, movedWithin map[int]bool) []op {
	for sr := range p.saved[ss].Rules {
		if movedWithin[sr] {
			continue
		}
		if gi, ok := p.claimed[[2]int{ss, sr}]; ok && p.sectionOf[gi] == ds {
			continue
		}
		ops = append(ops, p.removeRuleOps(node, items, sr)...)
	}
	return ops
}

// createRules writes a rules: key and every rule of a section that had none.
func (p *plan) createRules(ops []op, node *yaml.Node, globals []int) ([]op, error) {
	if node.Style&yaml.FlowStyle != 0 || len(node.Content) == 0 {
		return nil, i18n.Errorf("error.edit_shape", p.path, "sections")
	}
	indent := yamledit.MappingIndent(node)
	lines := []string{yamledit.Spaces(indent) + "rules:"}
	for _, gi := range globals {
		rule, err := ruleLines(p.ruleAt(gi), indent+2, p.path)
		if err != nil {
			return nil, err
		}
		lines = append(lines, rule...)
	}
	return append(ops, op{kind: opInsert, line: yamledit.NodeEndLine(node), lines: lines}), nil
}

// globalRules is the draft rule indices of one draft section, in order.
func (p *plan) globalRules(ds int) []int {
	var out []int
	for gi, at := range p.sectionOf {
		if at == ds {
			out = append(out, gi)
		}
	}
	return out
}

// ruleAt is one rule of the draft, by its place among every rule of the draft.
func (p *plan) ruleAt(gi int) Rule {
	at := 0
	for _, sec := range p.draft.Doc.Sections {
		for _, rule := range sec.Rules {
			if at == gi {
				return rule
			}
			at++
		}
	}
	return Rule{}
}

// rulePlacementLines is the lines one rule is written with when it moves within
// its section: its own lines, shifted to where its section writes them, when the
// values did not change; Emit's lines when they did.
func (p *plan) rulePlacementLines(item *yaml.Node, rule Rule, dashCol int) ([]string, error) {
	if !sameRule(p.ruleAtSaved(item), rule) {
		return ruleLines(rule, dashCol, p.path)
	}
	from := p.src.ItemStartLine(item)
	to := yamledit.NodeEndLine(item)
	shift := dashCol - (item.Column - 3)
	lines := make([]string, 0, to-from+1)
	for n := from; n <= to; n++ {
		lines = append(lines, shifted(p.src.Line(n), shift))
	}
	return lines, nil
}

// ruleAtSaved is the rule a saved rule's node wrote, found by its id, so a moved
// rule can tell whether the draft changed its values.
func (p *plan) ruleAtSaved(item *yaml.Node) Rule {
	id := yamledit.MappingValue(item, "id")
	if id == nil {
		return Rule{}
	}
	for _, sec := range p.saved {
		for _, rule := range sec.Rules {
			if rule.ID == id.Value {
				return rule
			}
		}
	}
	return Rule{}
}

// removeRuleOps is the edits that take one rule of a section out: the rule's own
// lines, the key that held it when it was the section's only rule, or the whole
// section when the section was nothing but its rules.
func (p *plan) removeRuleOps(node *yaml.Node, items []*yaml.Node, sr int) []op {
	item := items[sr]
	out := []op{{kind: opRemove, line: p.src.ItemStartLine(item), to: yamledit.NodeEndLine(item)}}
	switch {
	case len(items) > 1:
		return out
	case len(node.Content) > 2:
		// The item's lines and the key's line come out; they do not overlap, and
		// the item's lines go first (they are below the key).
		key := yamledit.MappingKeyNode(node, "rules")
		return append(out, op{kind: opRemove, line: p.src.ItemStartLine(key), to: yamledit.NodeEndLine(key)})
	default:
		return []op{{kind: opReplace, line: p.src.ItemStartLine(node), to: yamledit.NodeEndLine(node),
			lines: []string{yamledit.Spaces(node.Column-3) + "- {}"}}}
	}
}

// sameRule is whether two rules hold the same values.
func sameRule(a, b Rule) bool {
	if a.ID != b.ID || a.Text != b.Text || a.Why != b.Why || a.Note != b.Note || a.Report != b.Report {
		return false
	}
	if (a.SendWhy == nil) != (b.SendWhy == nil) {
		return false
	}
	if a.SendWhy != nil && *a.SendWhy != *b.SendWhy {
		return false
	}
	if (a.InPrompt == nil) != (b.InPrompt == nil) {
		return false
	}
	if a.InPrompt != nil && *a.InPrompt != *b.InPrompt {
		return false
	}
	if !sameStrings(a.Tests, b.Tests) {
		return false
	}
	if len(a.Origin) != len(b.Origin) {
		return false
	}
	for i := range a.Origin {
		if a.Origin[i].Kind != b.Origin[i].Kind || a.Origin[i].Date != b.Origin[i].Date || a.Origin[i].Note != b.Origin[i].Note || !sameStrings(a.Origin[i].Refs, b.Origin[i].Refs) {
			return false
		}
	}
	return true
}

// sameStrings is whether two string slices hold the same values in order.
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// planSectionKeys plans the heading, level and intro of one section — the value
// on the line it stands on, a key added where hergy writes it, or a key taken out
// when the draft empties it — and takes out a style: line a file still holds:
// nothing lays a section out but the one list now (d-f4f6).
func (p *plan) planSectionKeys(node *yaml.Node, want Section) ([]op, error) {
	var ops []op
	for _, key := range sectionKeyOrder {
		value := sectionKeyValue(key, want)
		keyNode := yamledit.MappingKeyNode(node, key)
		switch {
		case keyNode == nil && value == "":
		case keyNode == nil:
			o, err := p.addSectionKey(node, key, value)
			if err != nil {
				return nil, err
			}
			ops = append(ops, o)
		case value == "":
			ops = append(ops, removeKeyOp(p.src, node, key))
		default:
			o, err := p.sectionValueOp(node, key, value)
			if err != nil {
				return nil, err
			}
			if o != nil {
				ops = append(ops, *o)
			}
		}
	}
	if yamledit.MappingKeyNode(node, "style") != nil {
		ops = append(ops, removeKeyOp(p.src, node, "style"))
	}
	return ops, nil
}

// sectionKeyValue is one of a section's own keys as the draft writes it: an
// empty string means the key is not written (level with 0).
func sectionKeyValue(key string, section Section) string {
	switch key {
	case "heading":
		return section.Heading
	case "level":
		if section.Level == 0 {
			return ""
		}
		return strconv.Itoa(section.Level)
	case "intro":
		return section.Intro
	}
	return ""
}

// sectionValueOp is the edit that writes one new value on a section key's line,
// or nil when the value the file holds is the one wanted.
func (p *plan) sectionValueOp(node *yaml.Node, key, value string) (*op, error) {
	v := yamledit.MappingValue(node, key)
	if v == nil {
		return nil, i18n.Errorf("error.edit_shape", p.path, key)
	}
	if v.Value == value {
		return nil, nil
	}
	if v.Anchor != "" || v.Kind != yaml.ScalarNode || v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return nil, i18n.Errorf("error.edit_shape", p.path, key)
	}
	literal := value
	if key != "level" {
		literal = yamledit.ScalarLiteral(value, v.Style)
	}
	line, ok := replaceScalarAt(p.src.Line(v.Line), v, literal, false)
	if !ok {
		return nil, i18n.Errorf("error.edit_shape", p.path, key)
	}
	return &op{kind: opReplaceLine, line: v.Line, lines: []string{line}}, nil
}

// addSectionKey plans writing a section key the file does not hold, after the
// section key it follows in hergy's order, or, when it goes first and the
// section's first key stands on its "- " line, by putting it there.
func (p *plan) addSectionKey(node *yaml.Node, key, value string) (op, error) {
	indent := yamledit.MappingIndent(node)
	line := yamledit.Spaces(indent) + key + ": " + sectionLiteral(key, value)

	index := 0
	for i, k := range sectionKeyOrder {
		if k == key {
			index = i
			break
		}
	}
	for i := index - 1; i >= 0; i-- {
		if yamledit.MappingKeyNode(node, sectionKeyOrder[i]) != nil {
			return op{kind: opInsert, line: keyEnd(node, sectionKeyOrder[i]), lines: []string{line}}, nil
		}
	}
	first := firstKey(node)
	if first == nil {
		return op{}, i18n.Errorf("error.edit_shape", p.path, "sections")
	}
	if first.Line == node.Line {
		// The item's first key stands on the "- " line; the new key takes that
		// place and the first key moves to its own line.
		dash := node.Column - 3
		return op{kind: opReplaceLine, line: first.Line, lines: []string{
			yamledit.Spaces(dash) + "- " + key + ": " + sectionLiteral(key, value),
			yamledit.Spaces(dash+2) + first.Value + ":",
		}}, nil
	}
	return op{kind: opInsert, line: first.Line - 1, lines: []string{line}}, nil
}

// sectionLiteral writes a section key's value: an integer key plain, a text key
// the way hergy writes a new text.
func sectionLiteral(key, value string) string {
	if key == "level" {
		return value
	}
	return yamledit.ScalarLiteral(value, 0)
}

// firstKey is the first key of a mapping, in the order the file writes them.
func firstKey(node *yaml.Node) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode || len(node.Content) < 2 {
		return nil
	}
	return node.Content[0]
}

// sectionLines is one section written the way Emit writes it, as the lines of a
// block sequence item whose "- " stands at dashCol.
func sectionLines(section Section, dashCol int, path string) ([]string, error) {
	text, err := Emit(&Document{Sections: []Section{section}})
	if err != nil {
		return nil, err
	}
	root, err := readRoot([]byte(text), path)
	if err != nil {
		return nil, err
	}
	sections := yamledit.MappingValue(root, "sections")
	if sections == nil || len(sections.Content) == 0 {
		return nil, i18n.Errorf("error.edit_yaml", path, "節 1 つの文が読み取れません")
	}
	return shiftedNodeLines(text, sections.Content[0], dashCol, path)
}

// shiftedNodeLines is the lines one node of a written document stands on, moved
// to the column its "- " stands at.
func shiftedNodeLines(text string, node *yaml.Node, dashCol int, path string) ([]string, error) {
	from, to := yamledit.NodeRange(node)
	all := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if from < 1 || to > len(all) {
		return nil, i18n.Errorf("error.edit_yaml", path, "書いた行が読み取れません")
	}
	shift := dashCol - (node.Column - 3)
	lines := make([]string, 0, to-from+1)
	for _, line := range all[from-1 : to] {
		lines = append(lines, shifted(line, shift))
	}
	return lines, nil
}
