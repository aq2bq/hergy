package definition_test

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
)

// ledgerTeam is a team.yaml body with two members (a and b), a worker hired by a,
// and the shortest turn graph, then the ledger: lines a test writes. The worker's
// name is a/build, which is what a ledger: line must not name.
func ledgerTeam(ledger string) string {
	return "version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}\"\nmembers:\n" +
		"  - name: a\n    role: r\n    profile: claude-default\n" +
		"  - name: b\n    role: r\n    profile: claude-default\n" +
		"workers:\n  - from: a\n    purpose: build\n    profile: claude-default\n" +
		"turns:\n  - human -> a\n  - a -> b\n  - b -> a\n  - a -> human\n" +
		ledger
}

// TestLedgerReadsRules: ledger: lines read into rules in the written order, one
// rule per line, each a write and the members it names. A definition that writes
// no ledger: carries none (the key is optional).
func TestLedgerReadsRules(t *testing.T) {
	l := newLayer(t, "")
	got, err := parseTeam(t, l, ledgerTeam("ledger:\n  - req approve by a\n  - question to human by a, b\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []definition.Ledger{
		{Write: "req approve", Members: []string{"a"}},
		{Write: definition.QuestionToHuman, Members: []string{"a", "b"}},
	}
	if len(got.def.Ledger) != len(want) {
		t.Fatalf("Ledger = %v, want %v", got.def.Ledger, want)
	}
	for i := range want {
		if got.def.Ledger[i].Write != want[i].Write || strings.Join(got.def.Ledger[i].Members, ",") != strings.Join(want[i].Members, ",") {
			t.Errorf("Ledger[%d] = %v, want %v", i, got.def.Ledger[i], want[i])
		}
	}

	none, err := parseTeam(t, newLayer(t, ""), ledgerTeam(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(none.def.Ledger) != 0 {
		t.Errorf("a definition with no ledger: carries %v, want none", none.def.Ledger)
	}
}

// TestLedgerBrokenLineStops: a line hergy cannot read, a write it does not know,
// a name that is no member and a worker's name each stop the definition, the way
// a broken turns: line does. The sentence names what is wrong.
func TestLedgerBrokenLineStops(t *testing.T) {
	cases := []struct{ name, line, want string }{
		{"no by", "req approve a", "req approve a"},
		{"an unknown write", "req approvex by a", "req approvex"},
		{"a name that is no member", "req approve by c", "c"},
		{"a worker's name", "req approve by a/build", "a/build"},
		{"the purpose alone", "req approve by build", "build"},
		{"no member at all", "req approve by ", "req approve"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTeam(t, newLayer(t, ""), ledgerTeam("ledger:\n  - "+tc.line+"\n"))
			if err == nil {
				t.Fatal("a broken ledger: line did not stop the parse")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %q", err, tc.want)
			}
		})
	}
}

// TestLedgerBrokenLineNamesTheLine: a broken ledger: line is reported by the line
// a person counts, the way turns: is (段 7).
func TestLedgerBrokenLineNamesTheLine(t *testing.T) {
	_, err := parseTeam(t, newLayer(t, ""),
		ledgerTeam("ledger:\n  - req approve by a\n  - req approve c\n"))
	if err == nil {
		t.Fatal("a broken ledger: line did not stop the parse")
	}
	want := i18n.Tf("label.ledger_line", 2)
	if !strings.Contains(err.Error(), want) {
		t.Errorf("the error %q does not name %q", err, want)
	}
}

// TestLedgerReadsTheUnknownProfile: a profile that is not there does not stop
// the parse, with a ledger: line beside it or without one (spec-teams 1.9): the
// check reports the name, and the ledger is read.
func TestLedgerReadsTheUnknownProfile(t *testing.T) {
	l := newLayer(t, "")
	profiles := map[string]profile.Profile{"claude-default": {Name: "claude-default", Kind: "claude"}}
	got, err := parse("version: 1\nmission: a mission for the test\nteam: t\nprompt: \"%{role_doc}%{docs}\"\nmembers:\n  - name: a\n    role: r\n    profile: absent\n"+
		"turns:\n  - human -> a\n  - a -> human\nledger:\n  - req approve by a\n", "t", "", l, profiles)
	if err != nil {
		t.Fatalf("a profile that does not exist stopped the parse: %v", err)
	}
	if got := got.def.Graph.Members[0].Profile; got != "absent" {
		t.Errorf("the member's profile reads %q, want the name as it is written", got)
	}
}
