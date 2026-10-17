// Package yamledit writes one place in a settings file and leaves every other
// line as it was. The yaml.Node tree says where the place is (Line, Column), and
// only the lines that place lives on are written; the line ending (CRLF or LF),
// a leading "---", the comments, the blank lines, the indent a person wrote and
// a value's own quoting all come back byte for byte.
//
// It is the one line editor hergy's settings files share (d-7b58, spec/41 S1):
// definition's team.yaml and a rule document both write this way, and a profiles
// file follows, so the same thing is not built twice. A write that cannot be
// placed (a merge, an anchor, an alias, a shape the lines do not say) is refused
// with error.edit_shape, and the written lines are read back and compared with
// the tree the write means before the caller writes them.
//
// It reads yaml.v3 and i18n only, so every core package may use it.
package yamledit

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/aq2bq/hergy/internal/i18n"
	"go.yaml.in/yaml/v3"
)

// The node-range write (段 4): the lines one node's own text is on, replaced,
// added after, or taken out. This is what a rule document writes a rule with.

// NodeRange is the lines a node's own text is on: its first line and the last
// line of the node (a block value under it counts as the node's own).
func NodeRange(n *yaml.Node) (from, to int) {
	return n.Line, NodeEndLine(n)
}

// ReplaceNode puts lines where a node's own lines were. Every line outside that
// range stays as it was.
func ReplaceNode(src *Source, n *yaml.Node, lines []string) {
	from, to := NodeRange(n)
	src.Replace(from, to, lines)
}

// InsertAfterNode writes lines after a node's last line.
func InsertAfterNode(src *Source, n *yaml.Node, lines ...string) {
	src.Insert(NodeEndLine(n), lines...)
}

// RemoveNode takes a node's own lines out, with the comment lines the file wrote
// above the node's first line at its own indent (they are the node's, and the
// node is going).
func RemoveNode(src *Source, n *yaml.Node) {
	src.Remove(src.ItemStartLine(n), NodeEndLine(n))
}

// SameValues is whether the written text reads as the intended tree. Only the
// values are compared (the order of a mapping's keys is not the file's promise),
// so a value put on the wrong line, dropped, or written unquoted where it needs
// quotes is caught.
func SameValues(want *yaml.Node, out []byte) error {
	encoded, err := encode(want)
	if err != nil {
		return err
	}
	var wanted, got any
	if err := yaml.Unmarshal(encoded, &wanted); err != nil {
		return i18n.Errorf("error.edit_yaml", err)
	}
	if err := yaml.Unmarshal(out, &got); err != nil {
		return i18n.Errorf("error.edit_yaml", err)
	}
	if !reflect.DeepEqual(wanted, got) {
		return i18n.Errorf("error.edit_yaml", fmt.Errorf("書き換えた行が、意図した変更と読み取れません"))
	}
	return nil
}

// encode writes the document the way hergy writes its own settings: a
// two-space indent, so a file already in that form is not re-indented. It is
// used to compare values, not to write the file.
func encode(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, i18n.Errorf("error.edit_yaml", err)
	}
	if err := enc.Close(); err != nil {
		return nil, i18n.Errorf("error.edit_yaml", err)
	}
	return buf.Bytes(), nil
}

// ReplaceScalar writes a new value over the value on a line, keeping the
// key, the indent, the comment after it and the line ending. The new value is
// written the way the old one was (a quoted value stays quoted).
func ReplaceScalar(src *Source, val *yaml.Node, value, label string, flow bool) error {
	if err := WritableValue(value, label); err != nil {
		return err
	}
	if val.Anchor != "" {
		// The value is referred to from elsewhere by an alias; rewriting it
		// here would move what every alias reads, so the file is left alone.
		return i18n.Errorf("error.edit_shape", label)
	}
	if val.Kind != yaml.ScalarNode || val.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return i18n.Errorf("error.edit_shape", label)
	}
	line := src.Line(val.Line)
	start := ByteColumn(line, val.Column)
	end, ok := ScalarEnd(line, start, flow)
	if !ok {
		return i18n.Errorf("error.edit_shape", label)
	}
	src.lines[val.Line-1] = line[:start] + ScalarLiteral(value, val.Style) + line[end:]
	return nil
}

// WritableValue is a value this editor can put on one line. A value with a line
// break in it cannot be a one-line scalar, so the file is left for a person to
// edit rather than rewritten on a guess.
func WritableValue(value, label string) error {
	if strings.ContainsAny(value, "\n\r") {
		return i18n.Errorf("error.edit_shape", label)
	}
	return nil
}

// SeqPlace is where a block sequence's items live on the file's lines: the
// column of the "-", and the line the last item ends on (what the next item is
// written after).
type SeqPlace struct {
	DashCol   int
	AfterLast int
}

// BlockPlace is the place a new item goes, and rewrites the key's own line when
// the file wrote the sequence on that line (a lone scalar, or a flow sequence
// [a -> b]), so the rest of the file keeps its lines. The rewrite touches only
// the key's line: the comment on it stays, and the next key's blank lines and
// comments stay where they were.
func BlockPlace(src *Source, root *yaml.Node, key string, v *yaml.Node) (SeqPlace, error) {
	keyNode := MappingKeyNode(root, key)
	if keyNode == nil {
		return SeqPlace{}, i18n.Errorf("error.edit_shape", key)
	}
	keyLine := src.Line(keyNode.Line)
	keyIndent := LeadingSpaces(keyLine)
	if v.Kind == yaml.ScalarNode && v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 {
		if v.Tag != "!!null" {
			items := []string{ScalarLiteral(v.Value, v.Style)}
			lines := append([]string{SequenceKeyLine(keyLine, key, v.LineComment)}, ItemLines(items, keyIndent+2)...)
			src.ReplaceLine(keyNode.Line, lines)
			return SeqPlace{DashCol: keyIndent + 2, AfterLast: keyNode.Line + len(lines) - 1}, nil
		}
		return SeqPlace{DashCol: keyIndent + 2, AfterLast: keyNode.Line}, nil
	}
	if v.Kind != yaml.SequenceNode {
		return SeqPlace{}, i18n.Errorf("error.edit_shape", key)
	}
	if v.Style&yaml.FlowStyle != 0 {
		items := make([]string, 0, len(v.Content))
		for _, item := range v.Content {
			text, err := FlowItem(item, key)
			if err != nil {
				return SeqPlace{}, err
			}
			items = append(items, text)
		}
		lines := append([]string{SequenceKeyLine(keyLine, key, v.LineComment)}, ItemLines(items, keyIndent+2)...)
		src.ReplaceLine(keyNode.Line, lines)
		return SeqPlace{DashCol: keyIndent + 2, AfterLast: keyNode.Line + len(lines) - 1}, nil
	}
	if len(v.Content) == 0 {
		return SeqPlace{DashCol: keyIndent + 2, AfterLast: keyNode.Line}, nil
	}
	last := v.Content[len(v.Content)-1]
	return SeqPlace{DashCol: v.Content[0].Column - 3, AfterLast: NodeEndLine(last)}, nil
}

// SequenceKeyLine is the key's line without a value on it ("turns:"), keeping
// its indent and any comment the file had after the value.
func SequenceKeyLine(keyLine, key, comment string) string {
	out := Spaces(LeadingSpaces(keyLine)) + key + ":"
	if comment != "" {
		out += " " + comment
	}
	return out
}

// ItemLines is the block lines of a sequence's items, one per item.
func ItemLines(items []string, indent int) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, Spaces(indent)+"- "+item)
	}
	return lines
}

// FlowItem is one item written as a block sequence entry holds it: its value, or
// a flow mapping ({from: …}) so a worker written either way keeps its shape.
func FlowItem(item *yaml.Node, key string) (string, error) {
	switch item.Kind {
	case yaml.ScalarNode:
		return ScalarLiteral(item.Value, item.Style), nil
	case yaml.MappingNode:
		parts := make([]string, 0, len(item.Content)/2)
		for i := 0; i+1 < len(item.Content); i += 2 {
			parts = append(parts, item.Content[i].Value+": "+ScalarLiteral(item.Content[i+1].Value, item.Content[i+1].Style))
		}
		return "{" + strings.Join(parts, ", ") + "}", nil
	default:
		return "", i18n.Errorf("error.edit_shape", key)
	}
}

// RemoveItemLines takes one item out: its own lines, and the comment lines
// yaml.v3 attached to it above. The blank lines around it stay. When it was the
// last item, the key's line says the sequence is empty (turns: []), the way the
// tree edit writes it.
func RemoveItemLines(src *Source, root *yaml.Node, key string, v, item *yaml.Node) error {
	if v.Style&yaml.FlowStyle != 0 {
		return RemoveFlowItem(src, root, key, v, item)
	}
	if len(v.Content) == 1 {
		if keyNode := MappingKeyNode(root, key); keyNode != nil {
			src.lines[keyNode.Line-1] = Spaces(LeadingSpaces(src.Line(keyNode.Line))) + key + ": []"
		}
	}
	src.Remove(src.ItemStartLine(item), NodeEndLine(item))
	return nil
}

// RemoveFlowItem rewrites the key's line without one item, when the sequence
// was written as a flow sequence on it.
func RemoveFlowItem(src *Source, root *yaml.Node, key string, v, item *yaml.Node) error {
	texts := make([]string, 0, len(v.Content))
	for _, other := range v.Content {
		if other == item {
			continue
		}
		text, err := FlowItem(other, key)
		if err != nil {
			return err
		}
		texts = append(texts, text)
	}
	keyNode := MappingKeyNode(root, key)
	line := src.Line(keyNode.Line)
	start := ByteColumn(line, v.Column)
	comment := ""
	if v.LineComment != "" {
		comment = " " + v.LineComment
	}
	src.lines[keyNode.Line-1] = line[:start] + "[" + strings.Join(texts, ", ") + "]" + comment
	return nil
}

// RemoveKeyLines takes a key and its value out of a file, with the comment
// lines the key had above it (they are the key's, and the key is going).
func RemoveKeyLines(src *Source, root *yaml.Node, key string) {
	keyNode := MappingKeyNode(root, key)
	if keyNode == nil {
		return
	}
	src.Remove(src.ItemStartLine(keyNode), NodeEndLine(keyNode))
}

// MappingValue is the value a mapping writes for a key, or nil. The first key
// wins, which is the one the reader keeps (a repeated key is another gate's).
func MappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// MappingKeyNode is the key node a mapping writes for a key, or nil.
func MappingKeyNode(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i]
		}
	}
	return nil
}

// MappingIndent is the column a mapping's own keys start at, as spaces, so a
// key added to it lines up with the keys beside it.
func MappingIndent(mapping *yaml.Node) int {
	if len(mapping.Content) >= 2 && mapping.Content[0].Column > 1 {
		return mapping.Content[0].Column - 1
	}
	return 0
}

// ShapeKeyOf is the key of a shape a line edit cannot rewrite, if the file has
// one: a `<<:` merge, which gives a mapping keys it does not write, or a `*`
// alias, whose value is written elsewhere in the file. The key is the
// language-neutral yaml key the shape sits under (`members`, `turns`, …), so
// the refusal reads the same in every locale; a shape at the top of the file
// names itself (`<<`, `*`).
func ShapeKeyOf(root *yaml.Node) (string, bool) {
	owner, found := "", false
	var walk func(n *yaml.Node, at string)
	walk = func(n *yaml.Node, at string) {
		if n == nil || found {
			return
		}
		if n.Kind == yaml.AliasNode {
			owner, found = named(at, "*"), true
			return
		}
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if k := n.Content[i]; k.Tag == "!!merge" || k.Value == "<<" {
					owner, found = named(at, "<<"), true
					return
				}
			}
			for i := 0; i+1 < len(n.Content); i += 2 {
				walk(n.Content[i+1], n.Content[i].Value)
				if found {
					return
				}
			}
			return
		}
		for _, c := range n.Content {
			walk(c, at)
		}
	}
	walk(root, "")
	return owner, found
}

// named is a shape's own key when it has no key above it.
func named(at, self string) string {
	if at == "" {
		return self
	}
	return at
}

// Source is a file split into lines, remembering the line ending it uses and
// whether it ends with one, so an edit can put back the lines it did not change
// byte for byte.
type Source struct {
	lines    []string
	nl       string
	trailing bool
}

// New splits text into lines, with the line ending kept aside.
func New(text []byte) *Source {
	s := string(text)
	nl := "\n"
	if strings.Contains(s, "\r\n") {
		nl = "\r\n"
	}
	body := strings.ReplaceAll(s, "\r\n", "\n")
	trailing := strings.HasSuffix(body, "\n")
	return &Source{
		lines:    strings.Split(strings.TrimSuffix(body, "\n"), "\n"),
		nl:       nl,
		trailing: trailing,
	}
}

// Bytes is the file back, with the line ending it came with.
func (s *Source) Bytes() []byte {
	out := strings.Join(s.lines, s.nl)
	if s.trailing {
		out += s.nl
	}
	return []byte(out)
}

// Line is line n (1-based), or "" past the end.
func (s *Source) Line(n int) string {
	if n < 1 || n > len(s.lines) {
		return ""
	}
	return s.lines[n-1]
}

// Insert writes lines after line n (0 means at the head).
func (s *Source) Insert(after int, lines ...string) {
	at := max(min(after, len(s.lines)), 0)
	rest := append([]string(nil), s.lines[at:]...)
	s.lines = append(s.lines[:at:at], lines...)
	s.lines = append(s.lines, rest...)
}

// Append writes lines at the end of the file.
func (s *Source) Append(lines ...string) {
	s.lines = append(s.lines, lines...)
}

// ReplaceLine puts lines where line n was.
func (s *Source) ReplaceLine(n int, lines []string) {
	if n < 1 || n > len(s.lines) {
		return
	}
	at := n - 1
	rest := append([]string(nil), s.lines[at+1:]...)
	s.lines = append(s.lines[:at:at], lines...)
	s.lines = append(s.lines, rest...)
}

// Replace puts lines where the lines from..to were (1-based, both ends
// included).
func (s *Source) Replace(from, to int, lines []string) {
	if from < 1 || to > len(s.lines) || from > to {
		return
	}
	rest := append([]string(nil), s.lines[to:]...)
	s.lines = append(s.lines[:from-1:from-1], lines...)
	s.lines = append(s.lines, rest...)
}

// Remove takes out lines from..to (1-based, both ends included).
func (s *Source) Remove(from, to int) {
	if from < 1 {
		from = 1
	}
	if to > len(s.lines) {
		to = len(s.lines)
	}
	if from > to {
		return
	}
	s.lines = append(s.lines[:from-1:from-1], s.lines[to:]...)
}

// NodeEndLine is the last line a node's own text is on. A block scalar's value
// runs under its key, so those lines count as the node's own: a write that takes
// a rule out, or puts a key after one, must not land in the middle of a
// multi-line text.
func NodeEndLine(n *yaml.Node) int {
	end := n.Line
	if lines := blockLines(n); lines > 0 {
		end += lines
	}
	for _, c := range n.Content {
		if e := NodeEndLine(c); e > end {
			end = e
		}
	}
	return end
}

// blockLines is how many lines a block scalar's value runs over under its own
// line: none for any other node, and none for an empty value.
func blockLines(n *yaml.Node) int {
	if n.Kind != yaml.ScalarNode || n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 || n.Value == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSuffix(n.Value, "\n"), "\n"))
}

// ItemStartLine is where an item starts: its line, pulled up by the comment
// lines the file writes above it at the item's own indent (they are the item's,
// and the item is going). The comment is read off the text, not yaml.v3's
// HeadComment, which it does not attach in a CRLF file; a line that is not such
// a comment stops the walk.
func (s *Source) ItemStartLine(n *yaml.Node) int {
	indent := LeadingSpaces(s.Line(n.Line))
	start := n.Line
	for start > 1 && isCommentLine(s.Line(start-1), indent) {
		start--
	}
	return start
}

// isCommentLine is whether a line is a comment at an indent (a "#" after
// exactly that many spaces).
func isCommentLine(line string, indent int) bool {
	return LeadingSpaces(line) == indent && strings.HasPrefix(strings.TrimLeft(line, " "), "#")
}

// ByteColumn is where a 1-based yaml column starts in a line, as a byte index
// (a line may hold characters that are more than one byte).
func ByteColumn(line string, col int) int {
	i, n := 0, 0
	for i < len(line) && n < col-1 {
		_, size := utf8.DecodeRuneInString(line[i:])
		i += size
		n++
	}
	return i
}

// ScalarEnd is where the scalar that starts at start ends on a line, so the
// comment and the spaces after the value can be kept. In flow context ({from:
// …}) the scalar also ends at a flow indicator.
func ScalarEnd(line string, start int, flow bool) (int, bool) {
	if start >= len(line) {
		return start, true
	}
	switch line[start] {
	case '"':
		for i := start + 1; i < len(line); i++ {
			if line[i] == '\\' {
				i++
				continue
			}
			if line[i] == '"' {
				return i + 1, true
			}
		}
		return 0, false
	case '\'':
		for i := start + 1; i < len(line); i++ {
			if line[i] == '\'' {
				if i+1 < len(line) && line[i+1] == '\'' {
					i++
					continue
				}
				return i + 1, true
			}
		}
		return 0, false
	default:
		i := start
		for i < len(line) {
			if flow && strings.IndexByte(",}]", line[i]) >= 0 {
				break
			}
			if line[i] == ' ' && i+1 < len(line) && line[i+1] == '#' {
				break
			}
			i++
		}
		for i > start && line[i-1] == ' ' {
			i--
		}
		return i, true
	}
}

// ScalarLiteral is a value written the way the file wrote the value it
// replaces: the same quoting style when there was one, and quotes when the
// value would not read back as the same string without them.
func ScalarLiteral(value string, style yaml.Style) string {
	switch {
	case style&yaml.DoubleQuotedStyle != 0:
		return doubleQuote(value)
	case style&yaml.SingleQuotedStyle != 0:
		return singleQuote(value)
	case plainSafe(value):
		return value
	default:
		return doubleQuote(value)
	}
}

// plainSafe is whether a value written without quotes reads back as the same
// string (a value holding " #", starting with an indicator, or reading as a
// number is quoted instead).
func plainSafe(value string) bool {
	if value == "" {
		return false
	}
	var out struct {
		V string `yaml:"v"`
	}
	if err := yaml.Unmarshal([]byte("v: "+value+"\n"), &out); err != nil {
		return false
	}
	return out.V == value
}

// doubleQuote is a value as a double-quoted scalar.
func doubleQuote(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// singleQuote is a value as a single-quoted scalar.
func singleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// Spaces is n spaces.
func Spaces(n int) string {
	if n < 0 {
		n = 0
	}
	return strings.Repeat(" ", n)
}

// LeadingSpaces is how many spaces a line starts with.
func LeadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
