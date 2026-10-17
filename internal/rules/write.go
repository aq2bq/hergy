package rules

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// This file writes one document back with one rule changed, the way a person
// wrote the file it changes (段 4, d-7b58). The document reader (Parse) and the
// document writer (Emit) work on the whole document; a screen that changes one
// rule needs neither: only the lines of that rule's own item are written, and
// every other line — the blank lines, the comments, the quoting, the flow style
// of another rule's refs — comes back byte for byte.
//
// Emit is still the writer for a document hergy makes from nothing (the empty
// document a screen makes, 段 4 の「作る」): there, every line is hergy's.

// Edit is which one rule a screen changed: the rule at (Section, Rule) written
// over, one added at the end of Section (Added), or the rule at (Section, Rule)
// taken out (Removed).
type Edit struct {
	Section int
	Rule    int
	Added   bool
	Removed bool
}

// Write returns text with that one rule written the way doc holds it: doc is
// Parse(text) with SetRule, AddRule or DeleteRule applied, and edit says which
// rule the change was. The written lines are read back and compared with doc, so
// a splice that lands on the wrong lines is refused rather than written.
//
// A document whose sections or rules are written as a flow sequence (all on one
// line) is refused with error.edit_shape: there is no line of that rule's own to
// write. An added rule is written after the section's last item — or under the
// section itself when the section has no rules: key yet — and the key's line is
// turned into a block sequence when the file wrote the rules in flow.
//
// The sentence of a refusal names the document as doc.Path writes it, so the
// caller hands in the path it shows (spec/42 R2).
func Write(text []byte, doc *Document, edit Edit) ([]byte, error) {
	path := doc.Path
	root, err := readRoot(text, path)
	if err != nil {
		return nil, err
	}
	sections := yamledit.MappingValue(root, "sections")
	if sections == nil || sections.Kind != yaml.SequenceNode || sections.Style&yaml.FlowStyle != 0 {
		return nil, i18n.Errorf("error.edit_shape", path, "sections")
	}
	if edit.Section < 0 || edit.Section >= len(sections.Content) {
		return nil, i18n.Errorf("error.edit_shape", path, "sections")
	}
	section := sections.Content[edit.Section]
	if section.Kind != yaml.MappingNode {
		return nil, i18n.Errorf("error.edit_shape", path, "sections")
	}
	src := yamledit.New(text, path)

	switch {
	case edit.Added:
		if err := addRuleLines(src, section, doc, edit.Section); err != nil {
			return nil, err
		}
	case edit.Removed:
		if err := removeRuleLines(src, section, edit.Rule, path); err != nil {
			return nil, err
		}
	default:
		item, err := ruleItem(section, edit.Rule, path)
		if err != nil {
			return nil, err
		}
		lines, err := ruleLines(ruleOf(doc, edit.Section, edit.Rule), item.Column-3, path)
		if err != nil {
			return nil, err
		}
		yamledit.ReplaceNode(src, item, lines)
	}

	out := src.Bytes()
	// A write whose values are the ones the file already holds writes nothing:
	// a save that changed no value must not rewrite a shape a person wrote (a
	// quoted value, a flow sequence) into hergy's own (spec/42 R1).
	if err := sameValuesAs(doc, text); err == nil {
		return text, nil
	}
	if err := sameValuesAs(doc, out); err != nil {
		return nil, err
	}
	return out, nil
}

// readRoot is the file's own tree, refusing the shapes a line write cannot
// place: a file that is not one mapping, or one that holds a `<<:` merge, an
// anchor or a `*` alias (the same refusal team.yaml gets).
func readRoot(text []byte, path string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(text, &doc); err != nil {
		return nil, i18n.Errorf("error.edit_yaml", path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, i18n.Errorf("error.edit_not_mapping", path)
	}
	root := doc.Content[0]
	if key, ok := yamledit.ShapeKeyOf(root); ok {
		return nil, i18n.Errorf("error.edit_shape", path, key)
	}
	return root, nil
}

// sameValuesAs reads the text back and compares it with the document the write
// means, so lines placed on the wrong place are caught here rather than written.
// Only the values are compared: the order the file keeps its keys in is the
// file's own.
func sameValuesAs(doc *Document, out []byte) error {
	text, err := Emit(doc)
	if err != nil {
		return err
	}
	want, err := readRoot([]byte(text), doc.Path)
	if err != nil {
		return err
	}
	return yamledit.SameValues(want, out, doc.Path)
}

// ruleOf is one rule of the document the write means.
func ruleOf(doc *Document, section, rule int) Rule {
	if section < 0 || section >= len(doc.Sections) {
		return Rule{}
	}
	if rule < 0 || rule >= len(doc.Sections[section].Rules) {
		return Rule{}
	}
	return doc.Sections[section].Rules[rule]
}

// ruleItem is the node of one rule of a section, or the refusal its place gets.
func ruleItem(section *yaml.Node, rule int, path string) (*yaml.Node, error) {
	v := yamledit.MappingValue(section, "rules")
	if v == nil || v.Kind != yaml.SequenceNode || v.Style&yaml.FlowStyle != 0 {
		return nil, i18n.Errorf("error.edit_shape", path, "rules")
	}
	if rule < 0 || rule >= len(v.Content) {
		return nil, i18n.Errorf("error.edit_shape", path, "rules")
	}
	return v.Content[rule], nil
}

// removeRuleLines takes one rule's item out, with the key that held it when it
// was the section's only rule: the document reads as the section that is left,
// which Emit writes without a rules: key. A section that was nothing but its
// rules becomes the empty section Emit writes ({}) — the one shape where the
// key cannot go on its own.
func removeRuleLines(src *yamledit.Source, section *yaml.Node, rule int, path string) error {
	item, err := ruleItem(section, rule, path)
	if err != nil {
		return err
	}
	v := yamledit.MappingValue(section, "rules")
	if len(v.Content) > 1 {
		yamledit.RemoveNode(src, item)
		return nil
	}
	if len(section.Content) > 2 {
		// The item's lines go first: taking the key's line out above would move
		// the item's own lines up under it.
		yamledit.RemoveNode(src, item)
		yamledit.RemoveKeyLines(src, section, "rules")
		return nil
	}
	yamledit.ReplaceNode(src, section, []string{yamledit.Spaces(section.Column-3) + "- {}"})
	return nil
}

// addRuleLines writes the document's last rule of one section after that
// section's last rule, or, where the section has no rules: key yet, under the
// section itself. A section written as one empty flow mapping ({}) is the one
// shape with no line to write under: it becomes the section's own item.
func addRuleLines(src *yamledit.Source, section *yaml.Node, doc *Document, at int) error {
	path := doc.Path
	rules := doc.Sections[at].Rules
	if len(rules) == 0 {
		return i18n.Errorf("error.edit_shape", path, "rules")
	}
	rule := rules[len(rules)-1]

	v := yamledit.MappingValue(section, "rules")
	switch {
	case section.Style&yaml.FlowStyle != 0:
		if len(section.Content) != 0 {
			// {heading: …} has no line of its own to write the key under.
			return i18n.Errorf("error.edit_shape", path, "sections")
		}
		dash := section.Column - 3
		lines, err := ruleLines(rule, dash+4, path)
		if err != nil {
			return err
		}
		yamledit.ReplaceNode(src, section, append([]string{yamledit.Spaces(dash+2) + "rules:"}, lines...))
		return nil
	case v == nil:
		if len(section.Content) == 0 {
			return i18n.Errorf("error.edit_shape", path, "sections")
		}
		indent := yamledit.MappingIndent(section)
		lines, err := ruleLines(rule, indent+2, path)
		if err != nil {
			return err
		}
		head := append([]string{yamledit.Spaces(indent) + "rules:"}, lines...)
		yamledit.InsertAfterNode(src, section, head...)
		return nil
	default:
		place, err := yamledit.BlockPlace(src, section, "rules", v)
		if err != nil {
			return err
		}
		lines, err := ruleLines(rule, place.DashCol, path)
		if err != nil {
			return err
		}
		src.Insert(place.AfterLast, lines...)
		return nil
	}
}

// ruleLines is one rule written the way Emit writes it, as the lines of a block
// sequence item whose "- " stands at dashCol: the item's first line, and the
// lines under it, moved to where the file writes that section's rules.
func ruleLines(rule Rule, dashCol int, path string) ([]string, error) {
	text, err := Emit(&Document{Sections: []Section{{Rules: []Rule{rule}}}})
	if err != nil {
		return nil, err
	}
	root, err := readRoot([]byte(text), path)
	if err != nil {
		return nil, err
	}
	sections := yamledit.MappingValue(root, "sections")
	if sections == nil || len(sections.Content) == 0 {
		return nil, i18n.Errorf("error.edit_yaml", fmt.Errorf("規則 1 件の文が読み取れません"))
	}
	rules := yamledit.MappingValue(sections.Content[0], "rules")
	if rules == nil || len(rules.Content) == 0 {
		return nil, i18n.Errorf("error.edit_yaml", fmt.Errorf("規則 1 件の文が読み取れません"))
	}
	node := rules.Content[0]
	from, to := yamledit.NodeRange(node)
	all := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if from < 1 || to > len(all) {
		return nil, i18n.Errorf("error.edit_yaml", fmt.Errorf("規則 1 件の文が読み取れません"))
	}
	shift := dashCol - (node.Column - 3)
	lines := make([]string, 0, to-from+1)
	for _, line := range all[from-1 : to] {
		lines = append(lines, shifted(line, shift))
	}
	return lines, nil
}

// shifted moves one line right (or left) by shift spaces, so a rule written at
// Emit's indent is written where the file writes that section's rules. Only the
// leading spaces move: a block scalar's own lines keep their depth under the
// key.
func shifted(line string, shift int) string {
	if shift == 0 || line == "" || strings.TrimSpace(line) == "" {
		return line
	}
	if shift > 0 {
		return strings.Repeat(" ", shift) + line
	}
	n := -shift
	i := 0
	for i < n && i < len(line) && line[i] == ' ' {
		i++
	}
	return line[i:]
}
