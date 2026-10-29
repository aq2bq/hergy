package rules

import (
	"math/rand/v2"
	"regexp"
	"sort"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// This file is the one place hergy gives a rule its id (d-4aab). A person and a
// screen write no id: a rule is written without one, and the next write gives it
// a four-character id that no rule hergy reads and no launch record holds. An id
// already in the current shape is never changed, so a rule keeps its identity
// across edits — which is what the apply's changed list and the day a rule was
// switched off stand on.

// NeedsRuleID says a rule's id is missing or is one hergy did not write in its
// current shape. Such a rule is not broken: the next write gives it an id.
func NeedsRuleID(id string) bool { return !RuleIDPattern.MatchString(id) }

// idHead and idBody are the letters and digits an id is made of. The letters
// that read alike are left out (I, L, O, U), so an id read from a screen or a
// message cannot be taken for another.
const (
	idHead = "ABCDEFGHJKMNPQRSTVWXYZ"
	idBody = idHead + "0123456789"
)

// NewRuleID picks an id no one in taken holds, and gives up only when every id
// is taken (about a million of them): a caller that sees "" has a machine with
// more rules than an id can name.
func NewRuleID(taken map[string]bool) string {
	for range 1 << 20 {
		//nolint:gosec // a rule id is not a secret; the order is what keeps one from being guessed before it is written
		b := []byte{idHead[rand.IntN(len(idHead))]}
		for range 3 {
			//nolint:gosec // a rule id is not a secret
			b = append(b, idBody[rand.IntN(len(idBody))])
		}
		id := string(b)
		if !taken[id] {
			return id
		}
	}
	return ""
}

// AssignIDs gives every rule whose id is missing or in an older shape a new id,
// adds each to taken so the next rule cannot take it, and answers how many it
// gave and, for the ids it replaced, the new id by the old one. The caller
// rewrites the references to those old ids in every rule's text.
func (d *Document) AssignIDs(taken map[string]bool) (int, map[string]string) {
	renames := map[string]string{}
	given := 0
	for si := range d.Sections {
		for ri := range d.Sections[si].Rules {
			rule := &d.Sections[si].Rules[ri]
			if !NeedsRuleID(rule.ID) {
				taken[rule.ID] = true
				continue
			}
			old := rule.ID
			next := NewRuleID(taken)
			if next == "" {
				return given, renames
			}
			taken[next] = true
			rule.ID = next
			given++
			if old != "" {
				renames[old] = next
			}
		}
	}
	return given, renames
}

// Places is every rule where it stands, in the document's own order: the shape a
// write that only changes values hands Rewrite, so nothing moves.
func (d *Document) Places() []Place {
	var out []Place
	for si := range d.Sections {
		for ri := range d.Sections[si].Rules {
			out = append(out, Place{Section: si, Rule: ri})
		}
	}
	return out
}

// RewriteRuleRefs writes the new id in every rule text that names an old one,
// and answers how many texts it changed. A text names an id as a whole word, so
// C1 inside C10 is left alone.
func (d *Document) RewriteRuleRefs(renames map[string]string) int {
	if len(renames) == 0 {
		return 0
	}
	changed := 0
	for si := range d.Sections {
		for ri := range d.Sections[si].Rules {
			rule := &d.Sections[si].Rules[ri]
			text := rule.Text
			for old, next := range renames {
				text = replaceWord(text, old, next)
			}
			if text != rule.Text {
				rule.Text = text
				changed++
			}
		}
	}
	return changed
}

// ApplyIDs writes the ids hergy gave into a file as it stands: an old id is
// written as its new one everywhere it stands (its own "id:" line and every
// rule's text that names it), and a rule with no "id:" line gets one on a line
// of its own. Every other line — a comment, a blank line, a key hergy does not
// read — comes back byte for byte. A rule written as a flow mapping cannot hold
// a line of its own, and is refused.
func ApplyIDs(text []byte, path string, renames map[string]string, ids map[Place]string) ([]byte, error) {
	out := string(text)
	for old, next := range renames {
		out = replaceWord(out, old, next)
	}
	if len(ids) == 0 {
		return []byte(out), nil
	}
	root, err := readRoot([]byte(out), path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	changes, err := idLineChanges(root, lines, ids, path)
	if err != nil {
		return nil, err
	}
	lines = applyLineChanges(lines, changes)
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// idLine is where one id is written: the line it stands on, the indentation a
// new line takes, and whether the line is new.
type idLine struct {
	at     int
	indent int
	id     string
	insert bool
}

// idLineChanges says where each id of ids goes in a file's lines: on the rule's
// existing "id:" line, or on a line of its own when the rule has none.
func idLineChanges(root *yaml.Node, lines []string, ids map[Place]string, path string) ([]idLine, error) {
	sections := yamledit.MappingValue(root, "sections")
	if sections == nil || sections.Kind != yaml.SequenceNode {
		return nil, nil
	}
	var out []idLine
	for si, section := range sections.Content {
		rules := yamledit.MappingValue(section, "rules")
		if rules == nil || rules.Kind != yaml.SequenceNode {
			continue
		}
		for ri, item := range rules.Content {
			id, ok := ids[Place{Section: si, Rule: ri}]
			if !ok {
				continue
			}
			if item.Kind != yaml.MappingNode || len(item.Content) == 0 {
				return nil, i18n.Errorf("error.edit_shape", path, "rules")
			}
			if key := yamledit.MappingValue(item, "id"); key != nil {
				out = append(out, idKeyLine(key, lines, id))
				continue
			}
			line, err := idInsertLine(item, lines, id, path)
			if err != nil {
				return nil, err
			}
			out = append(out, line)
		}
	}
	return out, nil
}

// idKeyLine is the line an id already stands on: the key keeps its line (a rule
// written as "- id: X" keeps the dash), and only the value is written.
func idKeyLine(key *yaml.Node, lines []string, id string) idLine {
	if key.Value == id {
		return idLine{at: key.Line}
	}
	if key.Line >= 1 && key.Line <= len(lines) {
		if at := strings.Index(lines[key.Line-1], "id:"); at >= 0 {
			lines[key.Line-1] = lines[key.Line-1][:at] + "id: " + id
			return idLine{at: key.Line}
		}
	}
	return idLine{at: key.Line, indent: key.Column - 1, id: id}
}

// idInsertLine is the line a rule with no id gets: the first key's indentation,
// after the dash's own line when the first key stands on it.
func idInsertLine(item *yaml.Node, lines []string, id, path string) (idLine, error) {
	first := item.Content[0]
	col := max(first.Column, 1)
	line := first.Line
	if line < 1 || line > len(lines) {
		return idLine{}, i18n.Errorf("error.edit_shape", path, "rules")
	}
	onDash := strings.Contains(lines[line-1][:min(col-1, len(lines[line-1]))], "-")
	if onDash {
		line++
	}
	return idLine{at: line, indent: col - 1, id: id, insert: true}, nil
}

// applyLineChanges writes the id lines from the bottom of the file up, so an
// insertion does not move a later one.
func applyLineChanges(lines []string, changes []idLine) []string {
	sort.Slice(changes, func(i, j int) bool { return changes[i].at > changes[j].at })
	for _, one := range changes {
		if one.id == "" {
			continue
		}
		value := strings.Repeat(" ", one.indent) + "id: " + one.id
		if one.insert {
			idx := min(one.at-1, len(lines))
			lines = append(lines[:idx], append([]string{value}, lines[idx:]...)...)
			continue
		}
		if one.at >= 1 && one.at <= len(lines) {
			lines[one.at-1] = value
		}
	}
	return lines
}

// replaceWord writes next where word stands as a whole word in text.
func replaceWord(text, word, next string) string {
	if word == "" || word == next || !strings.Contains(text, word) {
		return text
	}
	return regexp.MustCompile(`\b`+regexp.QuoteMeta(word)+`\b`).ReplaceAllString(text, next)
}
