package check

import (
	"errors"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
)

// ledgerDef is a team definition with the two members a ledger: test names, and
// the rules a test writes.
func ledgerDef(rules ...definition.Ledger) *definition.Definition {
	return &definition.Definition{
		Name: "hergy-dev",
		Graph: &definition.Graph{
			Members: []definition.MemberNode{{Name: "lead"}, {Name: "designer"}},
		},
		Ledger: rules,
	}
}

// noDecider is a decider that is never asked: a test whose writes are not
// questions uses it, so a question sneaking in fails loudly.
func noDecider(t *testing.T) Decider {
	t.Helper()
	return func(node string) (string, error) {
		t.Errorf("the check read a decider for %q, and the test wrote no question", node)
		return "", nil
	}
}

// TestLedgerFindingsWarnOnABrokenRule: a write a team member made that the rule
// gives to other members is one warning, naming the node, the write, the writer,
// the team and the members the rule allows.
func TestLedgerFindingsWarnOnABrokenRule(t *testing.T) {
	def := ledgerDef(definition.Ledger{Write: "req approve", Members: []string{"lead"}})
	writes := []LedgerWrite{{Actor: "hergy-dev-designer", Node: "r-1c42", Why: "req approve r-1c42"}}
	got, err := LedgerFindings(def, []string{"hergy-dev"}, writes, noDecider(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("LedgerFindings = %v, want one warning", got)
	}
	want := i18n.Tf("validate.ledger_breach", "r-1c42", "req approve", "hergy-dev-designer", "hergy-dev", "lead")
	if got[0].Text != want {
		t.Errorf("warning = %q, want %q", got[0].Text, want)
	}
	if got[0].Kind != KindLedger || got[0].Stops {
		t.Errorf("the warning is %v, stops=%v; want KindLedger and no stop", got[0].Kind, got[0].Stops)
	}
	if got[0].Field != "ledger[0]" {
		t.Errorf("Field = %q, want ledger[0]", got[0].Field)
	}

	// The member the rule allows, and another write the rule does not speak of,
	// are not warnings.
	allowed := []LedgerWrite{{Actor: "hergy-dev-lead", Node: "r-1", Why: "req approve r-1"}}
	if got, err := LedgerFindings(def, []string{"hergy-dev"}, allowed, noDecider(t)); err != nil || len(got) != 0 {
		t.Errorf("a write by the allowed member = %v, %v; want none", got, err)
	}
	other := []LedgerWrite{{Actor: "hergy-dev-designer", Node: "r-1", Why: "req done r-1"}}
	if got, err := LedgerFindings(def, []string{"hergy-dev"}, other, noDecider(t)); err != nil || len(got) != 0 {
		t.Errorf("a write no rule speaks of = %v, %v; want none", got, err)
	}
}

// TestLedgerFindingsIgnoreWritersOutsideTheTeam: the human, another team and an
// older way of writing a name are not this team's writer, so the rules are not
// about them (spec-ledger).
func TestLedgerFindingsIgnoreWritersOutsideTheTeam(t *testing.T) {
	def := ledgerDef(definition.Ledger{Write: "req approve", Members: []string{"lead"}})
	writes := []LedgerWrite{
		{Actor: "po", Node: "r-1", Why: "req approve r-1"},              // the human
		{Actor: "hergy-dev", Node: "r-2", Why: "req approve r-2"},       // the definition name alone
		{Actor: "other-lead", Node: "r-3", Why: "req approve r-3"},      // another team
		{Actor: "hergy-dev-ghost", Node: "r-4", Why: "req approve r-4"}, // a name that is no member
		{Actor: "", Node: "r-5", Why: "req approve r-5"},                // nobody
	}
	got, err := LedgerFindings(def, []string{"hergy-dev"}, writes, noDecider(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("LedgerFindings = %v, want none from writers outside the team", got)
	}
}

// TestLedgerFindingsSeeTheTeamNameItWasStartedAs: a team started with --as is
// the same definition, so a write by its members is judged, and the warning names
// the team it was started as (spec-ledger).
func TestLedgerFindingsSeeTheTeamNameItWasStartedAs(t *testing.T) {
	def := ledgerDef(definition.Ledger{Write: "req approve", Members: []string{"lead"}})
	writes := []LedgerWrite{{Actor: "hergy-dev-2-designer", Node: "r-1c42", Why: "req approve r-1c42"}}
	got, err := LedgerFindings(def, []string{"hergy-dev", "hergy-dev-2"}, writes, noDecider(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("LedgerFindings = %v, want one warning", got)
	}
	want := i18n.Tf("validate.ledger_breach", "r-1c42", "req approve", "hergy-dev-2-designer", "hergy-dev-2", "lead")
	if got[0].Text != want {
		t.Errorf("warning = %q, want %q", got[0].Text, want)
	}
}

// TestLedgerFindingsTellAQuestionApartByItsDecider: a question add is judged
// against question add when its decider is a member of the team, and against
// question to human when it is anybody else.
func TestLedgerFindingsTellAQuestionApartByItsDecider(t *testing.T) {
	def := ledgerDef(
		definition.Ledger{Write: definition.QuestionAdd, Members: []string{"lead"}},
		definition.Ledger{Write: definition.QuestionToHuman, Members: []string{"lead"}},
	)
	deciders := map[string]string{
		"q-member": "hergy-dev-lead",
		"q-human":  "po",
		"q-empty":  "",
	}
	decider := func(node string) (string, error) { return deciders[node], nil }
	writes := []LedgerWrite{
		{Actor: "hergy-dev-designer", Node: "q-member", Why: "question add q-member"},
		{Actor: "hergy-dev-designer", Node: "q-human", Why: "question add q-human"},
		{Actor: "hergy-dev-designer", Node: "q-empty", Why: "question add q-empty"},
	}
	got, err := LedgerFindings(def, []string{"hergy-dev"}, writes, decider)
	if err != nil {
		t.Fatal(err)
	}
	// The question to a member breaks question add; the two to nobody in the team
	// break question to human.
	if len(got) != 3 {
		t.Fatalf("LedgerFindings = %d warnings, want 3", len(got))
	}
	if !strings.Contains(got[0].Text, "q-member") || !strings.Contains(got[0].Text, definition.QuestionAdd) {
		t.Errorf("the member question was not judged as %s: %q", definition.QuestionAdd, got[0].Text)
	}
	for _, warning := range got[1:] {
		if !strings.Contains(warning.Text, definition.QuestionToHuman) {
			t.Errorf("the outside question was not judged as %s: %q", definition.QuestionToHuman, warning.Text)
		}
	}
}

// TestLedgerFindingsStopOnADeciderThatCannotBeRead: a decider gy cannot answer
// makes the whole check unread rather than a warning, so the caller reports a
// rule that was not checked (spec-ledger 3).
func TestLedgerFindingsStopOnADeciderThatCannotBeRead(t *testing.T) {
	def := ledgerDef(definition.Ledger{Write: definition.QuestionToHuman, Members: []string{"lead"}})
	decider := func(string) (string, error) { return "", errors.New("gy is not here") }
	writes := []LedgerWrite{{Actor: "hergy-dev-designer", Node: "q-1", Why: "question add q-1"}}
	got, err := LedgerFindings(def, []string{"hergy-dev"}, writes, decider)
	if err == nil {
		t.Fatalf("LedgerFindings = %v, nil; want the error of the decider that could not be read", got)
	}
}

// TestLedgerFindingsWithNoRulesReadNothing: a definition that writes no ledger:
// rule is not a definition with nothing broken; there is nothing to check.
func TestLedgerFindingsWithNoRulesReadNothing(t *testing.T) {
	def := ledgerDef()
	writes := []LedgerWrite{{Actor: "hergy-dev-designer", Node: "r-1", Why: "req approve r-1"}}
	got, err := LedgerFindings(def, []string{"hergy-dev"}, writes, noDecider(t))
	if err != nil || len(got) != 0 {
		t.Errorf("LedgerFindings = %v, %v; want none and no error", got, err)
	}
}
