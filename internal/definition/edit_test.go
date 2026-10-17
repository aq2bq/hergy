// These are the tests of editing team.yaml (C3): one op changes one place, and
// every line the op does not touch comes back byte for byte. The golden files
// under testdata/edit are the after of each op, and the assertions below also
// take the changed lines out of both sides and compare the rest, so a line that
// quietly moved or a comment that was lost fails even if the golden is stale.
package definition_test

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

var update = flag.Bool("update", false, "rewrite the golden files under testdata/edit")

// memberID is a member as a turns: line writes it.
func memberID(name string) definition.NodeID {
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// humanID is the human as a turns: line writes it.
func humanID() definition.NodeID {
	return definition.NodeID{Kind: definition.KindHuman, Name: "human"}
}

// editCase is one op, the golden file its after stands in, and the lines that
// leave and come in (so the rest of the file can be compared on its own).
type editCase struct {
	name    string
	op      definition.Op
	golden  string
	removed []string
	added   []string
}

func editCases() []editCase {
	return []editCase{
		{
			name:   "mission",
			op:     definition.Mission("あたらしい使命"),
			golden: "mission.yaml",
			removed: []string{
				`mission: "いまの使命" # 使命は 1 行`,
			},
			added: []string{
				`mission: "あたらしい使命" # 使命は 1 行`,
			},
		},
		{
			name:   "member-role-keeps-the-quotes",
			op:     definition.MemberRole("lead", "reviewer"),
			golden: "member_role.yaml",
			removed: []string{
				`    role: "lead" # ロールは引用してある`,
			},
			added: []string{
				`    role: "reviewer" # ロールは引用してある`,
			},
		},
		{
			name:   "member-profile",
			op:     definition.MemberProfile("spec", "coder"),
			golden: "member_profile.yaml",
			removed: []string{
				`    profile: reviewer`,
			},
			added: []string{
				`    profile: coder`,
			},
		},
		{
			name:   "add-turn",
			op:     definition.AddTurn(memberID("spec"), humanID()),
			golden: "add_turn.yaml",
			added: []string{
				`  - spec -> human`,
			},
		},
		{
			name:   "remove-turn",
			op:     definition.RemoveTurn(memberID("lead"), memberID("spec")),
			golden: "remove_turn.yaml",
			removed: []string{
				`  - lead -> spec`,
			},
		},
		{
			name:   "add-worker",
			op:     definition.AddWorker("spec", "試作", "planner"),
			golden: "add_worker.yaml",
			added: []string{
				`  - from: spec`,
				`    purpose: 試作`,
				`    profile: planner`,
			},
		},
		{
			name:   "remove-worker",
			op:     definition.RemoveWorker("lead", "実装"),
			golden: "remove_worker.yaml",
			removed: []string{
				`  - from: lead`,
				`    purpose: 実装`,
				`    profile: coder`,
			},
		},
		{
			name:   "worker-profile",
			op:     definition.WorkerProfile("spec", "調査", "coder"),
			golden: "worker_profile.yaml",
			removed: []string{
				`    profile: reader`,
			},
			added: []string{
				`    profile: coder`,
			},
		},
	}
}

// handCases is the same eight ops against the hand-written before: the file a
// person wrote, with blank lines, a four-space indent, comments lined up after
// the values, and CRLF.
func handCases() []editCase {
	return []editCase{
		{
			name:   "mission",
			op:     definition.Mission("あたらしい使命"),
			golden: "hand_mission.yaml",
			removed: []string{
				`mission: "いまの使命" # 使命は 1 行`,
			},
			added: []string{
				`mission: "あたらしい使命" # 使命は 1 行`,
			},
		},
		{
			name:   "member-role-keeps-the-quotes",
			op:     definition.MemberRole("lead", "reviewer"),
			golden: "hand_member_role.yaml",
			removed: []string{
				`      role: "lead" # ロールは引用してある`,
			},
			added: []string{
				`      role: "reviewer" # ロールは引用してある`,
			},
		},
		{
			name:   "member-profile",
			op:     definition.MemberProfile("spec", "coder"),
			golden: "hand_member_profile.yaml",
			removed: []string{
				`      profile: reviewer`,
			},
			added: []string{
				`      profile: coder`,
			},
		},
		{
			name:   "add-turn",
			op:     definition.AddTurn(memberID("spec"), humanID()),
			golden: "hand_add_turn.yaml",
			added: []string{
				`    - spec -> human`,
			},
		},
		{
			name:   "remove-turn",
			op:     definition.RemoveTurn(memberID("lead"), memberID("spec")),
			golden: "hand_remove_turn.yaml",
			removed: []string{
				`    - lead -> spec`,
			},
		},
		{
			name:   "add-worker",
			op:     definition.AddWorker("spec", "試作", "planner"),
			golden: "hand_add_worker.yaml",
			added: []string{
				`    - from: spec`,
				`      purpose: 試作`,
				`      profile: planner`,
			},
		},
		{
			name:   "remove-worker",
			op:     definition.RemoveWorker("lead", "実装"),
			golden: "hand_remove_worker.yaml",
			removed: []string{
				`    - from: lead`,
				`      purpose: 実装`,
				`      profile: coder`,
			},
		},
		{
			name:   "worker-profile",
			op:     definition.WorkerProfile("spec", "調査", "coder"),
			golden: "hand_worker_profile.yaml",
			removed: []string{
				`      profile: reader`,
			},
			added: []string{
				`      profile: coder`,
			},
		},
	}
}

func editGolden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "edit", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestEditYAMLKeepsEveryLineItDoesNotTouch: each op changes its one place, the
// after matches its golden, and taking the changed lines out of the before and
// the after leaves the same lines in the same order.
func TestEditYAMLKeepsEveryLineItDoesNotTouch(t *testing.T) {
	before := editGolden(t, "before.yaml")
	for _, tc := range editCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := definition.EditYAML([]byte(before), tc.op)
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := os.WriteFile(filepath.Join("testdata", "edit", tc.golden), got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want := editGolden(t, tc.golden)
			if string(got) != want {
				t.Errorf("EditYAML changed more than the op:\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
			assertOnly(t, before, string(got), tc.removed, tc.added)
		})
	}
}

// TestEditYAMLLeavesAFileInItsOwnForm: the before file is already the form hergy
// writes (a two-space indent, no blank lines, one space before a comment), so
// writing the same value back gives the file byte for byte. This is why every
// line the op does not touch can come back unchanged.
func TestEditYAMLLeavesAFileInItsOwnForm(t *testing.T) {
	before := editGolden(t, "before.yaml")
	got, err := definition.EditYAML([]byte(before), definition.Mission("いまの使命"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != before {
		t.Errorf("writing the same mission rewrote the file:\n--- got ---\n%s", got)
	}
}

// TestEditYAMLStopsOnANameTheFileDoesNotHold: an op that names a member, a turn
// or a worker the file does not hold is an error, so a screen that acts on a
// stale row is told (I12).
func TestEditYAMLStopsOnANameTheFileDoesNotHold(t *testing.T) {
	before := editGolden(t, "before.yaml")
	cases := []struct {
		name string
		op   definition.Op
		want string
	}{
		{"no member", definition.MemberRole("nope", "lead"), i18n.Tf("error.edit_no_member", "nope")},
		{"no member profile", definition.MemberProfile("nope", "lead"), i18n.Tf("error.edit_no_member", "nope")},
		{"turn twice", definition.AddTurn(memberID("lead"), memberID("spec")), i18n.Tf("error.edit_turn_twice", "lead -> spec")},
		{"no turn", definition.RemoveTurn(memberID("spec"), humanID()), i18n.Tf("error.edit_no_turn", "spec -> human")},
		{"turn end is not a node", definition.AddTurn(memberID("nope"), humanID()), i18n.Tf("error.graph_edge_from", "nope -> human", "nope")},
		{"turn start is not a node", definition.AddTurn(humanID(), memberID("nope")), i18n.Tf("error.graph_edge_to", "human -> nope", "nope")},
		{"worker from is not a member", definition.AddWorker("nope", "試作", "planner"), i18n.Tf("error.graph_worker_from", "nope")},
		{"worker twice", definition.AddWorker("lead", "実装", "coder"), i18n.Tf("error.edit_worker_twice", "実装", "lead")},
		{"no worker", definition.RemoveWorker("lead", "試作"), i18n.Tf("error.edit_no_worker", "試作", "lead")},
		{"no worker profile", definition.WorkerProfile("lead", "試作", "coder"), i18n.Tf("error.edit_no_worker", "試作", "lead")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := definition.EditYAML([]byte(before), tc.op)
			if err == nil {
				t.Fatalf("EditYAML wrote %q, want an error", got)
			}
			if err.Error() != tc.want {
				t.Errorf("error = %q, want %q", err, tc.want)
			}
			if string(got) != "" {
				t.Errorf("EditYAML returned %q with the error", got)
			}
		})
	}
}

// assertOnly finds the one region the op changed, by taking the longest common
// head and tail off the before and the after, and checks that the region is
// exactly the lines the case says leave and the lines it says come in. A line
// that moved, a comment that was lost or an extra rewrite fails.
func assertOnly(t *testing.T, before, after string, removed, added []string) {
	t.Helper()
	b := splitLines(before)
	a := splitLines(after)
	head := 0
	for head < len(b) && head < len(a) && b[head] == a[head] {
		head++
	}
	tail := 0
	for tail < len(b)-head && tail < len(a)-head && b[len(b)-1-tail] == a[len(a)-1-tail] {
		tail++
	}
	gotRemoved := b[head : len(b)-tail]
	gotAdded := a[head : len(a)-tail]
	if !slices.Equal(gotRemoved, removed) {
		t.Errorf("the lines that leave = %q, want %q", gotRemoved, removed)
	}
	if !slices.Equal(gotAdded, added) {
		t.Errorf("the lines that come in = %q, want %q", gotAdded, added)
	}
}

func splitLines(text string) []string {
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// TestEditYAMLKeepsTheLinesOfAHandWrittenFile is the S1 golden: the file a
// person wrote (blank lines, a four-space indent, lined-up comments, CRLF, a
// `---`) is edited in its own form. Every line the op does not touch comes back
// byte for byte, the golden holds exactly the after, and the line ending does
// not change.
func TestEditYAMLKeepsTheLinesOfAHandWrittenFile(t *testing.T) {
	before := editGolden(t, "before_hand.yaml")
	if !strings.Contains(before, "\r\n") || !strings.Contains(before, "---\r\n") {
		t.Fatalf("before_hand.yaml is not the CRLF form with a document start")
	}
	for _, tc := range handCases() {
		t.Run(tc.name, func(t *testing.T) {
			got, err := definition.EditYAML([]byte(before), tc.op)
			if err != nil {
				t.Fatal(err)
			}
			if *update {
				if err := os.WriteFile(filepath.Join("testdata", "edit", tc.golden), got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want := editGolden(t, tc.golden)
			if string(got) != want {
				t.Errorf("EditYAML changed more than the op:\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
			assertOnly(t, stripCR(before), stripCR(string(got)), tc.removed, tc.added)
			if strings.Contains(string(got), "\r\n") != strings.Contains(before, "\r\n") {
				t.Errorf("the line ending changed: %q", got)
			}
			if strings.Contains(strings.ReplaceAll(string(got), "\r\n", ""), "\n") {
				t.Errorf("a CRLF line ending became a lone LF: %q", got)
			}
		})
	}
}

// TestEditYAMLLeavesAHandWrittenFileInItsOwnForm: writing the value that is
// already there gives the file back byte for byte, CRLF and all, whatever shape
// a person wrote it in.
func TestEditYAMLLeavesAHandWrittenFileInItsOwnForm(t *testing.T) {
	before := editGolden(t, "before_hand.yaml")
	got, err := definition.EditYAML([]byte(before), definition.Mission("いまの使命"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != before {
		t.Errorf("writing the same mission rewrote the file:\n--- got ---\n%s", got)
	}
}

// stripCR is the golden text without its CR, so a changed-line assertion reads
// the same whether the file is CRLF or LF.
func stripCR(text string) string { return strings.ReplaceAll(text, "\r\n", "\n") }

// TestEditYAMLRefusesAShapeItCannotRewrite: a value written as a block scalar,
// or a `<<:` merge that gives a mapping keys it does not write, is refused
// rather than rewritten whole.
func TestEditYAMLRefusesAShapeItCannotRewrite(t *testing.T) {
	cases := []struct {
		name   string
		before string
		op     definition.Op
		label  string
	}{
		{
			name:   "a block mission",
			before: "version: 1\nmission: |\n  いまの使命\nmembers:\n  - name: lead\n    role: lead\n    profile: planner\n",
			op:     definition.Mission("あたらしい使命"),
			label:  "mission",
		},
		{
			name:   "a block role",
			before: "version: 1\nmission: m\nmembers:\n  - name: lead\n    role: |\n      lead\n    profile: planner\n",
			op:     definition.MemberRole("lead", "spec"),
			label:  "role",
		},
		{
			name:   "a merge key",
			before: "base: &b\n  role: lead\nversion: 1\nmission: m\nmembers:\n  - <<: *b\n    name: lead\n    profile: planner\n",
			op:     definition.MemberRole("lead", "spec"),
			label:  "メンバーの合成",
		},
		{
			name:   "a value with a line break",
			before: "version: 1\nmission: m\nmembers:\n  - name: lead\n    role: lead\n    profile: planner\n",
			op:     definition.Mission("one\ntwo"),
			label:  "mission",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := definition.EditYAML([]byte(tc.before), tc.op)
			if err == nil {
				t.Fatalf("EditYAML wrote %q, want an error", got)
			}
			if want := i18n.Tf("error.edit_shape", tc.label); err.Error() != want {
				t.Errorf("error = %q, want %q", err, want)
			}
		})
	}
}

// TestEditYAMLAddsAKeyTheFileDoesNotHave: a file without mission:, turns: or a
// member's role: gets the key on the line it belongs on, and the rest of the
// file keeps its lines.
func TestEditYAMLAddsAKeyTheFileDoesNotHave(t *testing.T) {
	members := "members:\n  - name: lead\n    role: lead\n    profile: planner\n"
	cases := []struct {
		name   string
		before string
		op     definition.Op
		want   string
	}{
		{
			name:   "mission after version",
			before: "version: 1\n" + members,
			op:     definition.Mission("あたらしい使命"),
			want:   "version: 1\nmission: あたらしい使命\n" + members,
		},
		{
			name:   "mission at the head",
			before: members,
			op:     definition.Mission("あたらしい使命"),
			want:   "mission: あたらしい使命\n" + members,
		},
		{
			name:   "turns at the end",
			before: "version: 1\n" + members,
			op:     definition.AddTurn(memberID("lead"), humanID()),
			want:   "version: 1\n" + members + "turns:\n  - lead -> human\n",
		},
		{
			name:   "workers at the end",
			before: "version: 1\n" + members,
			op:     definition.AddWorker("lead", "試作", "planner"),
			want:   "version: 1\n" + members + "workers:\n  - from: lead\n    purpose: 試作\n    profile: planner\n",
		},
		{
			name:   "a member's role",
			before: "version: 1\nmission: m\nmembers:\n  - name: lead\n    profile: planner\n",
			op:     definition.MemberRole("lead", "lead"),
			want:   "version: 1\nmission: m\nmembers:\n  - name: lead\n    profile: planner\n    role: lead\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := definition.EditYAML([]byte(tc.before), tc.op)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("EditYAML = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEditYAMLKeepsTheShapeTheFileWroteASequenceIn: a flow worker stays a flow
// worker when one is added, and a turns: written on one line (a lone scalar or
// a flow sequence) is written as a block when it is added to, and stays a flow
// sequence when one is taken out of it.
func TestEditYAMLKeepsTheShapeTheFileWroteASequenceIn(t *testing.T) {
	head := "version: 1\nmission: m\nmembers:\n  - name: lead\n    role: lead\n    profile: planner\n  - name: spec\n    role: spec\n    profile: reviewer\n"
	cases := []struct {
		name   string
		before string
		op     definition.Op
		want   string
	}{
		{
			name:   "a flow worker stays flow",
			before: head + "workers:\n  - {from: lead, purpose: 実装, profile: coder}\n",
			op:     definition.AddWorker("spec", "試作", "planner"),
			want:   head + "workers:\n  - {from: lead, purpose: 実装, profile: coder}\n  - {from: spec, purpose: 試作, profile: planner}\n",
		},
		{
			name:   "a scalar turns is written as a block",
			before: head + "turns: human -> lead\n",
			op:     definition.AddTurn(memberID("lead"), humanID()),
			want:   head + "turns:\n  - human -> lead\n  - lead -> human\n",
		},
		{
			name:   "a flow turns is written as a block",
			before: head + "turns: [human -> lead]\n",
			op:     definition.AddTurn(memberID("lead"), humanID()),
			want:   head + "turns:\n  - human -> lead\n  - lead -> human\n",
		},
		{
			name:   "a flow turns keeps flow when one leaves",
			before: head + "turns: [human -> lead, lead -> human]\n",
			op:     definition.RemoveTurn(memberID("lead"), humanID()),
			want:   head + "turns: [human -> lead]\n",
		},
		{
			name:   "the last turn leaves the key empty",
			before: head + "turns:\n  - human -> lead\n",
			op:     definition.RemoveTurn(humanID(), memberID("lead")),
			want:   head + "turns: []\n",
		},
		{
			name:   "the last worker leaves the key empty",
			before: head + "workers:\n  - from: lead\n    purpose: 実装\n    profile: coder\n",
			op:     definition.RemoveWorker("lead", "実装"),
			want:   head + "workers: []\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := definition.EditYAML([]byte(tc.before), tc.op)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("EditYAML = %q, want %q", got, tc.want)
			}
		})
	}
}
