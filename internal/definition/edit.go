package definition

import (
	"bytes"
	"strings"

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

// EditYAML changes one place in a team.yaml and writes the file back (C3). It
// walks the yaml.Node tree, so the order of the keys, the comments yaml.v3
// attached to the nodes, and the style a scalar was written with (a quoted role
// stays quoted) come out the way they went in. The file is written with a
// two-space indent, which is the form hergy writes its own settings in, so a
// file in that form has every line it does not touch come back byte for byte.
//
// It does not check the result: whether the definition still reads is another
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

	var err error
	switch op.Kind {
	case OpSetMission:
		putKey(root, "mission", op.Mission)
	case OpSetMemberRole:
		err = editMember(root, op.Member, "role", op.Role)
	case OpSetMemberProfile:
		err = editMember(root, op.Member, "profile", op.Profile)
	case OpAddTurn:
		err = addTurn(root, op.Turn)
	case OpRemoveTurn:
		err = removeTurn(root, op.Turn)
	case OpAddWorker:
		err = addWorker(root, op.From, op.Purpose, op.Profile)
	case OpRemoveWorker:
		err = removeWorker(root, op.From, op.Purpose)
	case OpSetWorkerProfile:
		err = setWorkerProfile(root, op.From, op.Purpose, op.Profile)
	default:
		err = i18n.Errorf("error.edit_unknown_op")
	}
	if err != nil {
		return nil, err
	}
	return encode(&doc)
}

// encode writes the document the way hergy writes its own settings: a
// two-space indent, so a file already in that form is not re-indented.
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

// addTurn appends one turns: line. Both ends have to be the human or a member
// the file holds, so a turn nobody can hand the turn to is not written; the
// same line twice is an error rather than a duplicate.
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

// removeTurn takes out one turns: line, and stops when the file does not hold
// it. A key that held a lone scalar holds one line; taking it out takes the key
// with it.
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
// holds.
func checkTurnEnds(root *yaml.Node, turn Edge) error {
	for _, id := range []NodeID{turn.From, turn.To} {
		if !knownEnd(root, id) {
			return i18n.Errorf("error.edit_turn_end", id.String())
		}
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

// addWorker appends one workers: entry. Its from: has to be a member the file
// holds, and the same member-and-purpose pair twice is an error (I12c).
func addWorker(root *yaml.Node, from, purpose, profile string) error {
	if _, ok := findMember(root, from); !ok {
		return i18n.Errorf("error.edit_worker_from", from)
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

// removeWorker takes out one workers: entry, by the member that hires it and
// its purpose (a worker's identity, I12c).
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

// setWorkerProfile changes the profile of one workers: entry, named by the
// member that hires it and its purpose.
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
