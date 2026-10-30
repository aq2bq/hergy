package yamledit

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// tree is a small document with the two shapes a write has to tell apart: a
// block sequence with a blank line and a comment, and a flow sequence.
const tree = `version: 1

# 並び
items:
  - a
  - b

  # c は最後
  - c
flowed: [x, y]
`

// nodes is the document's own tree.
func nodes(t *testing.T, text string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Content[0]
}

// item is one item of one key's sequence.
func item(t *testing.T, root *yaml.Node, key string, i int) *yaml.Node {
	t.Helper()
	v := MappingValue(root, key)
	if v == nil || v.Kind != yaml.SequenceNode || i >= len(v.Content) {
		t.Fatalf("no item %d of %s", i, key)
	}
	return v.Content[i]
}

// TestReplaceNodeWritesOneItem: the lines of one item are written, and every
// other line — the blank line, the comment above the next item, the flow
// sequence — comes back as it was.
func TestReplaceNodeWritesOneItem(t *testing.T) {
	root := nodes(t, tree)
	src := New([]byte(tree), "x.yaml")
	ReplaceNode(src, item(t, root, "items", 1), []string{"  - B", "    note: 書き換えた"})
	out := string(src.Bytes())
	want := strings.Replace(tree, "  - b\n", "  - B\n    note: 書き換えた\n", 1)
	if out != want {
		t.Errorf("ReplaceNode wrote\n%q\nwant\n%q", out, want)
	}
	for _, kept := range []string{"\n\n# 並び", "\n\n  # c は最後", "flowed: [x, y]"} {
		if !strings.Contains(out, kept) {
			t.Errorf("the write changed %q:\n%s", kept, out)
		}
	}
}

// TestInsertAfterNodePutsLinesAfterOneItem: the lines go after the item's own
// lines, before the blank line that follows.
func TestInsertAfterNodePutsLinesAfterOneItem(t *testing.T) {
	root := nodes(t, tree)
	src := New([]byte(tree), "x.yaml")
	InsertAfterNode(src, item(t, root, "items", 0), "  - a2")
	out := string(src.Bytes())
	want := strings.Replace(tree, "  - a\n", "  - a\n  - a2\n", 1)
	if out != want {
		t.Errorf("InsertAfterNode wrote\n%q\nwant\n%q", out, want)
	}
}

// TestRemoveNodeTakesTheCommentWithIt: an item's lines go, with the comment the
// file wrote above it at its own indent; the blank line stays.
func TestRemoveNodeTakesTheCommentWithIt(t *testing.T) {
	root := nodes(t, tree)
	src := New([]byte(tree), "x.yaml")
	RemoveNode(src, item(t, root, "items", 2))
	out := string(src.Bytes())
	want := strings.Replace(tree, "\n  # c は最後\n  - c\n", "\n", 1)
	if out != want {
		t.Errorf("RemoveNode wrote\n%q\nwant\n%q", out, want)
	}
}

// TestRemoveFlowKeyLinesTakesOneEntryOut: a flow mapping loses the entry's own
// text and one of the commas beside it, not the line the key stands on, whether
// the entries share the key's line or each stands on its own; a line the entry
// stood alone on goes with it, and a mapping left with no entry keeps its braces
// (ac-adb6).
func TestRemoveFlowKeyLinesTakesOneEntryOut(t *testing.T) {
	cases := []struct{ name, text, key, want string }{
		{
			name: "two entries on the key's line",
			text: "version: 1\nprofiles: {a: {kind: claude}, b: {kind: codex}}\n",
			key:  "a",
			want: "version: 1\nprofiles: {b: {kind: codex}}\n",
		},
		{
			name: "the entry alone on its line",
			text: "version: 1\nprofiles: {\n  a: {kind: claude},\n  b: {kind: codex},\n}\n",
			key:  "a",
			want: "version: 1\nprofiles: {\n  b: {kind: codex},\n}\n",
		},
		{
			name: "the first entry on the key's line",
			text: "version: 1\nprofiles: {a: {kind: claude},\n  b: {kind: codex}}\n",
			key:  "a",
			want: "version: 1\nprofiles: {\n  b: {kind: codex}}\n",
		},
		{
			name: "a value that holds a comma and a collection",
			text: "version: 1\nprofiles: {a: {kind: claude, args: [--x, --y]}, b: {kind: codex}}\n",
			key:  "a",
			want: "version: 1\nprofiles: {b: {kind: codex}}\n",
		},
		{
			name: "the only entry",
			text: "version: 1\nprofiles: {a: {kind: claude}}\n",
			key:  "a",
			want: "version: 1\nprofiles: {}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := nodes(t, tc.text)
			src := New([]byte(tc.text), "x.yaml")
			RemoveFlowKeyLines(src, MappingValue(root, "profiles"), tc.key)
			if got := string(src.Bytes()); got != tc.want {
				t.Errorf("RemoveFlowKeyLines wrote\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// TestNodeRangeIsTheNodesOwnLines: a mapping item's range starts at its first
// key's line and ends at the last line of the node's own text, so a block value
// under it is inside the range.

func TestNodeRangeIsTheNodesOwnLines(t *testing.T) {
	const text = "a:\n  - id: T1\n    text: |-\n      1 行目\n      2 行目\n  - id: T2\n"
	root := nodes(t, text)
	from, to := NodeRange(item(t, root, "a", 0))
	if from != 2 || to != 5 {
		t.Errorf("NodeRange = %d..%d, want 2..5", from, to)
	}
	src := New([]byte(text), "x.yaml")
	ReplaceNode(src, item(t, root, "a", 0), []string{"  - id: T9", "    text: 書き換えた"})
	want := "a:\n  - id: T9\n    text: 書き換えた\n  - id: T2\n"
	if string(src.Bytes()) != want {
		t.Errorf("ReplaceNode wrote\n%q\nwant\n%q", string(src.Bytes()), want)
	}
}

// testBug is the sentence a write of this test reports with: what stopped, and
// the way around it, the way a screen hands them in (d-040e E3).
var testBug = Bug{Operation: "the save", Workaround: "edit the file itself"}

// TestSameValuesCatchesAWrongWrite: a write whose lines do not read as the
// intended tree is refused rather than written, and the sentence names hergy's
// bug and the way around it, not the person's file.
func TestSameValuesCatchesAWrongWrite(t *testing.T) {
	want := nodes(t, "a: 1\n")
	err := SameValues(want, []byte("a: 2\n"), "x.yaml", testBug)
	if err == nil {
		t.Fatal("a write that reads as another value was accepted")
	}
	if got, want := err.Error(), testBug.Errorf().Error(); got != want {
		t.Errorf("the refusal reads %q; want the internal-bug sentence %q", got, want)
	}
	if err := SameValues(want, []byte("a: 1\n"), "x.yaml", testBug); err != nil {
		t.Errorf("a write that reads as the intended tree was refused: %v", err)
	}
}

// TestSameValuesReadsADateAsTheTextItWasWrittenWith: hergy writes every date
// quoted and a person may write it plainly, and the two are the same value, so a
// hand-written document is not refused for the quotes. Another date is still
// another value.
func TestSameValuesReadsADateAsTheTextItWasWrittenWith(t *testing.T) {
	quoted := nodes(t, "date: \"2026-09-14\"\n")
	if err := SameValues(quoted, []byte("date: 2026-09-14\n"), "x.yaml", testBug); err != nil {
		t.Errorf("an unquoted date was refused: %v", err)
	}
	plain := nodes(t, "date: 2026-09-14\n")
	if err := SameValues(plain, []byte("date: \"2026-09-14\"\n"), "x.yaml", testBug); err != nil {
		t.Errorf("a quoted date was refused against a plain one: %v", err)
	}
	if err := SameValues(quoted, []byte("date: 2026-09-15\n"), "x.yaml", testBug); err == nil {
		t.Error("another date was accepted")
	}
}

// TestShapeKeyOfNamesTheShape: a merge key and an alias are the shapes a line
// write refuses, named by the key they stand under.
func TestShapeKeyOfNamesTheShape(t *testing.T) {
	root := nodes(t, "base: &base\n  a: 1\na:\n  <<: *base\n")
	if key, ok := ShapeKeyOf(root); !ok || key != "a" {
		t.Errorf("ShapeKeyOf = %q, %v, want a, true", key, ok)
	}
	root = nodes(t, "x: &x 1\na:\n  b: *x\n")
	if key, ok := ShapeKeyOf(root); !ok || key != "b" {
		t.Errorf("ShapeKeyOf = %q, %v, want b, true", key, ok)
	}
	root = nodes(t, "a:\n  b: c\n")
	if key, ok := ShapeKeyOf(root); ok {
		t.Errorf("ShapeKeyOf = %q, %v, want none", key, ok)
	}
}
