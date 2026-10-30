package check

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// memberNode is a member as a turn's end, for the tests that build ops.
func memberNode(name string) definition.NodeID {
	return definition.NodeID{Kind: definition.KindMember, Name: name}
}

// humanNode is the human as a turn's end.
func humanNode() definition.NodeID {
	return definition.NodeID{Kind: definition.KindHuman, Name: "human"}
}

// teamInput is the fixture's demo definition, checked in the project's layer.
func teamInput(f *fixture, ops ...definition.Op) TeamInput {
	return TeamInput{Catalog: f.cat, Name: "demo", Layer: source.LayerLocal, Ops: ops}
}

// overlayOfFixture applies one op to the fixture's saved team.yaml and lays it
// over the catalog, the way Team does, so a test can ask check.Definition the
// same question hergy validate would.
func overlayOfFixture(t *testing.T, f *fixture, ops ...definition.Op) *catalog.Catalog {
	t.Helper()
	text, err := os.ReadFile(f.teamPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		out, err := definition.EditYAML(text, f.cat.Display(f.teamPath()), op)
		if err != nil {
			t.Fatalf("EditYAML: %v", err)
		}
		text = out
	}
	return f.cat.With(f.teamPath(), string(text))
}

// TestTeamMatchesTheDefinitionsFindings: with no ops, Team answers exactly what
// check.Definition answers, so a screen shows the sentences hergy validate prints
// (X1, V2), and the figure's numbers are the definition's own.
func TestTeamMatchesTheDefinitionsFindings(t *testing.T) {
	f := newFixture(t)
	f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nmembers:\n"+
		leadMember+specMember+
		"workers:\n  - from: lead\n    purpose: 実装\n    profile: claude\n"+
		"turns:\n  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.roles()

	got := Team(teamInput(f))
	want := Definition(Input{Catalog: f.cat, Name: "demo", Layer: source.LayerLocal})
	if !slices.Equal(got.Problems, want) {
		t.Errorf("problems = %+v, want check.Definition's %+v", got.Problems, want)
	}
	if got.Counts != (TeamCounts{Members: 2, Workers: 1, Turns: 3}) {
		t.Errorf("counts = %+v, want 2 members, 1 worker, 3 turns", got.Counts)
	}
	wantNodes := []TeamNode{
		{ID: "human", Kind: "human"},
		{ID: "lead", Kind: "member", Role: "lead"},
		{ID: "spec", Kind: "member", Role: "spec"},
	}
	if !slices.Equal(got.Graph.Nodes, wantNodes) {
		t.Errorf("nodes = %+v, want %+v", got.Graph.Nodes, wantNodes)
	}
	if len(got.Graph.Workers) != 1 || got.Graph.Workers[0].Purpose != "実装" {
		t.Errorf("workers = %+v, want the one worker of lead", got.Graph.Workers)
	}
}

// TestTeamLosesATurnAnOpTookAway: an op that takes a turn away moves the draft's
// turns and leaves the saved turns whole, and the findings are the ones hergy
// validate prints for the draft — the rule that still writes the turn warns
// (1.4.1, 1.6, spec 1.4.4 M2).
func TestTeamLosesATurnAnOpTookAway(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - lead -> spec\n  - spec -> human\n")
	f.role("lead", "      - id: PAAF\n        text: 実装は spec に渡す（lead → spec）。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	op := definition.RemoveTurn(memberNode("lead"), memberNode("spec"))
	got := Team(teamInput(f, op))

	want := Definition(Input{Catalog: overlayOfFixture(t, f, op), Name: "demo", Layer: source.LayerLocal})
	if !slices.Equal(got.Problems, want) {
		t.Errorf("problems = %+v, want check.Definition's %+v", got.Problems, want)
	}
	warn := i18n.Tf("validate.turn_not_in_turns", "PAAF", "lead → spec")
	found := false
	for _, p := range got.Problems {
		if p.Text == warn {
			found = true
		}
	}
	if !found {
		t.Errorf("problems = %+v, want the turn warning %q", got.Problems, warn)
	}

	if slices.Contains(got.Graph.Turns, [2]string{"lead", "spec"}) {
		t.Errorf("the draft still holds lead -> spec: %v", got.Graph.Turns)
	}
	if !slices.Contains(got.Graph.SavedTurns, [2]string{"lead", "spec"}) {
		t.Errorf("the saved turns lost lead -> spec: %v", got.Graph.SavedTurns)
	}
	if len(got.Graph.Turns) != 3 || len(got.Graph.SavedTurns) != 4 {
		t.Errorf("turns = %v, savedTurns = %v, want 3 and 4", got.Graph.Turns, got.Graph.SavedTurns)
	}
}

// TestTeamStopsOnATurnTheDraftCannotHold: an op that leaves a member with no
// outgoing turn makes the draft a graph hergy cannot hold, and the findings are
// validate's own error.graph_* sentences, each one stopping the definition.
func TestTeamStopsOnATurnTheDraftCannotHold(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.roles()

	op := definition.RemoveTurn(memberNode("lead"), humanNode())
	got := Team(teamInput(f, op))

	want := Definition(Input{Catalog: overlayOfFixture(t, f, op), Name: "demo", Layer: source.LayerLocal})
	if !slices.Equal(got.Problems, want) {
		t.Fatalf("problems = %+v, want check.Definition's %+v", got.Problems, want)
	}
	if len(got.Problems) == 0 {
		t.Fatal("problems = none, want the graph problem the draft made")
	}
	for _, p := range got.Problems {
		if !p.Stops || p.Kind != KindGraph {
			t.Errorf("problem = %+v, want a stopping graph problem", p)
		}
	}
}

// TestTeamStopsOnAnOpTheFileCannotHold: an op that names something the file does
// not hold is one finding that stops, with the op's own sentence, and the figure
// is still the one the saved file holds (the ops did not change it).
func TestTeamStopsOnAnOpTheFileCannotHold(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.roles()

	got := Team(teamInput(f, definition.RemoveTurn(memberNode("lead"), memberNode("spec"))))
	want := Finding{Stops: true, Text: i18n.Tf("error.edit_no_turn", "lead -> spec")}
	if len(got.Problems) != 1 || got.Problems[0] != want {
		t.Fatalf("problems = %+v, want the one refusal %+v", got.Problems, want)
	}
	if len(got.Graph.Nodes) != 3 || len(got.Graph.Turns) != 3 {
		t.Errorf("graph = %+v, want the saved figure still drawn", got.Graph)
	}
}

// TestALeftoverSharedAgreementIsOneBreakOnEveryDefinition: while a layer still
// holds the agreement every team used to share, every definition's check carries
// one break with the same sentence the commands stop with; removing the file
// takes the break away (d-31ad, T13 R1).
func TestALeftoverSharedAgreementIsOneBreakOnEveryDefinition(t *testing.T) {
	cat := teamsV2Catalog(t)
	path := filepath.Join(cat.Base(), ".hergy", "agreement.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version: 1\nsections:\n  - rules:\n      - id: WAAK\n        text: 全員が読む。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs := cat.SharedAgreements()
	if len(docs) != 1 {
		t.Fatalf("SharedAgreements() = %v, want the leftover one", docs)
	}
	want := SharedAgreementText(cat, docs)
	for _, name := range []string{"core", "docs", "migrate", "research"} {
		team := Team(TeamInput{Catalog: cat, Name: name, Layer: source.LayerLocal})
		found := 0
		for _, finding := range team.Problems {
			if finding.Stops && finding.Text == want {
				found++
			}
		}
		if found != 1 {
			t.Errorf("%s carries %d leftover breaks, want one", name, found)
		}
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	team := Team(TeamInput{Catalog: cat, Name: "core", Layer: source.LayerLocal})
	for _, finding := range team.Problems {
		if finding.Text == want {
			t.Error("the break stands after the file was removed")
		}
	}
}

// TestTeamSaysARoleNameThatIsNotOne: a member whose role is not a role name is
// the definition's own break (r-fc16). The document for that role is there, so
// nothing else about the definition stops: only the name does, and hergy cannot
// build the definition from it (catalog.RoleNameProblem). The sentence is the one
// the team's save and the roles screen use, and it says how to fix the name.
func TestTeamSaysARoleNameThatIsNotOne(t *testing.T) {
	f := newFixture(t)
	f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nmembers:\n"+
		leadMember+
		"  - name: spec-2\n    role: 進行管理\n    profile: claude\n"+
		"turns:\n  - human -> lead\n  - lead -> human\n  - spec-2 -> human\n")
	f.roles()
	f.role("進行管理", "      - id: SPEC-1\n        text: 何かをする。\n")

	report := Team(teamInput(f))
	var found *Finding
	for i := range report.Problems {
		if report.Problems[i].Field == "members[1].role" {
			found = &report.Problems[i]
		}
	}
	if found == nil {
		t.Fatalf("problems = %+v, want the member's role named", report.Problems)
	}
	if !found.Stops {
		t.Errorf("the finding about the role name does not stop: %+v", found)
	}
	if want := i18n.Tf("error.role_name_pattern", "進行管理"); !strings.Contains(found.Text, want) {
		t.Errorf("the finding is %q, want the sentence %q", found.Text, want)
	}
}
