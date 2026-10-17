package definition

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/aq2bq/hergy/internal/i18n"
	"go.yaml.in/yaml/v3"
)

// OpKind is which one edit an Op is.
type OpKind int

const (
	// OpSetMission writes team.yaml's mission.
	OpSetMission OpKind = iota
	// OpSetMemberRole changes one member's role.
	OpSetMemberRole
	// OpSetMemberProfile changes one member's profile.
	OpSetMemberProfile
	// OpAddTurn appends one turns: line.
	OpAddTurn
	// OpRemoveTurn removes one turns: line.
	OpRemoveTurn
	// OpAddWorker appends one workers: entry.
	OpAddWorker
	// OpRemoveWorker removes one workers: entry.
	OpRemoveWorker
	// OpSetWorkerProfile changes one worker's profile.
	OpSetWorkerProfile
)

// Op is one edit to team.yaml: one place, one change. Build it with the
// constructors below, which name the fields each kind reads.
//
// An Op that names a member, a turn or a worker the file does not hold is an
// error rather than a silent no-op, so a screen that acts on a stale row is
// told rather than writing nothing (I12).
type Op struct {
	Kind    OpKind
	Mission string // OpSetMission
	Member  string // OpSetMemberRole, OpSetMemberProfile
	Role    string // OpSetMemberRole
	Profile string // OpSetMemberProfile, OpAddWorker, OpSetWorkerProfile
	Turn    Edge   // OpAddTurn, OpRemoveTurn
	From    string // OpAddWorker, OpRemoveWorker, OpSetWorkerProfile
	Purpose string // OpAddWorker, OpRemoveWorker, OpSetWorkerProfile
}

// Mission is the op that writes team.yaml's mission.
func Mission(text string) Op { return Op{Kind: OpSetMission, Mission: text} }

// MemberRole is the op that changes one member's role.
func MemberRole(member, role string) Op {
	return Op{Kind: OpSetMemberRole, Member: member, Role: role}
}

// MemberProfile is the op that changes one member's profile.
func MemberProfile(member, profile string) Op {
	return Op{Kind: OpSetMemberProfile, Member: member, Profile: profile}
}

// AddTurn is the op that appends one turns: line.
func AddTurn(from, to NodeID) Op {
	return Op{Kind: OpAddTurn, Turn: Edge{From: from, To: to}}
}

// RemoveTurn is the op that removes one turns: line.
func RemoveTurn(from, to NodeID) Op {
	return Op{Kind: OpRemoveTurn, Turn: Edge{From: from, To: to}}
}

// AddWorker is the op that appends one workers: entry.
func AddWorker(from, purpose, profile string) Op {
	return Op{Kind: OpAddWorker, From: from, Purpose: purpose, Profile: profile}
}

// RemoveWorker is the op that removes one workers: entry.
func RemoveWorker(from, purpose string) Op {
	return Op{Kind: OpRemoveWorker, From: from, Purpose: purpose}
}

// WorkerProfile is the op that changes one worker's profile.
func WorkerProfile(from, purpose, profile string) Op {
	return Op{Kind: OpSetWorkerProfile, From: from, Purpose: purpose, Profile: profile}
}

// EditYAML changes one place in a team.yaml and writes the file back (C3).
//
// It reads the yaml.Node tree to find the node the op names, and the tree edit
// also says what value has to come out; the file itself is written by
// replacing the lines that node lives on, so every line the op does not touch
// comes back byte for byte — the order of the keys, the comments, the blank
// lines, the indent a person wrote, a quoted value and even the line ending
// (CRLF) stay as they were. A file hergy wrote itself comes back in its own
// form too.
//
// The value of the written text is read back and compared with the tree the op
// means, so a line the edit placed on the wrong line is caught here rather than
// written. The shapes a line edit cannot rewrite (a value written as a block
// scalar, a `<<:` merge, an anchored value or a `*` alias) are refused with
// error.edit_shape instead of being rewritten whole.
//
// It does not check the definition: whether the result still reads is another
// gate's to stop (C2), and a screen checks before it writes rather than after.
func EditYAML(text []byte, op Op) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(text, &doc); err != nil {
		return nil, i18n.Errorf("error.edit_yaml", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, i18n.Errorf("error.edit_not_mapping")
	}
	root := doc.Content[0]
	if key, ok := shapeKeyOf(root); ok {
		return nil, i18n.Errorf("error.edit_shape", key)
	}

	src := newEditSource(text)
	if err := applyLines(src, root, op); err != nil {
		return nil, err
	}

	var want yaml.Node
	if err := yaml.Unmarshal(text, &want); err != nil {
		return nil, i18n.Errorf("error.edit_yaml", err)
	}
	if err := applyTree(want.Content[0], op); err != nil {
		return nil, err
	}

	out := src.bytes()
	if err := sameValues(&want, out); err != nil {
		return nil, err
	}
	return out, nil
}

// applyTree is the edit a reader means, on the tree: the value the written
// lines have to read back as. It is the tree edit C3 was first written with.
func applyTree(root *yaml.Node, op Op) error {
	switch op.Kind {
	case OpSetMission:
		putKey(root, "mission", op.Mission)
		return nil
	case OpSetMemberRole:
		return editMember(root, op.Member, "role", op.Role)
	case OpSetMemberProfile:
		return editMember(root, op.Member, "profile", op.Profile)
	case OpAddTurn:
		return addTurn(root, op.Turn)
	case OpRemoveTurn:
		return removeTurn(root, op.Turn)
	case OpAddWorker:
		return addWorker(root, op.From, op.Purpose, op.Profile)
	case OpRemoveWorker:
		return removeWorker(root, op.From, op.Purpose)
	case OpSetWorkerProfile:
		return setWorkerProfile(root, op.From, op.Purpose, op.Profile)
	default:
		return i18n.Errorf("error.edit_unknown_op")
	}
}

// sameValues is whether the written text reads as the intended tree. Only the
// values are compared (the order of a mapping's keys is not the file's promise),
// so a value put on the wrong line, dropped, or written unquoted where it needs
// quotes is caught.
func sameValues(want *yaml.Node, out []byte) error {
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

// applyLines is the edit on the file's own lines: one place, and every other
// line left as it was.
func applyLines(src *editSource, root *yaml.Node, op Op) error {
	switch op.Kind {
	case OpSetMission:
		return setMissionLines(src, root, op.Mission)
	case OpSetMemberRole:
		return setMemberLines(src, root, op.Member, "role", op.Role)
	case OpSetMemberProfile:
		return setMemberLines(src, root, op.Member, "profile", op.Profile)
	case OpAddTurn:
		return addTurnLines(src, root, op.Turn)
	case OpRemoveTurn:
		return removeTurnLines(src, root, op.Turn)
	case OpAddWorker:
		return addWorkerLines(src, root, op.From, op.Purpose, op.Profile)
	case OpRemoveWorker:
		return removeWorkerLines(src, root, op.From, op.Purpose)
	case OpSetWorkerProfile:
		return setWorkerProfileLines(src, root, op.From, op.Purpose, op.Profile)
	default:
		return i18n.Errorf("error.edit_unknown_op")
	}
}

// setMissionLines writes the mission on its own line, or, when the file has no
// mission:, inserts the line after version: (at the head when there is none
// either).
func setMissionLines(src *editSource, root *yaml.Node, value string) error {
	if v := mappingValue(root, "mission"); v != nil {
		return replaceScalarLine(src, v, value, "mission", false)
	}
	if err := writableValue(value, "mission"); err != nil {
		return err
	}
	at := 0
	if key := mappingKeyNode(root, "version"); key != nil {
		at = key.Line
	}
	src.insert(at, "mission: "+scalarLiteral(value, 0))
	return nil
}

// setMemberLines changes one key of the member named, or stops when the file
// has no such member.
func setMemberLines(src *editSource, root *yaml.Node, member, key, value string) error {
	item, ok := findMember(root, member)
	if !ok {
		return i18n.Errorf("error.edit_no_member", member)
	}
	return putKeyLines(src, item, key, value)
}

// putKeyLines writes a key's value on its own line, or adds the key below the
// mapping's last line when the mapping has none, lined up with the keys beside
// it.
func putKeyLines(src *editSource, item *yaml.Node, key, value string) error {
	if v := mappingValue(item, key); v != nil {
		return replaceScalarLine(src, v, value, key, item.Style&yaml.FlowStyle != 0)
	}
	if err := writableValue(value, key); err != nil {
		return err
	}
	src.insert(nodeEndLine(item), spaces(mappingIndent(item))+key+": "+scalarLiteral(value, 0))
	return nil
}

// addTurnLines appends one turns: line. Both ends have to be the human or a
// member the file holds, so a turn nobody can hand the turn to is not written;
// the same line twice is an error rather than a duplicate.
func addTurnLines(src *editSource, root *yaml.Node, turn Edge) error {
	if err := checkTurnEnds(root, turn); err != nil {
		return err
	}
	if err := checkTurnNew(root, turn); err != nil {
		return err
	}
	v := mappingValue(root, "turns")
	if v == nil {
		src.appendLines("turns:", "  - "+turn.String())
		return nil
	}
	place, err := blockPlace(src, root, "turns", v)
	if err != nil {
		return err
	}
	src.insert(place.afterLast, spaces(place.dashCol)+"- "+turn.String())
	return nil
}

// checkTurnNew is whether a turn is already in the file, the way the file
// writes it (a lone scalar is one turn).
func checkTurnNew(root *yaml.Node, turn Edge) error {
	v := mappingValue(root, "turns")
	if v == nil {
		return nil
	}
	if v.Kind == yaml.ScalarNode {
		if v.Tag != "!!null" && sameTurn(v, turn) {
			return i18n.Errorf("error.edit_turn_twice", turn.String())
		}
		return nil
	}
	if v.Kind != yaml.SequenceNode {
		return nil
	}
	for _, item := range v.Content {
		if sameTurn(item, turn) {
			return i18n.Errorf("error.edit_turn_twice", turn.String())
		}
	}
	return nil
}

// removeTurnLines takes out one turns: line, and stops when the file does not
// hold it. A key that held a lone scalar holds one line; taking it out takes
// the key with it.
func removeTurnLines(src *editSource, root *yaml.Node, turn Edge) error {
	v := mappingValue(root, "turns")
	if v == nil {
		return i18n.Errorf("error.edit_no_turn", turn.String())
	}
	if v.Kind == yaml.ScalarNode {
		if !sameTurn(v, turn) {
			return i18n.Errorf("error.edit_no_turn", turn.String())
		}
		removeKeyLines(src, root, "turns")
		return nil
	}
	if v.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_turn", turn.String())
	}
	for _, item := range v.Content {
		if sameTurn(item, turn) {
			return removeItemLines(src, root, "turns", v, item)
		}
	}
	return i18n.Errorf("error.edit_no_turn", turn.String())
}

// addWorkerLines appends one workers: entry. Its from: has to be a member the
// file holds, and the same member-and-purpose pair twice is an error (I12c).
// The new entry is written the way the entries beside it are (a flow entry when
// they are).
func addWorkerLines(src *editSource, root *yaml.Node, from, purpose, profile string) error {
	if _, ok := findMember(root, from); !ok {
		return i18n.Errorf("error.graph_worker_from", from)
	}
	if err := checkWorkerNew(root, from, purpose); err != nil {
		return err
	}
	if err := writableValue(purpose, "workers"); err != nil {
		return err
	}
	v := mappingValue(root, "workers")
	if v == nil {
		src.appendLines("workers:",
			"  - from: "+scalarLiteral(from, 0),
			"    purpose: "+scalarLiteral(purpose, 0),
			"    profile: "+scalarLiteral(profile, 0))
		return nil
	}
	place, err := blockPlace(src, root, "workers", v)
	if err != nil {
		return err
	}
	if workerFlow(v) {
		src.insert(place.afterLast, spaces(place.dashCol)+"- {from: "+scalarLiteral(from, 0)+", purpose: "+scalarLiteral(purpose, 0)+", profile: "+scalarLiteral(profile, 0)+"}")
		return nil
	}
	src.insert(place.afterLast,
		spaces(place.dashCol)+"- from: "+scalarLiteral(from, 0),
		spaces(place.dashCol+2)+"purpose: "+scalarLiteral(purpose, 0),
		spaces(place.dashCol+2)+"profile: "+scalarLiteral(profile, 0))
	return nil
}

// checkWorkerNew is whether the member already hires a worker for that purpose.
func checkWorkerNew(root *yaml.Node, from, purpose string) error {
	seq := mappingValue(root, "workers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	for _, item := range seq.Content {
		if f, p, ok := workerKey(item); ok && f == from && p == purpose {
			return i18n.Errorf("error.edit_worker_twice", purpose, from)
		}
	}
	return nil
}

// workerFlow is whether the workers: entries beside the new one are written as
// flow mappings ({from: …}), so the new entry is written the same way.
func workerFlow(v *yaml.Node) bool {
	if v.Kind != yaml.SequenceNode {
		return false
	}
	if v.Style&yaml.FlowStyle != 0 {
		return true
	}
	for _, item := range v.Content {
		return item.Kind == yaml.MappingNode && item.Style&yaml.FlowStyle != 0
	}
	return false
}

// removeWorkerLines takes out one workers: entry, by the member that hires it
// and its purpose (a worker's identity, I12c).
func removeWorkerLines(src *editSource, root *yaml.Node, from, purpose string) error {
	seq := mappingValue(root, "workers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_worker", purpose, from)
	}
	for _, item := range seq.Content {
		if f, p, ok := workerKey(item); ok && f == from && p == purpose {
			return removeItemLines(src, root, "workers", seq, item)
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// setWorkerProfileLines changes the profile of one workers: entry, named by the
// member that hires it and its purpose.
func setWorkerProfileLines(src *editSource, root *yaml.Node, from, purpose, profile string) error {
	seq := mappingValue(root, "workers")
	if seq != nil && seq.Kind == yaml.SequenceNode {
		for _, item := range seq.Content {
			if f, p, ok := workerKey(item); ok && f == from && p == purpose {
				return putKeyLines(src, item, "profile", profile)
			}
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// replaceScalarLine writes a new value over the value on a line, keeping the
// key, the indent, the comment after it and the line ending. The new value is
// written the way the old one was (a quoted value stays quoted).
func replaceScalarLine(src *editSource, val *yaml.Node, value, label string, flow bool) error {
	if err := writableValue(value, label); err != nil {
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
	line := src.line(val.Line)
	start := byteColumn(line, val.Column)
	end, ok := scalarEnd(line, start, flow)
	if !ok {
		return i18n.Errorf("error.edit_shape", label)
	}
	src.lines[val.Line-1] = line[:start] + scalarLiteral(value, val.Style) + line[end:]
	return nil
}

// writableValue is a value this editor can put on one line. A value with a line
// break in it cannot be a one-line scalar, so the file is left for a person to
// edit rather than rewritten on a guess.
func writableValue(value, label string) error {
	if strings.ContainsAny(value, "\n\r") {
		return i18n.Errorf("error.edit_shape", label)
	}
	return nil
}

// seqPlace is where a block sequence's items live on the file's lines: the
// column of the "-", and the line the last item ends on (what the next item is
// written after).
type seqPlace struct {
	dashCol   int
	afterLast int
}

// blockPlace is the place a new item goes, and rewrites the key's own line when
// the file wrote the sequence on that line (a lone scalar, or a flow sequence
// [a -> b]), so the rest of the file keeps its lines. The rewrite touches only
// the key's line: the comment on it stays, and the next key's blank lines and
// comments stay where they were.
func blockPlace(src *editSource, root *yaml.Node, key string, v *yaml.Node) (seqPlace, error) {
	keyNode := mappingKeyNode(root, key)
	if keyNode == nil {
		return seqPlace{}, i18n.Errorf("error.edit_shape", key)
	}
	keyLine := src.line(keyNode.Line)
	keyIndent := leadingSpaces(keyLine)
	if v.Kind == yaml.ScalarNode && v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 {
		if v.Tag != "!!null" {
			items := []string{scalarLiteral(v.Value, v.Style)}
			lines := append([]string{sequenceKeyLine(keyLine, key, v.LineComment)}, itemLines(items, keyIndent+2)...)
			src.replaceLine(keyNode.Line, lines)
			return seqPlace{dashCol: keyIndent + 2, afterLast: keyNode.Line + len(lines) - 1}, nil
		}
		return seqPlace{dashCol: keyIndent + 2, afterLast: keyNode.Line}, nil
	}
	if v.Kind != yaml.SequenceNode {
		return seqPlace{}, i18n.Errorf("error.edit_shape", key)
	}
	if v.Style&yaml.FlowStyle != 0 {
		items := make([]string, 0, len(v.Content))
		for _, item := range v.Content {
			text, err := flowItem(item, key)
			if err != nil {
				return seqPlace{}, err
			}
			items = append(items, text)
		}
		lines := append([]string{sequenceKeyLine(keyLine, key, v.LineComment)}, itemLines(items, keyIndent+2)...)
		src.replaceLine(keyNode.Line, lines)
		return seqPlace{dashCol: keyIndent + 2, afterLast: keyNode.Line + len(lines) - 1}, nil
	}
	if len(v.Content) == 0 {
		return seqPlace{dashCol: keyIndent + 2, afterLast: keyNode.Line}, nil
	}
	last := v.Content[len(v.Content)-1]
	return seqPlace{dashCol: v.Content[0].Column - 3, afterLast: nodeEndLine(last)}, nil
}

// sequenceKeyLine is the key's line without a value on it ("turns:"), keeping
// its indent and any comment the file had after the value.
func sequenceKeyLine(keyLine, key, comment string) string {
	out := spaces(leadingSpaces(keyLine)) + key + ":"
	if comment != "" {
		out += " " + comment
	}
	return out
}

// itemLines is the block lines of a sequence's items, one per item.
func itemLines(items []string, indent int) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, spaces(indent)+"- "+item)
	}
	return lines
}

// flowItem is one item written as a block sequence entry holds it: its value, or
// a flow mapping ({from: …}) so a worker written either way keeps its shape.
func flowItem(item *yaml.Node, key string) (string, error) {
	switch item.Kind {
	case yaml.ScalarNode:
		return scalarLiteral(item.Value, item.Style), nil
	case yaml.MappingNode:
		parts := make([]string, 0, len(item.Content)/2)
		for i := 0; i+1 < len(item.Content); i += 2 {
			parts = append(parts, item.Content[i].Value+": "+scalarLiteral(item.Content[i+1].Value, item.Content[i+1].Style))
		}
		return "{" + strings.Join(parts, ", ") + "}", nil
	default:
		return "", i18n.Errorf("error.edit_shape", key)
	}
}

// removeItemLines takes one item out: its own lines, and the comment lines
// yaml.v3 attached to it above. The blank lines around it stay. When it was the
// last item, the key's line says the sequence is empty (turns: []), the way the
// tree edit writes it.
func removeItemLines(src *editSource, root *yaml.Node, key string, v, item *yaml.Node) error {
	if v.Style&yaml.FlowStyle != 0 {
		return removeFlowItem(src, root, key, v, item)
	}
	if len(v.Content) == 1 {
		if keyNode := mappingKeyNode(root, key); keyNode != nil {
			src.lines[keyNode.Line-1] = spaces(leadingSpaces(src.line(keyNode.Line))) + key + ": []"
		}
	}
	src.remove(src.itemStartLine(item), nodeEndLine(item))
	return nil
}

// removeFlowItem rewrites the key's line without one item, when the sequence
// was written as a flow sequence on it.
func removeFlowItem(src *editSource, root *yaml.Node, key string, v, item *yaml.Node) error {
	texts := make([]string, 0, len(v.Content))
	for _, other := range v.Content {
		if other == item {
			continue
		}
		text, err := flowItem(other, key)
		if err != nil {
			return err
		}
		texts = append(texts, text)
	}
	keyNode := mappingKeyNode(root, key)
	line := src.line(keyNode.Line)
	start := byteColumn(line, v.Column)
	comment := ""
	if v.LineComment != "" {
		comment = " " + v.LineComment
	}
	src.lines[keyNode.Line-1] = line[:start] + "[" + strings.Join(texts, ", ") + "]" + comment
	return nil
}

// removeKeyLines takes a key and its value out of a file, with the comment
// lines the key had above it (they are the key's, and the key is going).
func removeKeyLines(src *editSource, root *yaml.Node, key string) {
	keyNode := mappingKeyNode(root, key)
	if keyNode == nil {
		return
	}
	src.remove(src.itemStartLine(keyNode), nodeEndLine(keyNode))
}

// editMember changes one key of the member named, or stops when the file has no
// such member.
func editMember(root *yaml.Node, member, key, value string) error {
	item, ok := findMember(root, member)
	if !ok {
		return i18n.Errorf("error.edit_no_member", member)
	}
	putKey(item, key, value)
	return nil
}

// findMember is the members: entry whose name is member. A member's item is a
// mapping; another shape is skipped, and the name is compared as the reader
// compares it (trimmed).
func findMember(root *yaml.Node, member string) (*yaml.Node, bool) {
	if member == "" {
		return nil, false
	}
	for _, item := range sequenceOf(root, "members") {
		name := mappingValue(item, "name")
		if name != nil && name.Kind == yaml.ScalarNode && strings.TrimSpace(name.Value) == member {
			return item, true
		}
	}
	return nil, false
}

// addTurn appends one turns: line, on the tree (the intended value).
func addTurn(root *yaml.Node, turn Edge) error {
	if err := checkTurnEnds(root, turn); err != nil {
		return err
	}
	seq := ensureSequence(root, "turns")
	for _, item := range seq.Content {
		if sameTurn(item, turn) {
			return i18n.Errorf("error.edit_turn_twice", turn.String())
		}
	}
	seq.Content = append(seq.Content, valueScalar(turn.String()))
	return nil
}

// removeTurn takes out one turns: line, on the tree (the intended value).
func removeTurn(root *yaml.Node, turn Edge) error {
	value := mappingValue(root, "turns")
	if value == nil {
		return i18n.Errorf("error.edit_no_turn", turn.String())
	}
	if value.Kind == yaml.ScalarNode {
		if !sameTurn(value, turn) {
			return i18n.Errorf("error.edit_no_turn", turn.String())
		}
		removeKey(root, "turns")
		return nil
	}
	if value.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_turn", turn.String())
	}
	for i, item := range value.Content {
		if sameTurn(item, turn) {
			value.Content = append(value.Content[:i:i], value.Content[i+1:]...)
			return nil
		}
	}
	return i18n.Errorf("error.edit_no_turn", turn.String())
}

// checkTurnEnds stops a turn whose ends are not the human or a member the file
// holds. The sentences are validate's (error.graph_edge_from /
// error.graph_edge_to), with validate's arguments: the edge and the node.
func checkTurnEnds(root *yaml.Node, turn Edge) error {
	if !knownEnd(root, turn.From) {
		return i18n.Errorf("error.graph_edge_from", turn.String(), turn.From.String())
	}
	if !knownEnd(root, turn.To) {
		return i18n.Errorf("error.graph_edge_to", turn.String(), turn.To.String())
	}
	return nil
}

// knownEnd is whether a node a turns: line may name is the human or a member
// the file holds. Another kind is not written in turns: (I12c).
func knownEnd(root *yaml.Node, id NodeID) bool {
	switch id.Kind {
	case KindHuman:
		return id.Name == nodeKinds[KindHuman].fixedName
	case KindMember:
		_, ok := findMember(root, id.Name)
		return ok
	default:
		return false
	}
}

// addWorker appends one workers: entry, on the tree (the intended value).
func addWorker(root *yaml.Node, from, purpose, profile string) error {
	if _, ok := findMember(root, from); !ok {
		return i18n.Errorf("error.graph_worker_from", from)
	}
	seq := ensureSequence(root, "workers")
	for _, item := range seq.Content {
		if f, p, ok := workerKey(item); ok && f == from && p == purpose {
			return i18n.Errorf("error.edit_worker_twice", purpose, from)
		}
	}
	seq.Content = append(seq.Content, workerEntry(from, purpose, profile))
	return nil
}

// removeWorker takes out one workers: entry, on the tree (the intended value).
func removeWorker(root *yaml.Node, from, purpose string) error {
	seq := mappingValue(root, "workers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_worker", purpose, from)
	}
	for i, item := range seq.Content {
		if f, p, ok := workerKey(item); ok && f == from && p == purpose {
			seq.Content = append(seq.Content[:i:i], seq.Content[i+1:]...)
			return nil
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// setWorkerProfile changes the profile of one workers: entry, on the tree (the
// intended value).
func setWorkerProfile(root *yaml.Node, from, purpose, profile string) error {
	seq := mappingValue(root, "workers")
	if seq != nil && seq.Kind == yaml.SequenceNode {
		for _, item := range seq.Content {
			if f, p, ok := workerKey(item); ok && f == from && p == purpose {
				putKey(item, "profile", profile)
				return nil
			}
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// workerKey is a workers: entry's identity: the member that hires it and its
// purpose.
func workerKey(item *yaml.Node) (from, purpose string, ok bool) {
	fromNode := mappingValue(item, "from")
	purposeNode := mappingValue(item, "purpose")
	if fromNode == nil || purposeNode == nil || fromNode.Kind != yaml.ScalarNode || purposeNode.Kind != yaml.ScalarNode {
		return "", "", false
	}
	return strings.TrimSpace(fromNode.Value), strings.TrimSpace(purposeNode.Value), true
}

// workerEntry is one workers: entry as yaml nodes: from, purpose, profile.
func workerEntry(from, purpose, profile string) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		keyScalar("from"), valueScalar(from),
		keyScalar("purpose"), valueScalar(purpose),
		keyScalar("profile"), valueScalar(profile),
	}}
}

// mappingValue is the value a mapping writes for a key, or nil. The first key
// wins, which is the one the reader keeps (a repeated key is another gate's).
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
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

// mappingKeyNode is the key node a mapping writes for a key, or nil.
func mappingKeyNode(mapping *yaml.Node, key string) *yaml.Node {
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

// mappingIndent is the column a mapping's own keys start at, as spaces, so a
// key added to it lines up with the keys beside it.
func mappingIndent(mapping *yaml.Node) int {
	if len(mapping.Content) >= 2 && mapping.Content[0].Column > 1 {
		return mapping.Content[0].Column - 1
	}
	return 0
}

// shapeKeyOf is the key of a shape a line edit cannot rewrite, if the file has
// one: a `<<:` merge, which gives a mapping keys it does not write, or a `*`
// alias, whose value is written elsewhere in the file. The key is the
// language-neutral yaml key the shape sits under (`members`, `turns`, …), so
// the refusal reads the same in every locale; a shape at the top of the file
// names itself (`<<`, `*`).
func shapeKeyOf(root *yaml.Node) (string, bool) {
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

// sequenceOf is the items a key writes as a sequence, or nil when the key is
// absent or another shape.
func sequenceOf(root *yaml.Node, key string) []*yaml.Node {
	value := mappingValue(root, key)
	if value == nil || value.Kind != yaml.SequenceNode {
		return nil
	}
	return value.Content
}

// ensureSequence is the sequence a key writes, making the key and an empty
// sequence when the mapping has none. A key left as a lone scalar is turned
// into a one-item sequence holding what was there, so a file written either way
// edits the same (stringList reads both).
func ensureSequence(root *yaml.Node, key string) *yaml.Node {
	value := mappingValue(root, key)
	if value == nil {
		value = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, keyScalar(key), value)
		return value
	}
	if value.Kind == yaml.ScalarNode && value.Tag != "!!null" {
		one := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value.Value, Style: value.Style}
		value.Kind = yaml.SequenceNode
		value.Tag = "!!seq"
		value.Value = ""
		value.Style = 0
		value.Content = []*yaml.Node{one}
		return value
	}
	if value.Kind != yaml.SequenceNode {
		value.Kind = yaml.SequenceNode
		value.Tag = "!!seq"
		value.Value = ""
		value.Style = 0
		value.Content = nil
	}
	return value
}

// putKey writes a scalar for a key of a mapping, adding the key at the end when
// the mapping has none. An existing scalar keeps its style, so a quoted value
// stays quoted.
func putKey(mapping *yaml.Node, key, value string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			putScalar(mapping.Content[i+1], value)
			return
		}
	}
	mapping.Content = append(mapping.Content, keyScalar(key), valueScalar(value))
}

// putScalar changes a scalar's text in place, keeping the style the file wrote
// it with.
func putScalar(node *yaml.Node, value string) {
	node.Kind = yaml.ScalarNode
	node.Tag = "!!str"
	node.Value = value
	node.Content = nil
}

// removeKey takes a key and its value out of a mapping.
func removeKey(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// sameTurn is whether a turns: item is this edge, read the way the file reads
// it. An item that does not read as a turn is not this one.
func sameTurn(item *yaml.Node, turn Edge) bool {
	if item == nil || item.Kind != yaml.ScalarNode {
		return false
	}
	from, to, err := parseTurn(item.Value, "turns")
	if err != nil {
		return false
	}
	return from == turn.From && to == turn.To
}

// keyScalar is a mapping key.
func keyScalar(key string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
}

// valueScalar is a mapping or sequence value.
func valueScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// editSource is a file split into lines, remembering the line ending it uses
// and whether it ends with one, so an edit can put back the lines it did not
// change byte for byte.
type editSource struct {
	lines    []string
	nl       string
	trailing bool
}

// newEditSource splits text into lines, with the line ending kept aside.
func newEditSource(text []byte) *editSource {
	s := string(text)
	nl := "\n"
	if strings.Contains(s, "\r\n") {
		nl = "\r\n"
	}
	body := strings.ReplaceAll(s, "\r\n", "\n")
	trailing := strings.HasSuffix(body, "\n")
	return &editSource{
		lines:    strings.Split(strings.TrimSuffix(body, "\n"), "\n"),
		nl:       nl,
		trailing: trailing,
	}
}

// bytes is the file back, with the line ending it came with.
func (e *editSource) bytes() []byte {
	out := strings.Join(e.lines, e.nl)
	if e.trailing {
		out += e.nl
	}
	return []byte(out)
}

// line is line n (1-based), or "" past the end.
func (e *editSource) line(n int) string {
	if n < 1 || n > len(e.lines) {
		return ""
	}
	return e.lines[n-1]
}

// insert writes lines after line n (0 means at the head).
func (e *editSource) insert(after int, lines ...string) {
	at := max(min(after, len(e.lines)), 0)
	rest := append([]string(nil), e.lines[at:]...)
	e.lines = append(e.lines[:at:at], lines...)
	e.lines = append(e.lines, rest...)
}

// appendLines writes lines at the end of the file.
func (e *editSource) appendLines(lines ...string) {
	e.lines = append(e.lines, lines...)
}

// replaceLine puts lines where line n was.
func (e *editSource) replaceLine(n int, lines []string) {
	if n < 1 || n > len(e.lines) {
		return
	}
	at := n - 1
	rest := append([]string(nil), e.lines[at+1:]...)
	e.lines = append(e.lines[:at:at], lines...)
	e.lines = append(e.lines, rest...)
}

// remove takes out lines from..to (1-based, both ends included).
func (e *editSource) remove(from, to int) {
	if from < 1 {
		from = 1
	}
	if to > len(e.lines) {
		to = len(e.lines)
	}
	if from > to {
		return
	}
	e.lines = append(e.lines[:from-1:from-1], e.lines[to:]...)
}

// nodeEndLine is the last line a node's own text is on.
func nodeEndLine(n *yaml.Node) int {
	end := n.Line
	for _, c := range n.Content {
		if e := nodeEndLine(c); e > end {
			end = e
		}
	}
	return end
}

// itemStartLine is where an item starts: its line, pulled up by the comment
// lines the file writes above it at the item's own indent (they are the item's,
// and the item is going). The comment is read off the text, not yaml.v3's
// HeadComment, which it does not attach in a CRLF file; a line that is not such
// a comment stops the walk.
func (e *editSource) itemStartLine(n *yaml.Node) int {
	indent := leadingSpaces(e.line(n.Line))
	start := n.Line
	for start > 1 && isCommentLine(e.line(start-1), indent) {
		start--
	}
	return start
}

// isCommentLine is whether a line is a comment at an indent (a "#" after
// exactly that many spaces).
func isCommentLine(line string, indent int) bool {
	return leadingSpaces(line) == indent && strings.HasPrefix(strings.TrimLeft(line, " "), "#")
}

// byteColumn is where a 1-based yaml column starts in a line, as a byte index
// (a line may hold characters that are more than one byte).
func byteColumn(line string, col int) int {
	i, n := 0, 0
	for i < len(line) && n < col-1 {
		_, size := utf8.DecodeRuneInString(line[i:])
		i += size
		n++
	}
	return i
}

// scalarEnd is where the scalar that starts at start ends on a line, so the
// comment and the spaces after the value can be kept. In flow context ({from:
// …}) the scalar also ends at a flow indicator.
func scalarEnd(line string, start int, flow bool) (int, bool) {
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

// scalarLiteral is a value written the way the file wrote the value it
// replaces: the same quoting style when there was one, and quotes when the
// value would not read back as the same string without them.
func scalarLiteral(value string, style yaml.Style) string {
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

// spaces is n spaces.
func spaces(n int) string {
	if n < 0 {
		n = 0
	}
	return strings.Repeat(" ", n)
}

// leadingSpaces is how many spaces a line starts with.
func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
