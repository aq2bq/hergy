package check

import (
	"slices"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

// TestTeamEffectTakesATurnAway: one more op that takes a turn away moves one
// member's "ターンを渡す相手" line, written the way the prompt writes it
// (turns.member_item), and names the problem the change would add (1.4.4, X3).
func TestTeamEffectTakesATurnAway(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - lead -> spec\n  - spec -> human\n")
	f.role("lead", "      - id: PAAF\n        text: 実装は spec に渡す（lead → spec）。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	got := TeamEffect(teamInput(f), definition.RemoveTurn(memberNode("lead"), memberNode("spec")))
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %+v, want the one line of lead", got.Lines)
	}
	line := got.Lines[0]
	if line.Member != "demo-lead" || line.Part != "turns" {
		t.Errorf("line = %+v, want demo-lead's turns", line)
	}
	wantRemoved := []string{i18n.Tf("turns.member_item", "demo-spec", "spec")}
	if !slices.Equal(line.Removed, wantRemoved) {
		t.Errorf("removed = %q, want %q", line.Removed, wantRemoved)
	}
	if len(line.Added) != 0 {
		t.Errorf("added = %q, want nothing", line.Added)
	}

	warn := i18n.Tf("validate.turn_not_in_turns", "PAAF", "lead → spec")
	if len(got.Problems) != 1 || got.Problems[0].Text != warn {
		t.Errorf("problems = %+v, want the one turn warning %q", got.Problems, warn)
	}
}

// TestTeamEffectChangesAWorkersPurpose: one more op that changes a worker's
// purpose moves one member's "ワーカーの追加" line, written the way the prompt
// writes it (workers.allowed_item), with the old line leaving and the new one
// coming in.
func TestTeamEffectChangesAWorkersPurpose(t *testing.T) {
	f := newFixture(t)
	f.write(f.teamPath(), "version: 1\ndescription: a team for the test\nmembers:\n"+
		leadMember+specMember+
		"workers:\n  - from: lead\n    purpose: 実装\n    profile: claude\n"+
		"turns:\n  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.roles()

	got := TeamEffect(teamInput(f), definition.WorkerPurpose("lead", "実装", "実装する"))
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %+v, want the one line of lead", got.Lines)
	}
	line := got.Lines[0]
	if line.Member != "demo-lead" || line.Part != "workers" {
		t.Errorf("line = %+v, want demo-lead's workers", line)
	}
	if want := []string{i18n.Tf("workers.allowed_item", "実装", "claude")}; !slices.Equal(line.Removed, want) {
		t.Errorf("removed = %q, want %q", line.Removed, want)
	}
	if want := []string{i18n.Tf("workers.allowed_item", "実装する", "claude")}; !slices.Equal(line.Added, want) {
		t.Errorf("added = %q, want %q", line.Added, want)
	}
	if len(got.Problems) != 0 {
		t.Errorf("problems = %+v, want none", got.Problems)
	}
}

// TestTeamEffectShowsTheLineAStoppingChangeTakesAway: the last turn a member
// hands out is a change that stops the definition, so no prompt reads after it
// (spec-teams 1.9). The card still shows the line the change takes away — the
// parts a change moves are read from a graph that cannot hold — with the
// sentence the change adds beside it (1.4.4, X3, 監査 R11).
func TestTeamEffectShowsTheLineAStoppingChangeTakesAway(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - lead -> spec\n  - spec -> human\n")
	f.roles()

	got := TeamEffect(teamInput(f), definition.RemoveTurn(memberNode("spec"), humanNode()))
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %+v, want the one line of spec", got.Lines)
	}
	line := got.Lines[0]
	if line.Member != "demo-spec" || line.Part != "turns" {
		t.Errorf("line = %+v, want demo-spec's turns", line)
	}
	if want := []string{i18n.T("turns.human_item")}; !slices.Equal(line.Removed, want) {
		t.Errorf("removed = %q, want %q", line.Removed, want)
	}
	want := i18n.Tf("error.graph_member_no_turn", "spec", "spec", "spec", "lead")
	if len(got.Problems) != 1 || !got.Problems[0].Stops || !strings.Contains(got.Problems[0].Text, want) {
		t.Errorf("problems = %+v, want the one that stops: %s", got.Problems, want)
	}
}
