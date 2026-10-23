package rules

import (
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// This file writes one document back the way a person wrote the file it changes
// (段 4, d-7b58). The document reader (Parse) and the document writer (Emit)
// work on the whole document; a screen that changes one rule needs neither: only
// the lines of that rule's own item are written, and every other line — the
// blank lines, the comments, the quoting, the flow style of another rule's refs
// — comes back byte for byte. A screen that changes many places at once goes
// through Rewrite (rewrite.go), of which Write is the one-rule case.
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
func sameValuesAs(doc *Document, out []byte, screen SaveScreen) error {
	path := doc.Path
	text, err := Emit(doc)
	if err != nil {
		return err
	}
	want, err := readRoot([]byte(text), path)
	if err != nil {
		return err
	}
	return yamledit.SameValues(want, out, path, saveBug(screen))
}

// ruleLines is one rule written the way Emit writes it, as the lines of a block
// sequence item whose "- " stands at dashCol: the item's first line, and the
// lines under it, moved to where the file writes that section's rules.
func ruleLines(rule Rule, dashCol int, path string, screen SaveScreen) ([]string, error) {
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
		return nil, saveBug(screen).Errorf()
	}
	rules := yamledit.MappingValue(sections.Content[0], "rules")
	if rules == nil || len(rules.Content) == 0 {
		return nil, saveBug(screen).Errorf()
	}
	node := rules.Content[0]
	from, to := yamledit.NodeRange(node)
	all := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if from < 1 || to > len(all) {
		return nil, saveBug(screen).Errorf()
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
