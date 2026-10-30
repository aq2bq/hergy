package profile_test

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/profile"
)

// hand is a profiles.yaml written by a person: a comment at the head, a blank
// line, a comment per profile, the profiles in the order they were written (not
// by name), one profile's args and env as flow collections on the key's line,
// another's as block lines, and a comment after a value (spec/41 S1).
const hand = `version: 1
# このプロジェクトのプロファイル（手で書いた形）

profiles:
  planner:
    # 計画と仕様
    kind: claude
    args: ["--model", "opus"]   # 重いほう
    env:
      A: one

  coder:
    # 実装
    kind: opencode
    args:
      - --y
      - --z
    env: {B: two}
  spare:
    kind: gemini
`

// TestEditYAMLWritesOnlyThePlaceThatMoved is spec/41 S1: each of the five ops
// changes its own place and not one byte of the rest of the file — the comments,
// the blank line, the order of the profiles, the closing quote of an untouched
// value, and the value form the file wrote (a flow collection on the key's line,
// or block lines under it).
func TestEditYAMLWritesOnlyThePlaceThatMoved(t *testing.T) {
	cases := []struct {
		name string
		op   profile.Op
		want string
	}{
		{
			name: "kind: the profile's own line, and nothing else",
			op:   profile.SetKind("planner", "codex"),
			want: strings.Replace(hand, "    kind: claude\n", "    kind: codex\n", 1),
		},
		{
			name: "args: a block list gains its item at the end",
			op:   profile.SetArgs("coder", []string{"--y", "--z", "--w"}),
			want: strings.Replace(hand, "      - --y\n      - --z\n", "      - --y\n      - --z\n      - --w\n", 1),
		},
		{
			name: "args: a flow list stays on the key's line, with its comment",
			op:   profile.SetArgs("planner", []string{"--model", "sonnet"}),
			want: strings.Replace(hand, `args: ["--model", "opus"]   # 重いほう`, "args: [--model, sonnet]   # 重いほう", 1),
		},
		{
			name: "env: a flow mapping stays on the key's line",
			op:   profile.SetEnv("coder", map[string]string{"B": "three", "C": "four"}),
			want: strings.Replace(hand, "env: {B: two}", "env: {B: three, C: four}", 1),
		},
		{
			name: "env: a block mapping keeps the file's own order",
			op:   profile.SetEnv("planner", map[string]string{"C": "two", "A": "one"}),
			want: strings.Replace(hand, "    env:\n      A: one\n", "    env:\n      A: one\n      C: two\n", 1),
		},
		{
			name: "add: the new profile goes after the last one",
			op:   profile.Add("newp", "opencode"),
			want: hand + "  newp:\n    kind: opencode\n",
		},
		{
			name: "remove: the profile's own lines go, and its comment above them",
			op:   profile.Remove("spare"),
			want: strings.Replace(hand, "  spare:\n    kind: gemini\n", "", 1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := profile.EditYAML([]byte(hand), "profiles.yaml", tc.op)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("EditYAML wrote\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// TestEditYAMLRefusesAProfileTheFileDoesNotHold: an op that names a profile the
// file has not got is an error, not a silent no-op or an append (I8).
func TestEditYAMLRefusesAProfileTheFileDoesNotHold(t *testing.T) {
	for _, op := range []profile.Op{
		profile.SetKind("nosuch", "claude"),
		profile.SetArgs("nosuch", []string{"--x"}),
		profile.SetEnv("nosuch", map[string]string{"A": "one"}),
		profile.Remove("nosuch"),
	} {
		if out, err := profile.EditYAML([]byte(hand), "profiles.yaml", op); err == nil {
			t.Errorf("op %+v wrote\n%q\nwant a refusal", op, out)
		}
	}
}

// TestEditYAMLRefusesAProfileAlreadyThere: adding a name hergy already resolves
// is an error rather than a second entry (I8).
func TestEditYAMLRefusesAProfileAlreadyThere(t *testing.T) {
	if out, err := profile.EditYAML([]byte(hand), "profiles.yaml", profile.Add("coder", "claude")); err == nil {
		t.Errorf("adding a name that is there wrote\n%q\nwant a refusal", out)
	}
}

// TestEditYAMLRemovesAProfileFromEveryShapeTheFileWrites: taking one profile out
// writes the file the file wrote, whatever shape that is (ac-adb6, r-430b). The
// last profile of a block mapping leaves `profiles: {}` on the key's own line,
// because a bare `profiles:` reads as a null value and not as the empty mapping
// the op means; a flow mapping loses the entry's own text and one of the commas
// beside it, not the line the key stands on. Every other line comes back byte
// for byte, and what is written reads back as the profile less.
func TestEditYAMLRemovesAProfileFromEveryShapeTheFileWrites(t *testing.T) {
	cases := []struct{ name, text, target, want string }{
		{
			name:   "the only profile of a block mapping",
			text:   "version: 1\nprofiles:\n  a:\n    kind: claude\n",
			target: "a",
			want:   "version: 1\nprofiles: {}\n",
		},
		{
			name:   "the only profile, with a document start and a comment above it",
			text:   "---\nversion: 1\nprofiles:\n  # 使うエンジン\n  a:\n    kind: claude\n",
			target: "a",
			want:   "---\nversion: 1\nprofiles: {}\n",
		},
		{
			name:   "the only profile, and the comment after the key stays",
			text:   "version: 1\nprofiles: # 使うエンジン\n  a:\n    kind: claude\n",
			target: "a",
			want:   "version: 1\nprofiles: {} # 使うエンジン\n",
		},
		{
			name:   "one of two block profiles",
			text:   "version: 1\nprofiles:\n  a:\n    kind: claude\n  b:\n    kind: codex\n",
			target: "b",
			want:   "version: 1\nprofiles:\n  a:\n    kind: claude\n",
		},
		{
			name:   "a flow mapping on the key's line",
			text:   "version: 1\nprofiles: {a: {kind: claude}, b: {kind: codex}}\n",
			target: "a",
			want:   "version: 1\nprofiles: {b: {kind: codex}}\n",
		},
		{
			name:   "the last of a flow mapping",
			text:   "version: 1\nprofiles: {a: {kind: claude}, b: {kind: codex}}\n",
			target: "b",
			want:   "version: 1\nprofiles: {a: {kind: claude}}\n",
		},
		{
			name:   "the only entry of a flow mapping",
			text:   "version: 1\nprofiles: {a: {kind: claude}}\n",
			target: "a",
			want:   "version: 1\nprofiles: {}\n",
		},
		{
			name:   "a flow mapping written one entry per line",
			text:   "version: 1\nprofiles: {\n  a: {kind: claude},\n  b: {kind: codex},\n}\n",
			target: "a",
			want:   "version: 1\nprofiles: {\n  b: {kind: codex},\n}\n",
		},
		{
			name:   "an entry whose args are a flow list",
			text:   "version: 1\nprofiles: {a: {kind: claude, args: [--x, --y]}, b: {kind: codex}}\n",
			target: "a",
			want:   "version: 1\nprofiles: {b: {kind: codex}}\n",
		},
		{
			name:   "a CRLF file",
			text:   "version: 1\r\nprofiles: {a: {kind: claude}, b: {kind: codex}}\r\n",
			target: "a",
			want:   "version: 1\r\nprofiles: {b: {kind: codex}}\r\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := profile.EditYAML([]byte(tc.text), "profiles.yaml", profile.Remove(tc.target))
			if err != nil {
				t.Fatalf("the delete stopped: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("EditYAML wrote\n%q\nwant\n%q", got, tc.want)
			}
			profiles, err := profile.ParseProfiles(string(got), "profiles.yaml")
			if err != nil {
				t.Fatalf("what it wrote does not read: %v\n%s", err, got)
			}
			if _, there := profiles[tc.target]; there {
				t.Errorf("what it wrote still reads %s", tc.target)
			}
		})
	}
}

// TestEditYAMLTakesAnEmptyArgsAndEnvOut: a profile with no args and no env has
// neither key, the way hergy writes a file (設計 3.7).

func TestEditYAMLTakesAnEmptyArgsAndEnvOut(t *testing.T) {
	out, err := profile.EditYAML([]byte(hand), "profiles.yaml", profile.SetArgs("coder", nil))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `args: ["--model", "opus"]   # 重いほう`) {
		t.Errorf("the profile beside it moved:\n%q", out)
	}
	if !strings.Contains(string(out), "env: {B: two}") {
		t.Errorf("the env beside it moved:\n%q", out)
	}
	profiles, err := profile.ParseProfiles(string(out), "profiles.yaml")
	if err != nil {
		t.Fatalf("what it wrote does not read: %v", err)
	}
	if len(profiles["coder"].Args) != 0 {
		t.Errorf("coder reads args %v, want none", profiles["coder"].Args)
	}
	if len(profiles["planner"].Args) != 2 {
		t.Errorf("planner reads args %v, want the two the file wrote", profiles["planner"].Args)
	}
}
