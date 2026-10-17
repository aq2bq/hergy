package profile

import (
	"maps"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// OpKind is which one edit an Op is.
type OpKind int

const (
	// OpSetKind writes one profile's kind.
	OpSetKind OpKind = iota
	// OpSetArgs writes one profile's args.
	OpSetArgs
	// OpSetEnv writes one profile's env.
	OpSetEnv
	// OpAdd appends one profile to the file.
	OpAdd
	// OpRemove takes one profile out of the file.
	OpRemove
	// OpRename writes another name on one profile's key.
	OpRename
)

// Op is one edit to a profiles.yaml: one profile, one change. Build it with the
// constructors below, which name the fields each kind reads.
//
// An op that names a profile the file does not hold is an error rather than a
// silent no-op, so a screen that acts on a stale page is told rather than
// writing nothing (I8).
type Op struct {
	Kind     OpKind
	Name     string            // the profile the op names
	KindName string            // OpSetKind, OpAdd: the kind it runs
	NewName  string            // OpRename: the name it is written with
	Args     []string          // OpSetArgs
	Env      map[string]string // OpSetEnv
}

// SetKind is the op that writes one profile's kind.
func SetKind(name, kind string) Op { return Op{Kind: OpSetKind, Name: name, KindName: kind} }

// SetArgs is the op that writes one profile's args, in the order given.
func SetArgs(name string, args []string) Op { return Op{Kind: OpSetArgs, Name: name, Args: args} }

// SetEnv is the op that writes one profile's env.
func SetEnv(name string, env map[string]string) Op {
	return Op{Kind: OpSetEnv, Name: name, Env: env}
}

// Add is the op that appends one profile.
func Add(name, kind string) Op { return Op{Kind: OpAdd, Name: name, KindName: kind} }

// Remove is the op that takes one profile out.
func Remove(name string) Op { return Op{Kind: OpRemove, Name: name} }

// Rename is the op that writes another name on one profile's key, keeping the
// position, the comments and everything under it.
func Rename(old, next string) Op {
	return Op{Kind: OpRename, Name: old, NewName: next}
}

// EditYAML changes one place in a profiles.yaml and writes the file back
// (spec/41 S1).
//
// It reads the yaml.Node tree to find the profile the op names, and the tree edit
// also says what value has to come out; the file itself is written by replacing
// the lines that value lives on, so every line the op does not touch comes back
// byte for byte — the comments, the blank lines, the order of the profiles, the
// indent a person wrote, a flow value on the key's line and even the line ending
// (CRLF) stay as they were. A new profile goes after the last one, the way a
// person appends one; args and env keep the form the file wrote them in.
//
// The written text is read back and compared with the tree the op means, so a
// line the edit placed on the wrong line is caught here rather than written. The
// shapes a line edit cannot rewrite (a `<<:` merge, an anchored value or a `*`
// alias) are refused with error.edit_shape instead of being rewritten whole; that
// sentence names the file as path writes it, so a screen hands in the path it
// shows (spec/42 R2).
//
// It does not check the profiles: whether a definition that reads one still reads
// is another gate's to stop (C2), and a screen checks before it writes rather
// than after.
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

	src := yamledit.New(text, path)
	if err := applyLines(src, root, op); err != nil {
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
	if err := yamledit.SameValues(&want, out, path); err != nil {
		return nil, err
	}
	return out, nil
}

// applyLines is the edit on the file's own lines: one profile, and every other
// line left as it was.
func applyLines(src *yamledit.Source, root *yaml.Node, op Op) error {
	profiles := yamledit.MappingValue(root, "profiles")
	switch op.Kind {
	case OpAdd:
		return addLines(src, root, profiles, op)
	case OpRemove:
		if yamledit.MappingKeyNode(profiles, op.Name) == nil {
			return i18n.Errorf("error.edit_no_profile", op.Name)
		}
		yamledit.RemoveKeyLines(src, profiles, op.Name)
		return nil
	case OpRename:
		return renameLines(src, profiles, op)
	case OpSetKind, OpSetArgs, OpSetEnv:
	default:
		return i18n.Errorf("error.edit_unknown_op")
	}
	item, ok := findProfile(profiles, op.Name)
	if !ok {
		return i18n.Errorf("error.edit_no_profile", op.Name)
	}
	switch op.Kind {
	case OpSetKind:
		return putKeyLines(src, item, "kind", op.KindName, "profiles."+op.Name+".kind")
	case OpSetArgs:
		return yamledit.ReplaceSeq(src, item, "args", op.Args)
	default:
		return yamledit.ReplaceMap(src, item, "env", envPairs(item, op.Env))
	}
}

// renameLines writes another name on one profile's key, on the key's own line,
// so the profile stays where it was and the comments above it stay with it.
func renameLines(src *yamledit.Source, profiles *yaml.Node, op Op) error {
	if profiles == nil || yamledit.MappingKeyNode(profiles, op.Name) == nil {
		return i18n.Errorf("error.edit_no_profile", op.Name)
	}
	if op.NewName == op.Name {
		return nil
	}
	if yamledit.MappingKeyNode(profiles, op.NewName) != nil {
		return i18n.Errorf("error.edit_profile_twice", op.NewName)
	}
	return yamledit.ReplaceKey(src, yamledit.MappingKeyNode(profiles, op.Name), op.NewName, "profiles."+op.Name)
}

// addLines appends one profile after the last one, lined up with the profiles
// beside it. A file with no profiles: key gets one, and a profiles: key whose
// value was written on its own line is opened for the new entry.
func addLines(src *yamledit.Source, root, profiles *yaml.Node, op Op) error {
	if err := src.Writable(op.Name, "profiles"); err != nil {
		return err
	}
	if yamledit.MappingKeyNode(profiles, op.Name) != nil {
		return i18n.Errorf("error.edit_profile_twice", op.Name)
	}
	if profiles == nil {
		src.Append("profiles:",
			yamledit.Spaces(2)+yamledit.ScalarLiteral(op.Name, 0)+":",
			yamledit.Spaces(4)+"kind: "+yamledit.ScalarLiteral(op.KindName, 0))
		return nil
	}
	if profiles.Style&yaml.FlowStyle != 0 || len(profiles.Content) == 0 {
		// profiles: was written as a flow collection, or with an empty value on
		// its own line: it becomes the key of the new entry below it.
		if profiles.Style&yaml.FlowStyle != 0 {
			return src.Shape("profiles")
		}
		keyNode := yamledit.MappingKeyNode(root, "profiles")
		src.ReplaceLine(keyNode.Line, []string{yamledit.Spaces(yamledit.LeadingSpaces(src.Line(keyNode.Line))) + "profiles:"})
		src.Insert(keyNode.Line,
			yamledit.Spaces(2)+yamledit.ScalarLiteral(op.Name, 0)+":",
			yamledit.Spaces(4)+"kind: "+yamledit.ScalarLiteral(op.KindName, 0))
		return nil
	}
	indent := yamledit.MappingIndent(profiles)
	src.Insert(yamledit.NodeEndLine(profiles),
		yamledit.Spaces(indent)+yamledit.ScalarLiteral(op.Name, 0)+":",
		yamledit.Spaces(indent+2)+"kind: "+yamledit.ScalarLiteral(op.KindName, 0))
	return nil
}

// putKeyLines writes a key's value on its own line, or adds the key below the
// mapping's last line when the mapping has none, lined up with the keys beside
// it.
func putKeyLines(src *yamledit.Source, item *yaml.Node, key, value, label string) error {
	if v := yamledit.MappingValue(item, key); v != nil {
		return yamledit.ReplaceScalar(src, v, value, label, item.Style&yaml.FlowStyle != 0)
	}
	if err := src.Writable(value, label); err != nil {
		return err
	}
	if item.Style&yaml.FlowStyle != 0 {
		// A profile written as a flow mapping ({kind: claude}) is not a shape
		// this editor adds a key to.
		return src.Shape(key)
	}
	src.Insert(yamledit.NodeEndLine(item), yamledit.Spaces(yamledit.MappingIndent(item))+key+": "+yamledit.ScalarLiteral(value, 0))
	return nil
}

// envPairs is a profile's env as the file writes it: the names the file already
// has, in the order it has them, and the names it does not have yet, sorted
// after them, so a save that changes one value does not move the others.
func envPairs(item *yaml.Node, env map[string]string) []yamledit.Pair {
	pairs := make([]yamledit.Pair, 0, len(env))
	seen := map[string]bool{}
	for _, pair := range nodesOf(yamledit.MappingValue(item, "env")) {
		if value, ok := env[pair.Key]; ok && !seen[pair.Key] {
			seen[pair.Key] = true
			pairs = append(pairs, yamledit.Pair{Key: pair.Key, Value: value})
		}
	}
	for _, name := range slices.Sorted(maps.Keys(env)) {
		if !seen[name] {
			pairs = append(pairs, yamledit.Pair{Key: name, Value: env[name]})
		}
	}
	return pairs
}

// nodesOf is a mapping's own keys in the order the file wrote them.
func nodesOf(v *yaml.Node) []yamledit.Pair {
	if v == nil || v.Kind != yaml.MappingNode {
		return nil
	}
	out := make([]yamledit.Pair, 0, len(v.Content)/2)
	for i := 0; i+1 < len(v.Content); i += 2 {
		out = append(out, yamledit.Pair{Key: v.Content[i].Value})
	}
	return out
}

// findProfile is the profiles: entry named, and whether the file holds one.
func findProfile(profiles *yaml.Node, name string) (*yaml.Node, bool) {
	if name == "" || profiles == nil || profiles.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(profiles.Content); i += 2 {
		if strings.TrimSpace(profiles.Content[i].Value) == name {
			return profiles.Content[i+1], true
		}
	}
	return nil, false
}

// applyTree is the edit a reader means, on the tree: the value the written lines
// have to read back as.
func applyTree(root *yaml.Node, op Op) error {
	profiles := ensureMapping(root, "profiles")
	switch op.Kind {
	case OpSetKind, OpSetArgs, OpSetEnv:
		item, ok := findProfile(profiles, op.Name)
		if !ok {
			return i18n.Errorf("error.edit_no_profile", op.Name)
		}
		switch op.Kind {
		case OpSetKind:
			putScalar(item, "kind", op.KindName)
		case OpSetArgs:
			putValue(item, "args", seqValue(op.Args))
		default:
			putValue(item, "env", mapValue(op.Env))
		}
		return nil
	case OpAdd:
		if _, ok := findProfile(profiles, op.Name); ok {
			return i18n.Errorf("error.edit_profile_twice", op.Name)
		}
		profiles.Content = append(profiles.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: op.Name},
			&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "kind"},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: op.KindName},
			}})
		return nil
	case OpRemove:
		removeMapKey(profiles, op.Name)
		return nil
	case OpRename:
		return renameTree(profiles, op)
	default:
		return i18n.Errorf("error.edit_unknown_op")
	}
}

// renameTree is the rename on the tree: the key keeps its place in the mapping.
func renameTree(profiles *yaml.Node, op Op) error {
	for i := 0; i+1 < len(profiles.Content); i += 2 {
		key := profiles.Content[i]
		if key.Value == op.Name {
			if op.NewName != op.Name && yamledit.MappingKeyNode(profiles, op.NewName) != nil {
				return i18n.Errorf("error.edit_profile_twice", op.NewName)
			}
			key.Value = op.NewName
			return nil
		}
	}
	return i18n.Errorf("error.edit_no_profile", op.Name)
}

// ensureMapping is the mapping a key holds, making the key and an empty mapping
// when the file has none.
func ensureMapping(root *yaml.Node, key string) *yaml.Node {
	v := yamledit.MappingValue(root, key)
	if v == nil {
		v = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
		return v
	}
	if v.Kind != yaml.MappingNode {
		v.Kind = yaml.MappingNode
		v.Tag = "!!map"
		v.Value = ""
		v.Style = 0
		v.Content = nil
	}
	return v
}

// putScalar writes a scalar for a key of a mapping.
func putScalar(mapping *yaml.Node, key, value string) {
	putValue(mapping, key, scalarValue(value))
}

// putValue writes a value for a key of a mapping, taking the key out when the
// value is empty (hergy leaves out an empty args and env).
func putValue(mapping *yaml.Node, key string, value *yaml.Node) {
	if value == nil {
		removeMapKey(mapping, key)
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

// removeMapKey takes a key and its value out of a mapping.
func removeMapKey(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// scalarValue is one scalar value.
func scalarValue(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// seqValue is a sequence's items, or nil when there are none.
func seqValue(items []string) *yaml.Node {
	if len(items) == 0 {
		return nil
	}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, item := range items {
		seq.Content = append(seq.Content, scalarValue(item))
	}
	return seq
}

// mapValue is a mapping's keys and values, or nil when there are none.
func mapValue(values map[string]string) *yaml.Node {
	if len(values) == 0 {
		return nil
	}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, name := range slices.Sorted(maps.Keys(values)) {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name},
			scalarValue(values[name]))
	}
	return mapping
}
