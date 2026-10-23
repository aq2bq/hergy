package check

import (
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/source"
)

// agreementPath is where the fixture's definition keeps its agreement.
func (f *fixture) agreementPath() string {
	return filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
}

// agreementDoc is one agreement with a rule whose text writes the arrow the
// test is about.
const agreementDoc = "version: 1\ntitle: demo agreement\nsections:\n  - heading: 約束\n    rules:\n" +
	"      - id: AGR-1\n        text: 実装は spec に渡す（lead → spec）。\n"

// TestAgreementJudgesAnArrowAgainstTheDraftTurns: the agreement's rule writes
// lead → spec, and the definition's turns do not hold it, so the arrow comes
// back not held and one warning stands beside the rule (C6, 1.5).
func TestAgreementJudgesAnArrowAgainstTheDraftTurns(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	f.write(f.agreementPath(), agreementDoc)

	got := Agreement(AgreementInput{
		Catalog: f.cat, Name: "demo", Place: source.LayerLocal, Text: agreementDoc,
	})

	if len(got.Turns) != 1 {
		t.Fatalf("turns = %+v, want the one arrow the agreement writes", got.Turns)
	}
	turn := got.Turns[0]
	if turn.Arrow != "lead → spec" || turn.OK {
		t.Errorf("turn = %+v, want lead → spec not held", turn)
	}
	if turn.Definition != "demo" {
		t.Errorf("turn definition = %q, want demo", turn.Definition)
	}

	want := i18n.Tf("validate.turn_not_in_turns", "AGR-1", "lead → spec")
	if len(got.Problems) != 1 || got.Problems[0].Text != want {
		t.Errorf("problems = %+v, want the one warning %q", got.Problems, want)
	}

	if len(got.Markdown.Lines) == 0 || got.Markdown.Chars == 0 {
		t.Errorf("markdown = %+v, want the draft's preview", got.Markdown)
	}
}

// TestAgreementHoldsAnArrowTheTurnsHave: an arrow the definition's turns hold
// comes back held, and adds no warning.
func TestAgreementHoldsAnArrowTheTurnsHave(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - lead -> spec\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	f.write(f.agreementPath(), agreementDoc)

	got := Agreement(AgreementInput{
		Catalog: f.cat, Name: "demo", Place: source.LayerLocal, Text: agreementDoc,
	})
	if len(got.Turns) != 1 || !got.Turns[0].OK {
		t.Fatalf("turns = %+v, want the one arrow held", got.Turns)
	}
	if len(got.Problems) != 0 {
		t.Errorf("problems = %+v, want none", got.Problems)
	}
}

// TestAgreementJudgesAgainstTheOverlaidDraftTurns: the caller overlays the
// ops-applied team.yaml, so an arrow the draft turns hold is held even though
// the saved file does not write it (C6, 1.5).
func TestAgreementJudgesAgainstTheOverlaidDraftTurns(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	f.write(f.agreementPath(), agreementDoc)

	overlay := overlayOfFixture(t, f, definition.AddTurn(memberNode("lead"), memberNode("spec")))
	got := Agreement(AgreementInput{
		Catalog: overlay, Name: "demo", Place: source.LayerLocal, Text: agreementDoc,
	})
	if len(got.Turns) != 1 || !got.Turns[0].OK {
		t.Fatalf("turns = %+v, want the arrow held by the draft's turns", got.Turns)
	}
	if len(got.Problems) != 0 {
		t.Errorf("problems = %+v, want none", got.Problems)
	}
}
