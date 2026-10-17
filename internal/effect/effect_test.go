// These are the tests of the effects of an edit (C4): which part of which
// member's prompt moves, the "保存すると" line, and the 渡し方 line per running
// team. The layer is a real one (a roles/ directory and a catalog over it), so a
// member's prompt is built through the same doors the launcher uses.
package effect_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/prompt"
	"github.com/aq2bq/hergy/internal/source"
)

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		os.Exit(1)
	}
	// The design's lines are Japanese; the tests read them as the design writes
	// them.
	if err := i18n.Use("ja"); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func oneRule(id, text string) string {
	return "version: 1\nsections:\n  - rules:\n      - id: " + id + "\n        text: |-\n          " + text + "\n"
}

// layer is a test's layer: the root roles/ stand in, and the demo definition's
// own directory.
type layer struct {
	root string
	team string
}

func newLayer(t *testing.T) layer {
	t.Helper()
	root := t.TempDir()
	l := layer{root: root, team: filepath.Join(root, "teams", "demo")}
	if err := os.MkdirAll(l.team, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "roles", "lead.yaml"), oneRule("L1", "lead doc"))
	write(t, filepath.Join(root, "roles", "spec.yaml"), oneRule("S1", "spec doc"))
	return l
}

func (l layer) catalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	return catalog.New(filepath.Join(l.root, "global"), l.root, l.root, source.Disk{},
		func(path string) string { return path })
}

// profiles is the two profiles a test team names, with two kinds so a profile
// change can move the launch.
func (l layer) profiles() map[string]profile.Profile {
	path := filepath.Join(l.team, "profiles.yaml")
	return map[string]profile.Profile{
		"claude-default": {Name: "claude-default", Kind: "claude", Path: path},
		"codex-default":  {Name: "codex-default", Kind: "codex", Path: path},
	}
}

func (l layer) read(t *testing.T, text string) *definition.Definition {
	t.Helper()
	def, err := definition.Read(definition.Input{
		YAML:         text,
		Name:         "demo",
		Layer:        source.LayerLocal,
		TeamYAMLPath: filepath.Join(l.team, "team.yaml"),
		TeamDir:      l.team,
		Profiles:     l.profiles(),
		Base:         l.team,
		Reader:       source.Disk{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return def
}

func (l layer) view(t *testing.T, def *definition.Definition) effect.View {
	t.Helper()
	return effect.View{
		Def:      def,
		Profiles: l.profiles(),
		Run: prompt.Run{
			Team:       "demo",
			Executable: "hergy",
			Base:       l.team,
			Reader:     source.Disk{},
			Sources:    l.catalog(t),
		},
	}
}

// run is the facts a prompt is built with, for the I13 check below.
func (l layer) run(t *testing.T) prompt.Run {
	return l.view(t, nil).Run
}

func memberID(name string) definition.NodeID {
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// teamText is the test team: two members, one worker lead may hire, and the
// shortest valid turns graph (段 5).
func teamText() string {
	return "version: 1\nmission: a mission for the test\nteam: demo\n" +
		"members:\n" +
		"  - name: lead\n    role: lead\n    profile: claude-default\n" +
		"  - name: spec\n    role: spec\n    profile: claude-default\n" +
		"workers:\n  - from: lead\n    purpose: 試作\n    profile: claude-default\n" +
		"turns:\n  - human -> lead\n  - lead -> spec\n  - spec -> lead\n  - lead -> human\n"
}

// TestOfTurnRemovalMovesOnlyTheMemberThatHandsIt is the I13 golden: taking
// lead -> spec out of the definition leaves lead's turns part carrying spec, no
// other member has a Change, and the prompt itself no longer hands the turn to
// spec (it is still in the roster, which did not move).
func TestOfTurnRemovalMovesOnlyTheMemberThatHandsIt(t *testing.T) {
	l := newLayer(t)
	before := l.read(t, teamText())
	edited, err := definition.EditYAML([]byte(teamText()), definition.RemoveTurn(memberID("lead"), memberID("spec")))
	if err != nil {
		t.Fatal(err)
	}
	after := l.read(t, string(edited))

	changes := effect.Of(l.view(t, before), l.view(t, after))
	want := []effect.Change{{
		Member: "lead",
		Parts: []effect.PartChange{{
			Part:    effect.Turns,
			Removed: []effect.Named{{Name: "spec", Role: "spec"}},
		}},
	}}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("Of = %+v, want %+v", changes, want)
	}

	preview := effect.Preview(changes)
	wantLine := "lead のプロンプトの『ターンを渡す相手』から spec（ロール spec）が消えます"
	if len(preview) != 1 || preview[0] != wantLine {
		t.Errorf("Preview = %q, want [%q]", preview, wantLine)
	}

	run := l.run(t)
	for i := range before.Graph.Members {
		bm := before.Graph.Members[i]
		am, ok := after.Graph.Member(bm.Name)
		if !ok {
			t.Fatalf("%s is not in the after definition", bm.Name)
		}
		bt, err := prompt.Member(before, &bm, run)
		if err != nil {
			t.Fatal(err)
		}
		at, err := prompt.Member(after, &am, run)
		if err != nil {
			t.Fatal(err)
		}
		if bm.Name == "lead" && bt.Text == at.Text {
			t.Errorf("lead's prompt did not move")
		}
		if bm.Name != "lead" && bt.Text != at.Text {
			t.Errorf("%s's prompt moved, but no Change says so", bm.Name)
		}
	}
	leadTurns := prompt.TurnsNote(prompt.TurnsOf(after, "lead", "demo"))
	if strings.Contains(leadTurns, "spec") {
		t.Errorf("lead's turns still name spec: %q", leadTurns)
	}
}

// TestOfProfileChangeMovesTheLaunch: naming another profile moves how the member
// is started, and nothing else.
func TestOfProfileChangeMovesTheLaunch(t *testing.T) {
	l := newLayer(t)
	before := l.read(t, teamText())
	edited, err := definition.EditYAML([]byte(teamText()), definition.MemberProfile("lead", "codex-default"))
	if err != nil {
		t.Fatal(err)
	}
	after := l.read(t, string(edited))

	changes := effect.Of(l.view(t, before), l.view(t, after))
	want := []effect.Change{{Member: "lead", Parts: []effect.PartChange{{Part: effect.Launch}}}}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("Of = %+v, want %+v", changes, want)
	}
}

// TestOfRoleChangeMovesTheRosterAndTheRoleDocument: a role change reads another
// document and changes the roster every member reads, so every member has a
// Change.
func TestOfRoleChangeMovesTheRosterAndTheRoleDocument(t *testing.T) {
	l := newLayer(t)
	before := l.read(t, teamText())
	edited, err := definition.EditYAML([]byte(teamText()), definition.MemberRole("spec", "lead"))
	if err != nil {
		t.Fatal(err)
	}
	after := l.read(t, string(edited))

	changes := effect.Of(l.view(t, before), l.view(t, after))
	if len(changes) != 2 {
		t.Fatalf("Of = %+v, want both members", changes)
	}
	byMember := map[string][]effect.PartChange{}
	for _, change := range changes {
		byMember[change.Member] = change.Parts
	}
	if !hasPart(byMember["spec"], effect.RoleDocument) {
		t.Errorf("spec's parts = %+v, want a role document part", byMember["spec"])
	}
	for _, member := range []string{"lead", "spec"} {
		if !hasPart(byMember[member], effect.Roster) {
			t.Errorf("%s's parts = %+v, want a roster part", member, byMember[member])
		}
	}
}

func hasPart(parts []effect.PartChange, part effect.Part) bool {
	for _, p := range parts {
		if p.Part == part {
			return true
		}
	}
	return false
}

// TestPreviewNamesThePartyTheWorkerAndThePart: the "保存すると" lines, one line
// naming one party, worker or part, and the one line that says nothing moves.
func TestPreviewNamesThePartyTheWorkerAndThePart(t *testing.T) {
	cases := []struct {
		name    string
		changes []effect.Change
		want    []string
	}{
		{
			name:    "nothing moves",
			changes: nil,
			want:    []string{"どのメンバーのプロンプトも変わりません"},
		},
		{
			name: "a turn to a member leaves",
			changes: []effect.Change{{Member: "lead", Parts: []effect.PartChange{{
				Part: effect.Turns, Removed: []effect.Named{{Name: "spec", Role: "spec"}},
			}}}},
			want: []string{"lead のプロンプトの『ターンを渡す相手』から spec（ロール spec）が消えます"},
		},
		{
			name: "the human comes in",
			changes: []effect.Change{{Member: "lead", Parts: []effect.PartChange{{
				Part: effect.Turns, Added: []effect.Named{{}},
			}}}},
			want: []string{"lead のプロンプトの『ターンを渡す相手』に 人間 が入ります"},
		},
		{
			name: "a worker leaves",
			changes: []effect.Change{{Member: "lead", Parts: []effect.PartChange{{
				Part: effect.Workers, Removed: []effect.Named{{Name: "試作"}},
			}}}},
			want: []string{"lead のプロンプトの『呼べるワーカー』から 試作 が消えます。hergy hire --purpose 試作 は断られます"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effect.Preview(tc.changes)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Preview = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestHandoverSaysWhatToDoPerRunningTeam: one line per running team, by what
// moved. The commands are the design's table.
func TestHandoverSaysWhatToDoPerRunningTeam(t *testing.T) {
	running := []effect.Running{{Team: "demo", Definition: "demo"}}
	cases := []struct {
		name    string
		changes []effect.Change
		want    string
	}{
		{
			name:    "nothing to take up",
			changes: nil,
			want:    "動いているチームに渡すものはありません",
		},
		{
			name:    "a turn moves",
			changes: turnChange("lead"),
			want:    "hergy reprompt lead --team demo",
		},
		{
			name:    "a worker moves",
			changes: workerChange("spec"),
			want:    "hergy reprompt spec --team demo",
		},
		{
			name:    "a role moves",
			changes: rosterChange("spec"),
			want:    "hergy reprompt --all --team demo",
		},
		{
			name:    "a launch moves",
			changes: launchChange("lead"),
			want:    "hergy refresh lead --team demo（会話は消えます）",
		},
		{
			name:    "env moves",
			changes: envChange("lead"),
			want:    "hergy down demo のあと、hergy up demo --as demo",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effect.Handover(tc.changes, running)
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("Handover = %q, want [%q]", got, tc.want)
			}
		})
	}
}

// TestHandoverWithoutARunningTeam: no running team gives the line that says the
// next hergy up uses the change.
func TestHandoverWithoutARunningTeam(t *testing.T) {
	got := effect.Handover(turnChange("lead"), nil)
	want := "動いているチームはありません。次の hergy up から使われます"
	if len(got) != 1 || got[0] != want {
		t.Errorf("Handover = %q, want [%q]", got, want)
	}
}

// commandPattern is a hergy command as the handover lines write one: the word
// hergy, then ASCII words, up to the Japanese note after it.
var commandPattern = regexp.MustCompile(`hergy(?: [\x21-\x7e]+)+`)

// TestHandoverCommandsAreRealCommands: every command a handover line prints is
// taken apart by the real command definitions (internal/options), so a line that
// drifts from hergy's own command line fails here (d-a8b1 追加の決め 5).
func TestHandoverCommandsAreRealCommands(t *testing.T) {
	running := []effect.Running{{Team: "demo", Definition: "demo"}}
	changesList := [][]effect.Change{
		turnChange("lead"),
		workerChange("spec"),
		rosterChange("spec"),
		launchChange("lead"),
		envChange("lead"),
	}
	for _, changes := range changesList {
		for _, line := range effect.Handover(changes, running) {
			commands := commandPattern.FindAllString(line, -1)
			if len(commands) == 0 {
				t.Errorf("no command in %q", line)
			}
			for _, command := range commands {
				argv := strings.Fields(command)[1:] // drop the word hergy
				if _, err := options.Parse(argv); err != nil {
					t.Errorf("the command %q in %q does not parse: %v", command, line, err)
				}
			}
		}
	}
}

func turnChange(member string) []effect.Change {
	return []effect.Change{{Member: member, Parts: []effect.PartChange{{
		Part: effect.Turns, Removed: []effect.Named{{Name: "peer", Role: "peer"}},
	}}}}
}

func workerChange(member string) []effect.Change {
	return []effect.Change{{Member: member, Parts: []effect.PartChange{{
		Part: effect.Workers, Removed: []effect.Named{{Name: "試作"}},
	}}}}
}

func rosterChange(member string) []effect.Change {
	return []effect.Change{{Member: member, Parts: []effect.PartChange{{Part: effect.Roster}}}}
}

func launchChange(member string) []effect.Change {
	return []effect.Change{{Member: member, Parts: []effect.PartChange{{Part: effect.Launch}}}}
}

func envChange(member string) []effect.Change {
	return []effect.Change{{Member: member, Parts: []effect.PartChange{{Part: effect.Env}}}}
}
