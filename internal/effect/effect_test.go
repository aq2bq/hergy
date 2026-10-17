// These are the tests of the effects of an edit (C4): which part of which
// member's prompt moves, the "保存すると" line, and the 渡し方 line per running
// team. The layer is a real one (a roles/ directory and a catalog over it), so a
// member's prompt is built through the same doors the launcher uses.
package effect_test

import (
	"os"
	"path/filepath"
	"reflect"
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
	edited, err := definition.EditYAML([]byte(teamText()), "team.yaml", definition.RemoveTurn(memberID("lead"), memberID("spec")))
	if err != nil {
		t.Fatal(err)
	}
	after := l.read(t, string(edited))

	changes := effect.Of(l.view(t, before), l.view(t, after))
	want := []effect.Change{{
		Member:     "lead",
		Definition: "demo",
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
	leadTurns := prompt.TurnsNote("", prompt.TurnsOf(after, "lead", "demo"))
	if strings.Contains(leadTurns, "spec") {
		t.Errorf("lead's turns still name spec: %q", leadTurns)
	}
}

// TestOfProfileChangeMovesTheLaunch: naming another profile moves how the member
// is started, and nothing else.
func TestOfProfileChangeMovesTheLaunch(t *testing.T) {
	l := newLayer(t)
	before := l.read(t, teamText())
	edited, err := definition.EditYAML([]byte(teamText()), "team.yaml", definition.MemberProfile("lead", "codex-default"))
	if err != nil {
		t.Fatal(err)
	}
	after := l.read(t, string(edited))

	changes := effect.Of(l.view(t, before), l.view(t, after))
	want := []effect.Change{{Member: "lead", Definition: "demo", Parts: []effect.PartChange{{Part: effect.Launch}}}}
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
	edited, err := definition.EditYAML([]byte(teamText()), "team.yaml", definition.MemberRole("spec", "lead"))
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

// TestOfARuleDocumentChangeMovesTheAgreements: a screen that writes one rule of
// a document writes the same path, so the change is read from the text both
// sides read, not from the paths (段 4, 設計 7.3)。Both members read the shared
// agreement, so both have an Agreements part.
func TestOfARuleDocumentChangeMovesTheAgreements(t *testing.T) {
	l := newLayer(t)
	write(t, filepath.Join(l.team, "team.yaml"), teamText())
	write(t, filepath.Join(l.root, "profiles.yaml"), "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	agreement := filepath.Join(l.root, "agreement.yaml")
	write(t, agreement, oneRule("A1", "前の文"))
	cat := l.catalog(t)
	before, err := cat.Read("demo")
	if err != nil {
		t.Fatal(err)
	}
	with := cat.With(agreement, oneRule("A1", "後の文"))
	after, err := with.Read("demo")
	if err != nil {
		t.Fatal(err)
	}

	changes := effect.Of(
		effect.View{Def: before, Profiles: l.profiles(), Run: prompt.Run{Team: "demo", Base: l.team, Reader: cat.Reader(), Sources: cat}},
		effect.View{Def: after, Profiles: l.profiles(), Run: prompt.Run{Team: "demo", Base: l.team, Reader: with.Reader(), Sources: with}},
	)
	if len(changes) != 2 {
		t.Fatalf("Of = %+v, want both members", changes)
	}
	for _, change := range changes {
		if !hasPart(change.Parts, effect.Agreements) {
			t.Errorf("%s's parts = %+v, want an agreements part", change.Member, change.Parts)
		}
	}
}

// TestOfTheSameValuesInAnotherShapeChangesNothing is spec/42 N1: a document
// written back with the same values reads the same, so no member's prompt moves
// and no team is told to take it up. The two texts differ only in the shape the
// file was written with (a quoted reason, a blank line).
func TestOfTheSameValuesInAnotherShapeChangesNothing(t *testing.T) {
	l := newLayer(t)
	write(t, filepath.Join(l.team, "team.yaml"), teamText())
	write(t, filepath.Join(l.root, "profiles.yaml"), "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	agreement := filepath.Join(l.root, "agreement.yaml")
	write(t, agreement, "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: 前の文\n        why: 'だから'\n")
	cat := l.catalog(t)
	before, err := cat.Read("demo")
	if err != nil {
		t.Fatal(err)
	}
	// The same document, in the shape Emit writes: the quoting and the blank
	// line differ, the values do not.
	with := cat.With(agreement, "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: 前の文\n        why: だから\n")
	after, err := with.Read("demo")
	if err != nil {
		t.Fatal(err)
	}

	changes := effect.Of(
		effect.View{Def: before, Profiles: l.profiles(), Run: prompt.Run{Team: "demo", Base: l.team, Reader: cat.Reader(), Sources: cat}},
		effect.View{Def: after, Profiles: l.profiles(), Run: prompt.Run{Team: "demo", Base: l.team, Reader: with.Reader(), Sources: with}},
	)
	if len(changes) != 0 {
		t.Fatalf("Of = %+v, want no change: the prompts read the same", changes)
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
			want: []string{"lead のプロンプトの『ターンを渡す相手』に人間が入ります"},
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
			changes: turnChange(),
			want:    "hergy reprompt lead --team demo",
		},
		{
			name:    "a worker moves",
			changes: workerChange(),
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
	got := effect.Handover(turnChange(), nil)
	want := "動いているチームはありません。次の hergy up から使われます"
	if len(got) != 1 || got[0] != want {
		t.Errorf("Handover = %q, want [%q]", got, want)
	}
}

// TestHandoverCommandsAreRealCommands: every command a handover holds is taken
// apart by the real command definitions (internal/options), so a command that
// drifts from hergy's own command line fails here (d-a8b1 追加の決め 5). The
// commands come from Commands, which is what a copy button is given: the note
// after a command is not part of it.
func TestHandoverCommandsAreRealCommands(t *testing.T) {
	running := []effect.Running{{Team: "demo", Definition: "demo"}}
	changesList := [][]effect.Change{
		turnChange(),
		workerChange(),
		rosterChange("spec"),
		launchChange("lead"),
		envChange("lead"),
		append(launchChange("lead"), rosterChange("spec")...),
	}
	for _, changes := range changesList {
		for _, team := range effect.Commands(changes, running) {
			if len(team.Commands) == 0 {
				t.Errorf("no command for %s", team.Team)
			}
			for _, command := range team.Commands {
				argv := strings.Fields(command.Line)[1:] // drop the word hergy
				if _, err := options.Parse(argv); err != nil {
					t.Errorf("the command %q does not parse: %v", command.Line, err)
				}
			}
		}
	}
}

// TestCommandsGiveTheCommandAndItsNoteApart: the note after a command is not
// part of the command, so a screen can copy the command alone (N4).
func TestCommandsGiveTheCommandAndItsNoteApart(t *testing.T) {
	got := effect.Commands(launchChange("lead"), []effect.Running{{Team: "demo", Definition: "demo"}})
	want := []effect.TeamCommands{{Team: "demo", Commands: []effect.Command{
		{Line: "hergy refresh lead --team demo", Note: "会話は消えます"},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Commands = %+v, want %+v", got, want)
	}
	got = effect.Commands(envChange("lead"), []effect.Running{{Team: "demo", Definition: "demo"}})
	want = []effect.TeamCommands{{Team: "demo", Commands: []effect.Command{
		{Line: "hergy down demo"},
		{Line: "hergy up demo --as demo"},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Commands = %+v, want %+v", got, want)
	}
}

// TestOfLeavesOutAPartTheBuiltPromptDoesNotShow is N1: a definition whose
// prompt: writes no %{turns} reads the same text after a turn leaves, so Of
// does not say lead's turns moved, "保存すると" says nothing moves, and 渡し方 says
// there is nothing to hand over.
func TestOfLeavesOutAPartTheBuiltPromptDoesNotShow(t *testing.T) {
	l := newLayer(t)
	text := strings.Replace(teamText(), "team: demo\n", "team: demo\nprompt: \"%{role_doc}%{docs}\"\n", 1)
	before := l.read(t, text)
	edited, err := definition.EditYAML([]byte(text), "team.yaml", definition.RemoveTurn(memberID("lead"), memberID("spec")))
	if err != nil {
		t.Fatal(err)
	}
	after := l.read(t, string(edited))

	changes := effect.Of(l.view(t, before), l.view(t, after))
	if len(changes) != 0 {
		t.Fatalf("Of = %+v, want no change (the prompt text does not move)", changes)
	}
	preview := effect.Preview(changes)
	if len(preview) != 1 || preview[0] != "どのメンバーのプロンプトも変わりません" {
		t.Errorf("Preview = %q, want the one line that says nothing moves", preview)
	}
	running := []effect.Running{{Team: "demo", Definition: "demo"}}
	handover := effect.Handover(changes, running)
	if len(handover) != 1 || handover[0] != "動いているチームに渡すものはありません" {
		t.Errorf("Handover = %q, want the one line that says nothing is handed over", handover)
	}
}

// TestHandoverUsesOnlyTheChangesOfTheTeamsOwnDefinition is N2: a change to
// another definition's team.yaml is not a team's to take up.
func TestHandoverUsesOnlyTheChangesOfTheTeamsOwnDefinition(t *testing.T) {
	changes := turnChange()
	running := []effect.Running{
		{Team: "demo", Definition: "demo"},
		{Team: "other", Definition: "zzz"},
	}
	want := []string{
		"hergy reprompt lead --team demo",
		"動いているチームに渡すものはありません",
	}
	if got := effect.Handover(changes, running); !reflect.DeepEqual(got, want) {
		t.Errorf("Handover = %q, want %q", got, want)
	}
}

// TestHandoverDoesNotAlsoRefreshWhenEnvMoves is N3: a down and up starts every
// member afresh, so nothing is put beside it.
func TestHandoverDoesNotAlsoRefreshWhenEnvMoves(t *testing.T) {
	changes := []effect.Change{{Member: "lead", Definition: "demo", Parts: []effect.PartChange{
		{Part: effect.Launch},
		{Part: effect.Env},
	}}}
	running := []effect.Running{{Team: "x2", Definition: "demo"}}
	got := effect.Handover(changes, running)
	want := []string{"hergy down x2 のあと、hergy up demo --as x2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Handover = %q, want %q", got, want)
	}
}

// TestHandoverRefreshesFirstAndDoesNotRepromptARefreshedMember is T3: a member
// that is refreshed gets its bootstrap prompt again, so it is not also
// reprompted, and the refresh comes first.
func TestHandoverRefreshesFirstAndDoesNotRepromptARefreshedMember(t *testing.T) {
	changes := []effect.Change{{Member: "lead", Definition: "demo", Parts: []effect.PartChange{
		{Part: effect.Turns, Removed: []effect.Named{{Name: "peer", Role: "peer"}}},
		{Part: effect.Launch},
	}}}
	running := []effect.Running{{Team: "demo", Definition: "demo"}}
	got := effect.Handover(changes, running)
	want := []string{"hergy refresh lead --team demo（会話は消えます）"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Handover = %q, want %q", got, want)
	}

	both := append(launchChange("spec"), rosterChange("lead")...)
	got = effect.Handover(both, running)
	want = []string{"hergy refresh spec --team demo（会話は消えます）、hergy reprompt lead --team demo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Handover = %q, want %q", got, want)
	}
}

// TestHandoverDoesNotRepromptMembersItRefreshed is R3: when everyone is
// refreshed there is no reprompt at all, and when some are, they are reprompted
// one by one rather than with --all (which would name the refreshed ones too).
func TestHandoverDoesNotRepromptMembersItRefreshed(t *testing.T) {
	running := []effect.Running{{Team: "demo", Definition: "demo"}}
	everyone := []effect.Change{
		{Member: "lead", Definition: "demo", Parts: []effect.PartChange{{Part: effect.Roster}, {Part: effect.Launch}}},
		{Member: "spec", Definition: "demo", Parts: []effect.PartChange{{Part: effect.Roster}, {Part: effect.Launch}}},
	}
	want := []string{"hergy refresh lead --team demo（会話は消えます）、hergy refresh spec --team demo（会話は消えます）"}
	if got := effect.Handover(everyone, running); !reflect.DeepEqual(got, want) {
		t.Errorf("Handover = %q, want %q", got, want)
	}
	some := []effect.Change{
		{Member: "lead", Definition: "demo", Parts: []effect.PartChange{{Part: effect.Roster}, {Part: effect.Launch}}},
		{Member: "spec", Definition: "demo", Parts: []effect.PartChange{{Part: effect.Roster}}},
	}
	want = []string{"hergy refresh lead --team demo（会話は消えます）、hergy reprompt spec --team demo"}
	if got := effect.Handover(some, running); !reflect.DeepEqual(got, want) {
		t.Errorf("Handover = %q, want %q", got, want)
	}
}

// TestPreviewSaysTheRosterMovedForEveryoneInOneLine is T1: when the roster
// moves it moves for every member, so one line says so.
func TestPreviewSaysTheRosterMovedForEveryoneInOneLine(t *testing.T) {
	changes := []effect.Change{
		{Member: "lead", Definition: "demo", Parts: []effect.PartChange{{Part: effect.Roster}}},
		{Member: "spec", Definition: "demo", Parts: []effect.PartChange{{Part: effect.RoleDocument}, {Part: effect.Roster}}},
	}
	want := []string{"全員のプロンプトの『名簿』が変わります", "spec のプロンプトの『ロールの文書』が変わります"}
	got := effect.Preview(changes)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Preview = %q, want %q", got, want)
	}
}

func turnChange() []effect.Change {
	return []effect.Change{{Member: "lead", Definition: "demo", Parts: []effect.PartChange{{
		Part: effect.Turns, Removed: []effect.Named{{Name: "peer", Role: "peer"}},
	}}}}
}

func workerChange() []effect.Change {
	return []effect.Change{{Member: "spec", Definition: "demo", Parts: []effect.PartChange{{
		Part: effect.Workers, Removed: []effect.Named{{Name: "試作"}},
	}}}}
}

func rosterChange(member string) []effect.Change {
	return []effect.Change{{Member: member, Definition: "demo", Parts: []effect.PartChange{{Part: effect.Roster}}}}
}

func launchChange(member string) []effect.Change {
	return []effect.Change{{Member: member, Definition: "demo", Parts: []effect.PartChange{{Part: effect.Launch}}}}
}

func envChange(member string) []effect.Change {
	return []effect.Change{{Member: member, Definition: "demo", Parts: []effect.PartChange{{Part: effect.Env}}}}
}

// launchedHashes is each member's prompt hash as one launch handed it, by the
// member's short name.
func launchedHashes(t *testing.T, l layer, def *definition.Definition) map[string]string {
	t.Helper()
	run := l.run(t)
	out := map[string]string{}
	for i := range def.Graph.Members {
		member := &def.Graph.Members[i]
		built, err := prompt.Member(def, member, run)
		if err != nil {
			t.Fatal(err)
		}
		out[member.Name] = source.Hash(built.Text)
	}
	return out
}

// TestUndeliveredNamesTheMembersWhosePromptMoved is C9: comparing the launch
// record's hashes with the prompt as it reads now names the members that moved,
// with no parts (the record holds no text) and a reprompt as the handover
// (設計 10.2, W9).
func TestUndeliveredNamesTheMembersWhosePromptMoved(t *testing.T) {
	l := newLayer(t)
	launched := l.read(t, teamText())
	hashes := launchedHashes(t, l, launched)

	// Nothing changed: a team that took the launch up has nothing to take up.
	if changes := effect.Undelivered(hashes, l.view(t, launched)); len(changes) != 0 {
		t.Errorf("Undelivered on an unchanged definition = %+v, want none", changes)
	}

	edited, err := definition.EditYAML([]byte(teamText()), "team.yaml", definition.MemberRole("spec", "lead"))
	if err != nil {
		t.Fatal(err)
	}
	now := l.read(t, string(edited))
	changes := effect.Undelivered(hashes, l.view(t, now))
	if len(changes) == 0 {
		t.Fatalf("a moved prompt was not named")
	}
	for _, change := range changes {
		if len(change.Parts) != 0 {
			t.Errorf("%s carries parts: %+v (the record holds only hashes)", change.Member, change.Parts)
		}
		if change.Definition != "demo" {
			t.Errorf("%s belongs to %q, want demo", change.Member, change.Definition)
		}
	}

	lines := effect.UndeliveredLines(changes)
	if len(lines) != len(changes) {
		t.Fatalf("UndeliveredLines = %q, want one line per member", lines)
	}
	for i, line := range lines {
		want := "起動のあとで変わったもの: " + changes[i].Member + " のプロンプト"
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}

	// The handover reprompts the members that moved: nothing of the prompt can
	// be named, but a reprompt is what makes them read it again.
	running := []effect.Running{{Team: "demo", Definition: "demo"}}
	commands := effect.Commands(changes, running)
	if len(commands) != 1 {
		t.Fatalf("Commands = %+v, want one team", commands)
	}
	for _, command := range commands[0].Commands {
		if !strings.HasPrefix(command.Line, "hergy reprompt ") {
			t.Errorf("the handover says %q, want a reprompt", command.Line)
		}
	}
}

// TestUndeliveredLeavesOutAMemberTheRecordDoesNotName: a member added since the
// launch is not this round's change (W7).
func TestUndeliveredLeavesOutAMemberTheRecordDoesNotName(t *testing.T) {
	l := newLayer(t)
	now := l.read(t, teamText())
	hashes := map[string]string{"nosuch": "deadbeef"}
	if changes := effect.Undelivered(hashes, l.view(t, now)); len(changes) != 0 {
		t.Errorf("Undelivered = %+v, want none for a member the record does not name", changes)
	}
}
