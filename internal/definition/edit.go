package definition

import (
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// OpKind is which one edit an Op is.
type OpKind int

const (
	// OpSetDescription writes team.yaml's description.
	OpSetDescription OpKind = iota
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
	// OpSetWorkerPurpose changes one worker's purpose.
	OpSetWorkerPurpose
	// OpAddMember appends one members: entry.
	OpAddMember
	// OpRemoveMember removes one member, with its turns and workers.
	OpRemoveMember
	// OpRenameMember changes one member's name, and every turn and worker that
	// names it.
	OpRenameMember
	// OpMoveWorker moves one worker to another member, keeping its place in the
	// workers: list.
	OpMoveWorker
)

// Op is one edit to team.yaml: one place, one change. Build it with the
// constructors below, which name the fields each kind reads.
//
// An Op that names a member, a turn or a worker the file does not hold is an
// error rather than a silent no-op, so a screen that acts on a stale row is
// told rather than writing nothing (I12).
type Op struct {
	Kind        OpKind
	Description string // OpSetDescription
	Member      string // OpSetMemberRole, OpSetMemberProfile
	Role        string // OpSetMemberRole
	Profile     string // OpSetMemberProfile, OpAddWorker, OpSetWorkerProfile
	Turn        Edge   // OpAddTurn, OpRemoveTurn
	From        string // OpAddWorker, OpRemoveWorker, OpSetWorkerProfile, OpSetWorkerPurpose
	Purpose     string // OpAddWorker, OpRemoveWorker, OpSetWorkerProfile, OpSetWorkerPurpose (the worker's identity)
	// NewPurpose is the purpose an OpSetWorkerPurpose writes in place of the
	// worker's own.
	NewPurpose string // OpSetWorkerPurpose
	// NewMember is the name an OpRenameMember writes in place of the member's
	// old name.
	NewMember string // OpRenameMember
	// To is the member an OpMoveWorker moves the worker to.
	To string // OpMoveWorker
}

// Description is the op that writes team.yaml's description.
func Description(text string) Op { return Op{Kind: OpSetDescription, Description: text} }

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

// WorkerPurpose is the op that changes one worker's purpose.
func WorkerPurpose(from, old, next string) Op {
	return Op{Kind: OpSetWorkerPurpose, From: from, Purpose: old, NewPurpose: next}
}

// AddMember is the op that appends one member. The three fields are required;
// the name is the short name, and the herdr name (<team>-<name>) is checked
// where the file is written, from the team.yaml's own directory. A role that no
// role document holds is allowed: the team definition's check reports it
// (d-a575).
func AddMember(name, role, profile string) Op {
	return Op{Kind: OpAddMember, Member: strings.TrimSpace(name), Role: strings.TrimSpace(role), Profile: strings.TrimSpace(profile)}
}

// RemoveMember is the op that removes one member, its turns in both directions
// and the workers it hires.
func RemoveMember(name string) Op {
	return Op{Kind: OpRemoveMember, Member: strings.TrimSpace(name)}
}

// RenameMember is the op that changes one member's name. Every turn and every
// worker that names the old name is rewritten.
func RenameMember(old, next string) Op {
	return Op{Kind: OpRenameMember, Member: strings.TrimSpace(old), NewMember: strings.TrimSpace(next)}
}

// MoveWorker is the op that moves one worker to another member. The worker keeps
// its place in the workers: list; it is not removed and added.
func MoveWorker(from, purpose, to string) Op {
	return Op{Kind: OpMoveWorker, From: strings.TrimSpace(from), Purpose: strings.TrimSpace(purpose), To: strings.TrimSpace(to)}
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
// error.edit_shape instead of being rewritten whole; that sentence names the
// file as path writes it, so a screen hands in the path it shows (spec/42 R2).
//
// It does not check the definition: whether the result still reads is another
// gate's to stop (C2), and a screen checks before it writes rather than after.
func EditYAML(text []byte, path string, op Op) ([]byte, error) {
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

	if op.Kind == OpRemoveMember {
		return removeMemberEdit(text, path, op.Member)
	}

	src := yamledit.New(text, path)
	if err := applyLines(src, root, op, path); err != nil {
		return nil, err
	}

	var want yaml.Node
	if err := yaml.Unmarshal(text, &want); err != nil {
		return nil, i18n.Errorf("error.edit_yaml", path, err)
	}
	if err := applyTree(want.Content[0], op); err != nil {
		return nil, err
	}

	out := src.Bytes()
	if err := yamledit.SameValues(&want, out, path, teamSaveBug()); err != nil {
		return nil, err
	}
	return out, nil
}

// teamSaveBug is what a save of team.yaml says when hergy's own write does not
// read back: the screen that stopped, and the YAML tab that edits the same file
// by hand (d-040e E3).
func teamSaveBug() yamledit.Bug {
	return yamledit.Bug{
		Operation:  i18n.T("bug.operation_team_save"),
		Workaround: i18n.T("bug.workaround_team_yaml"),
	}
}

// applyTree is the edit a reader means, on the tree: the value the written
// lines have to read back as. It is the tree edit C3 was first written with.
func applyTree(root *yaml.Node, op Op) error {
	switch op.Kind {
	case OpSetDescription:
		putKey(root, "description", op.Description)
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
	case OpSetWorkerPurpose:
		return setWorkerPurpose(root, op.From, op.Purpose, op.NewPurpose)
	case OpAddMember:
		return addMember(root, op.Member, op.Role, op.Profile)
	case OpRenameMember:
		return renameMember(root, op.Member, op.NewMember)
	case OpMoveWorker:
		return moveWorker(root, op.From, op.Purpose, op.To)
	case OpRemoveMember:
		return removeMemberItem(root, op.Member)
	default:
		return i18n.Errorf("error.edit_unknown_op")
	}
}

// applyLines is the edit on the file's own lines: one place, and every other
// line left as it was.
func applyLines(src *yamledit.Source, root *yaml.Node, op Op, path string) error {
	switch op.Kind {
	case OpSetDescription:
		return setDescriptionLines(src, root, op.Description)
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
	case OpSetWorkerPurpose:
		return setWorkerPurposeLines(src, root, op.From, op.Purpose, op.NewPurpose)
	case OpAddMember:
		return addMemberLines(src, root, op.Member, op.Role, op.Profile, path)
	case OpRenameMember:
		return renameMemberLines(src, root, op.Member, op.NewMember, path)
	case OpMoveWorker:
		return moveWorkerLines(src, root, op.From, op.Purpose, op.To)
	case OpRemoveMember:
		return removeMemberItemLines(src, root, op.Member)
	default:
		return i18n.Errorf("error.edit_unknown_op")
	}
}

// setDescriptionLines writes the description on its own line, or, when the file
// has no description:, inserts the line after version: (at the head when there
// is none either).
func setDescriptionLines(src *yamledit.Source, root *yaml.Node, value string) error {
	if v := yamledit.MappingValue(root, "description"); v != nil {
		return yamledit.ReplaceScalar(src, v, value, "description", false)
	}
	if err := src.Writable(value, "description"); err != nil {
		return err
	}
	at := 0
	if key := yamledit.MappingKeyNode(root, "version"); key != nil {
		at = key.Line
	}
	src.Insert(at, "description: "+yamledit.ScalarLiteral(value, 0))
	return nil
}

// setMemberLines changes one key of the member named, or stops when the file
// has no such member.
func setMemberLines(src *yamledit.Source, root *yaml.Node, member, key, value string) error {
	item, ok := findMember(root, member)
	if !ok {
		return i18n.Errorf("error.edit_no_member", member)
	}
	return putKeyLines(src, item, key, value)
}

// putKeyLines writes a key's value on its own line, or adds the key below the
// mapping's last line when the mapping has none, lined up with the keys beside
// it.
func putKeyLines(src *yamledit.Source, item *yaml.Node, key, value string) error {
	if v := yamledit.MappingValue(item, key); v != nil {
		return yamledit.ReplaceScalar(src, v, value, key, item.Style&yaml.FlowStyle != 0)
	}
	if err := src.Writable(value, key); err != nil {
		return err
	}
	src.Insert(yamledit.NodeEndLine(item), yamledit.Spaces(yamledit.MappingIndent(item))+key+": "+yamledit.ScalarLiteral(value, 0))
	return nil
}

// addTurnLines appends one turns: line. Both ends have to be the human or a
// member the file holds, so a turn nobody can hand the turn to is not written;
// the same line twice is an error rather than a duplicate.
func addTurnLines(src *yamledit.Source, root *yaml.Node, turn Edge) error {
	if err := checkTurnEnds(root, turn); err != nil {
		return err
	}
	if err := checkTurnNew(root, turn); err != nil {
		return err
	}
	v := yamledit.MappingValue(root, "turns")
	if v == nil {
		src.Append("turns:", "  - "+turn.String())
		return nil
	}
	place, err := yamledit.BlockPlace(src, root, "turns", v)
	if err != nil {
		return err
	}
	src.Insert(place.AfterLast, yamledit.Spaces(place.DashCol)+"- "+turn.String())
	return nil
}

// checkTurnNew is whether a turn is already in the file, the way the file
// writes it (a lone scalar is one turn).
func checkTurnNew(root *yaml.Node, turn Edge) error {
	v := yamledit.MappingValue(root, "turns")
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
func removeTurnLines(src *yamledit.Source, root *yaml.Node, turn Edge) error {
	v := yamledit.MappingValue(root, "turns")
	if v == nil {
		return i18n.Errorf("error.edit_no_turn", turn.String())
	}
	if v.Kind == yaml.ScalarNode {
		if !sameTurn(v, turn) {
			return i18n.Errorf("error.edit_no_turn", turn.String())
		}
		yamledit.RemoveKeyLines(src, root, "turns")
		return nil
	}
	if v.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_turn", turn.String())
	}
	for _, item := range v.Content {
		if sameTurn(item, turn) {
			return yamledit.RemoveItemLines(src, root, "turns", v, item)
		}
	}
	return i18n.Errorf("error.edit_no_turn", turn.String())
}

// addWorkerLines appends one workers: entry. Its from: has to be a member the
// file holds, and the same member-and-purpose pair twice is an error (I12c).
// The new entry is written the way the entries beside it are (a flow entry when
// they are).
func addWorkerLines(src *yamledit.Source, root *yaml.Node, from, purpose, profile string) error {
	if _, ok := findMember(root, from); !ok {
		return i18n.Errorf("error.graph_worker_from", from)
	}
	if err := checkWorkerNew(root, from, purpose); err != nil {
		return err
	}
	if err := src.Writable(purpose, "workers"); err != nil {
		return err
	}
	v := yamledit.MappingValue(root, "workers")
	if v == nil {
		src.Append("workers:",
			"  - from: "+yamledit.ScalarLiteral(from, 0),
			"    purpose: "+yamledit.ScalarLiteral(purpose, 0),
			"    profile: "+yamledit.ScalarLiteral(profile, 0))
		return nil
	}
	place, err := yamledit.BlockPlace(src, root, "workers", v)
	if err != nil {
		return err
	}
	if sequenceFlow(v) {
		src.Insert(place.AfterLast, yamledit.Spaces(place.DashCol)+"- {from: "+yamledit.ScalarLiteral(from, 0)+", purpose: "+yamledit.ScalarLiteral(purpose, 0)+", profile: "+yamledit.ScalarLiteral(profile, 0)+"}")
		return nil
	}
	src.Insert(place.AfterLast,
		yamledit.Spaces(place.DashCol)+"- from: "+yamledit.ScalarLiteral(from, 0),
		yamledit.Spaces(place.DashCol+2)+"purpose: "+yamledit.ScalarLiteral(purpose, 0),
		yamledit.Spaces(place.DashCol+2)+"profile: "+yamledit.ScalarLiteral(profile, 0))
	return nil
}

// checkWorkerNew is whether the member already hires a worker for that purpose.
func checkWorkerNew(root *yaml.Node, from, purpose string) error {
	seq := yamledit.MappingValue(root, "workers")
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

// removeWorkerLines takes out one workers: entry, by the member that hires it
// and its purpose (a worker's identity, I12c).
func removeWorkerLines(src *yamledit.Source, root *yaml.Node, from, purpose string) error {
	seq := yamledit.MappingValue(root, "workers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_worker", purpose, from)
	}
	for _, item := range seq.Content {
		if f, p, ok := workerKey(item); ok && f == from && p == purpose {
			return yamledit.RemoveItemLines(src, root, "workers", seq, item)
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// setWorkerProfileLines changes the profile of one workers: entry, named by the
// member that hires it and its purpose.
func setWorkerProfileLines(src *yamledit.Source, root *yaml.Node, from, purpose, profile string) error {
	seq := yamledit.MappingValue(root, "workers")
	if seq != nil && seq.Kind == yaml.SequenceNode {
		for _, item := range seq.Content {
			if f, p, ok := workerKey(item); ok && f == from && p == purpose {
				return putKeyLines(src, item, "profile", profile)
			}
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// setWorkerPurposeLines changes the purpose of one workers: entry, named by the
// member that hires it and the purpose the file holds. Only the entry's own
// purpose: line moves; every other line is left as it was.
func setWorkerPurposeLines(src *yamledit.Source, root *yaml.Node, from, purpose, next string) error {
	seq := yamledit.MappingValue(root, "workers")
	if seq != nil && seq.Kind == yaml.SequenceNode {
		for _, item := range seq.Content {
			if f, p, ok := workerKey(item); ok && f == from && p == purpose {
				return putKeyLines(src, item, "purpose", next)
			}
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
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
		name := yamledit.MappingValue(item, "name")
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
	value := yamledit.MappingValue(root, "turns")
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
	seq := yamledit.MappingValue(root, "workers")
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
	seq := yamledit.MappingValue(root, "workers")
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

// setWorkerPurpose changes the purpose of one workers: entry, on the tree (the
// intended value).
func setWorkerPurpose(root *yaml.Node, from, purpose, next string) error {
	seq := yamledit.MappingValue(root, "workers")
	if seq != nil && seq.Kind == yaml.SequenceNode {
		for _, item := range seq.Content {
			if f, p, ok := workerKey(item); ok && f == from && p == purpose {
				putKey(item, "purpose", next)
				return nil
			}
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// workerKey is a workers: entry's identity: the member that hires it and its
// purpose.
func workerKey(item *yaml.Node) (from, purpose string, ok bool) {
	fromNode := yamledit.MappingValue(item, "from")
	purposeNode := yamledit.MappingValue(item, "purpose")
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

// sequenceOf is the items a key writes as a sequence, or nil when the key is
// absent or another shape.
func sequenceOf(root *yaml.Node, key string) []*yaml.Node {
	value := yamledit.MappingValue(root, key)
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
	value := yamledit.MappingValue(root, key)
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

// teamNameOf is the team a team.yaml belongs to: the directory it stands in.
// A member's herdr name is <team>-<name>, and the check for it needs the name
// the file itself does not write (the folder's).
func teamNameOf(path string) string {
	return filepath.Base(filepath.Dir(path))
}

// addMemberLines appends one members: entry. The three fields are required, the
// herdr name has to fit its pattern, and the member's name has to be new in this
// definition. The entry is written the way the entries beside it are.
func addMemberLines(src *yamledit.Source, root *yaml.Node, name, role, profileName, path string) error {
	if name == "" || role == "" || profileName == "" {
		return i18n.Errorf("error.edit_member_required")
	}
	herdr := teamNameOf(path) + "-" + name
	if !profile.NamePattern.MatchString(herdr) {
		return i18n.Errorf("error.herdr_name_pattern", herdr)
	}
	if _, ok := findMember(root, name); ok {
		return i18n.Errorf("error.edit_member_twice", name)
	}
	for _, value := range []string{name, role, profileName} {
		if err := src.Writable(value, "members"); err != nil {
			return err
		}
	}
	v := yamledit.MappingValue(root, "members")
	if v == nil {
		src.Append("members:",
			"  - name: "+yamledit.ScalarLiteral(name, 0),
			"    role: "+yamledit.ScalarLiteral(role, 0),
			"    profile: "+yamledit.ScalarLiteral(profileName, 0))
		return nil
	}
	place, err := yamledit.BlockPlace(src, root, "members", v)
	if err != nil {
		return err
	}
	if sequenceFlow(v) {
		src.Insert(place.AfterLast, yamledit.Spaces(place.DashCol)+"- {name: "+
			yamledit.ScalarLiteral(name, 0)+", role: "+yamledit.ScalarLiteral(role, 0)+
			", profile: "+yamledit.ScalarLiteral(profileName, 0)+"}")
		return nil
	}
	src.Insert(place.AfterLast,
		yamledit.Spaces(place.DashCol)+"- name: "+yamledit.ScalarLiteral(name, 0),
		yamledit.Spaces(place.DashCol+2)+"role: "+yamledit.ScalarLiteral(role, 0),
		yamledit.Spaces(place.DashCol+2)+"profile: "+yamledit.ScalarLiteral(profileName, 0))
	return nil
}

// removeMemberItemLines takes one members: entry out. The member's turns and
// workers are removed before this, by the same single-purpose edits, so every
// step keeps the lines it does not touch.
func removeMemberItemLines(src *yamledit.Source, root *yaml.Node, member string) error {
	seq := yamledit.MappingValue(root, "members")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_member", member)
	}
	for _, item := range seq.Content {
		name := yamledit.MappingValue(item, "name")
		if name != nil && name.Kind == yaml.ScalarNode && strings.TrimSpace(name.Value) == member {
			return yamledit.RemoveItemLines(src, root, "members", seq, item)
		}
	}
	return i18n.Errorf("error.edit_no_member", member)
}

// renameMemberLines changes one member's name, and every turns: line and
// workers: from: that names it. Only those values' own lines move; comments,
// order and the rest stay as they were.
func renameMemberLines(src *yamledit.Source, root *yaml.Node, old, next, path string) error {
	item, ok := findMember(root, old)
	if !ok {
		return i18n.Errorf("error.edit_no_member", old)
	}
	if old == next {
		return nil
	}
	if next == "" {
		return i18n.Errorf("error.edit_member_required")
	}
	herdr := teamNameOf(path) + "-" + next
	if !profile.NamePattern.MatchString(herdr) {
		return i18n.Errorf("error.herdr_name_pattern", herdr)
	}
	if _, exists := findMember(root, next); exists {
		return i18n.Errorf("error.edit_member_twice", next)
	}
	if err := putKeyLines(src, item, "name", next); err != nil {
		return err
	}
	if err := renameTurnLines(src, root, old, next); err != nil {
		return err
	}
	return renameWorkerFromLines(src, root, old, next)
}

// renameTurnLines rewrites the member's end of every turns: line. A quoted
// value keeps its quotes, and a flow sequence stays on its own line.
func renameTurnLines(src *yamledit.Source, root *yaml.Node, old, next string) error {
	v := yamledit.MappingValue(root, "turns")
	if v == nil {
		return nil
	}
	flow := v.Style&yaml.FlowStyle != 0
	replace := func(node *yaml.Node) error {
		from, to, err := parseTurn(node.Value, "turns")
		if err != nil {
			return nil
		}
		changed := false
		if from.Kind == KindMember && from.Name == old {
			from.Name = next
			changed = true
		}
		if to.Kind == KindMember && to.Name == old {
			to.Name = next
			changed = true
		}
		if !changed {
			return nil
		}
		return yamledit.ReplaceScalar(src, node, (Edge{From: from, To: to}).String(), "turns", flow)
	}
	switch v.Kind {
	case yaml.ScalarNode:
		return replace(v)
	case yaml.SequenceNode:
		for _, turn := range v.Content {
			if err := replace(turn); err != nil {
				return err
			}
		}
	default:
		// A shape turns: does not read as is checked where the file is read.
	}
	return nil
}

// renameWorkerFromLines rewrites the from: of every worker the member hires.
func renameWorkerFromLines(src *yamledit.Source, root *yaml.Node, old, next string) error {
	for _, worker := range sequenceOf(root, "workers") {
		if from, _, ok := workerKey(worker); ok && from == old {
			if err := putKeyLines(src, worker, "from", next); err != nil {
				return err
			}
		}
	}
	return nil
}

// moveWorkerLines changes one worker's from: to another member. The entry keeps
// its place in the list and its other values; the target may not already hire a
// worker for the same purpose.
func moveWorkerLines(src *yamledit.Source, root *yaml.Node, from, purpose, to string) error {
	seq := yamledit.MappingValue(root, "workers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_worker", purpose, from)
	}
	if _, ok := findMember(root, to); !ok {
		return i18n.Errorf("error.edit_no_member", to)
	}
	if from == to {
		for _, item := range seq.Content {
			if f, p, ok := workerKey(item); ok && f == from && p == purpose {
				return nil
			}
		}
		return i18n.Errorf("error.edit_no_worker", purpose, from)
	}
	if err := checkWorkerNew(root, to, purpose); err != nil {
		return err
	}
	for _, item := range seq.Content {
		if f, p, ok := workerKey(item); ok && f == from && p == purpose {
			return putKeyLines(src, item, "from", to)
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// removeMemberEdit is OpRemoveMember: the member's turns in both directions and
// its workers go first, each by the single-purpose edit, and then the member's
// own entry. Every step keeps the lines it does not touch, comments included.
func removeMemberEdit(text []byte, path, member string) ([]byte, error) {
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
	if _, ok := findMember(root, member); !ok {
		return nil, i18n.Errorf("error.edit_no_member", member)
	}
	edges := turnsOf(root)
	workers := workersOf(root)
	out := text
	for _, edge := range edges {
		if edge.From.Kind == KindMember && edge.From.Name == member || edge.To.Kind == KindMember && edge.To.Name == member {
			next, err := EditYAML(out, path, RemoveTurn(edge.From, edge.To))
			if err != nil {
				return nil, err
			}
			out = next
		}
	}
	for _, worker := range workers {
		if worker[0] != member {
			continue
		}
		next, err := EditYAML(out, path, RemoveWorker(worker[0], worker[1]))
		if err != nil {
			return nil, err
		}
		out = next
	}
	return removeMemberEntry(out, path, member)
}

// removeMemberEntry takes the member's own members: entry out; removeMemberEdit
// has already taken the turns and workers.
func removeMemberEntry(text []byte, path, member string) ([]byte, error) {
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
	src := yamledit.New(text, path)
	if err := removeMemberItemLines(src, root, member); err != nil {
		return nil, err
	}
	var want yaml.Node
	if err := yaml.Unmarshal(text, &want); err != nil {
		return nil, i18n.Errorf("error.edit_yaml", path, err)
	}
	if err := removeMemberItem(want.Content[0], member); err != nil {
		return nil, err
	}
	out := src.Bytes()
	if err := yamledit.SameValues(&want, out, path, teamSaveBug()); err != nil {
		return nil, err
	}
	return out, nil
}

// turnsOf is every turn the file writes, a lone scalar included.
func turnsOf(root *yaml.Node) []Edge {
	v := yamledit.MappingValue(root, "turns")
	if v == nil {
		return nil
	}
	var items []*yaml.Node
	switch v.Kind {
	case yaml.ScalarNode:
		items = []*yaml.Node{v}
	case yaml.SequenceNode:
		items = v.Content
	default:
		// Another shape holds no turn the reader can name.
	}
	out := make([]Edge, 0, len(items))
	for _, item := range items {
		if from, to, err := parseTurn(item.Value, "turns"); err == nil {
			out = append(out, Edge{From: from, To: to})
		}
	}
	return out
}

// workersOf is every worker the file writes, by the member and purpose that
// name it.
func workersOf(root *yaml.Node) [][2]string {
	var out [][2]string
	for _, item := range sequenceOf(root, "workers") {
		if from, purpose, ok := workerKey(item); ok {
			out = append(out, [2]string{from, purpose})
		}
	}
	return out
}

// addMember is the tree form of OpAddMember.
func addMember(root *yaml.Node, name, role, profileName string) error {
	if name == "" || role == "" || profileName == "" {
		return i18n.Errorf("error.edit_member_required")
	}
	if _, ok := findMember(root, name); ok {
		return i18n.Errorf("error.edit_member_twice", name)
	}
	seq := ensureSequence(root, "members")
	seq.Content = append(seq.Content, memberEntry(name, role, profileName))
	return nil
}

// memberEntry is one members: entry as yaml nodes.
func memberEntry(name, role, profileName string) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		keyScalar("name"), valueScalar(name),
		keyScalar("role"), valueScalar(role),
		keyScalar("profile"), valueScalar(profileName),
	}}
}

// renameMember is the tree form of OpRenameMember.
func renameMember(root *yaml.Node, old, next string) error {
	item, ok := findMember(root, old)
	if !ok {
		return i18n.Errorf("error.edit_no_member", old)
	}
	if old == next {
		return nil
	}
	if next == "" {
		return i18n.Errorf("error.edit_member_required")
	}
	if _, exists := findMember(root, next); exists {
		return i18n.Errorf("error.edit_member_twice", next)
	}
	putKey(item, "name", next)
	if v := yamledit.MappingValue(root, "turns"); v != nil {
		var items []*yaml.Node
		switch v.Kind {
		case yaml.ScalarNode:
			items = []*yaml.Node{v}
		case yaml.SequenceNode:
			items = v.Content
		default:
			// Another shape holds no turn the reader can name.
		}
		for _, turn := range items {
			from, to, err := parseTurn(turn.Value, "turns")
			if err != nil {
				continue
			}
			if from.Kind == KindMember && from.Name == old {
				from.Name = next
			}
			if to.Kind == KindMember && to.Name == old {
				to.Name = next
			}
			putScalar(turn, (Edge{From: from, To: to}).String())
		}
	}
	for _, worker := range sequenceOf(root, "workers") {
		if from, _, ok := workerKey(worker); ok && from == old {
			putKey(worker, "from", next)
		}
	}
	return nil
}

// moveWorker is the tree form of OpMoveWorker.
func moveWorker(root *yaml.Node, from, purpose, to string) error {
	seq := yamledit.MappingValue(root, "workers")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_worker", purpose, from)
	}
	if _, ok := findMember(root, to); !ok {
		return i18n.Errorf("error.edit_no_member", to)
	}
	if from != to {
		if err := checkWorkerNew(root, to, purpose); err != nil {
			return err
		}
	}
	for _, item := range seq.Content {
		if f, p, ok := workerKey(item); ok && f == from && p == purpose {
			putKey(item, "from", to)
			return nil
		}
	}
	return i18n.Errorf("error.edit_no_worker", purpose, from)
}

// removeMemberItem is the tree form of the member's own entry removal.
func removeMemberItem(root *yaml.Node, member string) error {
	seq := yamledit.MappingValue(root, "members")
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return i18n.Errorf("error.edit_no_member", member)
	}
	for i, item := range seq.Content {
		name := yamledit.MappingValue(item, "name")
		if name != nil && name.Kind == yaml.ScalarNode && strings.TrimSpace(name.Value) == member {
			seq.Content = append(seq.Content[:i:i], seq.Content[i+1:]...)
			return nil
		}
	}
	return i18n.Errorf("error.edit_no_member", member)
}

// sequenceFlow is whether a sequence's items are written as flow mappings
// ({name: …}), so a new item is written the same way. addWorkerLines and
// addMemberLines share it.
func sequenceFlow(v *yaml.Node) bool {
	if v == nil || v.Kind != yaml.SequenceNode {
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
