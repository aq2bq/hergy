package yamledit

import (
	"strings"

	"go.yaml.in/yaml/v3"
)

// The value write (段 5, spec/41 S1): a key's whole value replaced, in the form
// the file wrote it. A rule document and team.yaml write their own places (段 4,
// 段 2); a profiles.yaml writes a profile's args and env this way, so the shape a
// person wrote (a flow collection on the key's line, or block lines under it)
// comes back the same.

// Pair is one key and its value in a mapping a screen writes.
type Pair struct {
	Key   string
	Value string
}

// ReplaceSeq writes a sequence under a key of a mapping the way the file wrote
// that value: a value on the key's own line (a flow sequence, a lone item, or an
// empty value) is written there again, and a block sequence keeps its items'
// indent and its key's own line. An empty list takes the key out, the way hergy
// writes a file with no value there.
//
// The key's own line (its indent, its comment) and every line outside the value
// stay as they were. A value written as a block mapping, where a sequence is
// meant, is a shape this editor cannot place (error.edit_shape).
func ReplaceSeq(src *Source, mapping *yaml.Node, key string, items []string) error {
	if len(items) == 0 {
		RemoveKeyLines(src, mapping, key)
		return nil
	}
	keyNode := MappingKeyNode(mapping, key)
	v := MappingValue(mapping, key)
	if keyNode == nil {
		// The key is not there yet: it is written under the mapping, in hergy's
		// own block form, lined up with the keys beside it.
		indent := MappingIndent(mapping)
		lines := append([]string{Spaces(indent) + key + ":"}, ItemLines(quoted(items), indent+2)...)
		src.Insert(NodeEndLine(mapping), lines...)
		return nil
	}
	if !onTheKeyLine(v) {
		if v.Kind != yaml.SequenceNode {
			return src.Shape(key)
		}
		indent := v.Content[0].Column - 3
		src.Replace(v.Line, NodeEndLine(v), ItemLines(quoted(items), indent))
		return nil
	}
	return KeyLineWith(src, keyNode, v, flowSeq(items))
}

// ReplaceMap writes a mapping under a key the way the file wrote that value:
// flow ({a: b}) on the key's own line, or block lines under it. pairs are
// written in the order given. No pair takes the key out. A value written as a
// block sequence, where a mapping is meant, is a shape this editor cannot place.
func ReplaceMap(src *Source, mapping *yaml.Node, key string, pairs []Pair) error {
	if len(pairs) == 0 {
		RemoveKeyLines(src, mapping, key)
		return nil
	}
	keyNode := MappingKeyNode(mapping, key)
	v := MappingValue(mapping, key)
	if keyNode == nil {
		indent := MappingIndent(mapping)
		lines := append([]string{Spaces(indent) + key + ":"}, pairLines(pairs, indent+2)...)
		src.Insert(NodeEndLine(mapping), lines...)
		return nil
	}
	if !onTheKeyLine(v) {
		if v.Kind != yaml.MappingNode {
			return src.Shape(key)
		}
		indent := v.Content[0].Column - 1
		src.Replace(v.Line, NodeEndLine(v), pairLines(pairs, indent))
		return nil
	}
	return KeyLineWith(src, keyNode, v, flowMap(pairs))
}

// onTheKeyLine is whether a value lives on its key's own line: a scalar, a flow
// collection, or an empty value. Anything else is written as block lines under
// the key.
func onTheKeyLine(v *yaml.Node) bool {
	if v == nil {
		return true
	}
	if v.Kind != yaml.SequenceNode && v.Kind != yaml.MappingNode {
		return true
	}
	return v.Style&yaml.FlowStyle != 0 || len(v.Content) == 0
}

// FlowEnd is where a flow collection starting at start ends on one line: the
// byte after its closing bracket or brace, and whether it closes there. It reads
// the line the way YAML does, so a bracket inside a quoted item does not end it.
func FlowEnd(line string, start int) (int, bool) {
	if start >= len(line) {
		return 0, false
	}
	wanted := map[byte]byte{'[': ']', '{': '}'}[line[start]]
	if wanted == 0 {
		return 0, false
	}
	depth := 0
	for i := start; i < len(line); {
		switch c := line[i]; c {
		case '\'', '"':
			i = skipQuoted(line, i)
		case '[', '{':
			depth++
			i++
		case ']', '}':
			depth--
			if depth == 0 {
				return i + 1, c == wanted
			}
			i++
		default:
			i++
		}
	}
	return 0, false
}

// skipQuoted is the byte after a quoted string that starts at i, or the end of
// the line when it is not closed there. A doubled quote inside a single-quoted
// string is one quote, and a backslash escapes in a double-quoted one.
func skipQuoted(line string, i int) int {
	quote := line[i]
	for i++; i < len(line); i++ {
		if quote == '"' && line[i] == '\\' {
			i++
			continue
		}
		if line[i] != quote {
			continue
		}
		if quote == '\'' && i+1 < len(line) && line[i+1] == '\'' {
			i++
			continue
		}
		return i + 1
	}
	return i
}

// KeyLineWith is a line's value replaced in place: everything before the value
// and everything after it — the spaces, the comment — stays as the file wrote
// it.
func KeyLineWith(src *Source, keyNode *yaml.Node, v *yaml.Node, value string) error {
	line := src.Line(keyNode.Line)
	start := ByteColumn(line, v.Column)
	end, ok := FlowEnd(line, start)
	if !ok {
		return src.Shape(keyNode.Value)
	}
	src.ReplaceLine(keyNode.Line, []string{line[:start] + value + line[end:]})
	return nil
}

// flowSeq is a sequence's items on one line: [a, b].
func flowSeq(items []string) string {
	return "[" + strings.Join(quoted(items), ", ") + "]"
}

// quoted is a sequence's items as the file writes them: each one written so it
// reads back as the string it is (a value that reads as a number is quoted).
func quoted(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, ScalarLiteral(item, 0))
	}
	return out
}

// flowMap is a mapping's keys and values on one line: {a: b}.
func flowMap(pairs []Pair) string {
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, pair.Key+": "+ScalarLiteral(pair.Value, 0))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// pairLines is a mapping's keys and values as block lines.
func pairLines(pairs []Pair, indent int) []string {
	lines := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		lines = append(lines, Spaces(indent)+pair.Key+": "+ScalarLiteral(pair.Value, 0))
	}
	return lines
}
