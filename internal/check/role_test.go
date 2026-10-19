package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// reportFor checks the fixture's lead role document. A screen hands Role the
// draft it holds; the tests hand it the text the fixture wrote, which is what a
// screen sends for a document it has not changed.
func reportFor(f *fixture, in RoleInput) RoleReport {
	in.Catalog = f.cat
	if in.Place == "" {
		in.Place = source.LayerLocal
	}
	return Role(in)
}

// TestRoleWarnsAboutAModelNameInARuleText: a role document is meant to read the
// same whatever model runs it, so a rule whose text names the engine or a model
// family warns, once, beside that rule (C8, 1.6).
func TestRoleWarnsAboutAModelNameInARuleText(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	lead := f.roleText("lead", "      - id: LEAD-1\n        text: 設計は claude の opus に任せる。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	report := reportFor(f, RoleInput{Name: "lead", Text: lead})
	want := RoleProblem{
		Sev: rules.SevWarning, Section: 0, Rule: 0,
		Field: "rule:" + f.leadRolePath() + "#LEAD-1",
		Text:  i18n.Tf("validate.rule_model_name", "LEAD-1", "claude"+i18n.T("list.separator")+"opus"),
	}
	if len(report.Problems) != 1 || report.Problems[0] != want {
		t.Errorf("problems = %+v, want the one model warning %+v", report.Problems, want)
	}
}

// TestRoleNamesTheDefinitionAtTheHeadOfATurnWarning: the sentence hergy validate
// writes is prefixed with the definition a screen is showing it for, so a person
// reads the same sentence in both places (V4, 1.6).
func TestRoleNamesTheDefinitionAtTheHeadOfATurnWarning(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	lead := f.roleText("lead", "      - id: LEAD-1\n        text: 実装は impl に渡す（lead → impl）。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	report := reportFor(f, RoleInput{Name: "lead", Text: lead})
	want := RoleProblem{
		Sev: rules.SevWarning, Section: 0, Rule: 0,
		Field: "rule:" + f.leadRolePath() + "#LEAD-1", Definition: "demo",
		Text: "demo: " + i18n.Tf("validate.turn_unknown_name", "LEAD-1", "lead → impl", "impl"),
	}
	if len(report.Problems) != 1 || report.Problems[0] != want {
		t.Errorf("problems = %+v, want the one turn warning %+v", report.Problems, want)
	}
}

// TestRoleNamesTheDefinitionAtTheHeadOfASizeWarning: a member handed more than
// the provisional limit warns, with the definition the sentence is about at its
// head and the member's herdr name in it (C7, 1.6).
func TestRoleNamesTheDefinitionAtTheHeadOfASizeWarning(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	var rulesText strings.Builder
	for i := 0; i <= rules.MemberRuleLimit; i++ {
		fmt.Fprintf(&rulesText, "      - id: LEAD-%d\n        text: 何かをする。\n", i)
	}
	lead := f.roleText("lead", rulesText.String())
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	report := reportFor(f, RoleInput{Name: "lead", Text: lead})
	want := RoleProblem{
		Sev: rules.SevWarning, Section: -1, Rule: -1, Definition: "demo",
		Text: "demo: " + MemberRulesWarning("demo-lead", rules.MemberRuleLimit+1),
	}
	if len(report.Problems) != 1 || report.Problems[0] != want {
		t.Errorf("problems = %+v, want the one size warning %+v", report.Problems, want)
	}
	if len(report.Readers) != 1 {
		t.Fatalf("readers = %+v, want the one member that reads it", report.Readers)
	}
	reader := report.Readers[0]
	if reader.Definition != "demo" || reader.Member != "demo-lead" || !reader.Over {
		t.Errorf("reader = %+v, want demo/demo-lead over the limit", reader)
	}
	if reader.Rules != rules.MemberRuleLimit+1 {
		t.Errorf("rules = %d, want %d", reader.Rules, rules.MemberRuleLimit+1)
	}
}

// TestRoleReportsTheMarkdownAndTheDiffFromTheSavedDocument: a screen draws the
// Markdown the draft stands for, and marks the lines it added and removed
// against the document on disk (C1, C6).
func TestRoleReportsTheMarkdownAndTheDiffFromTheSavedDocument(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	saved := "version: 1\ntitle: lead\nsections:\n  - heading: 仕事\n    style: ids\n    rules:\n" +
		"      - id: LEAD-1\n        text: 何かをする。\n"
	f.write(f.leadRolePath(), saved)

	same := reportFor(f, RoleInput{Name: "lead", Text: saved})
	if len(same.Problems) != 0 {
		t.Errorf("the document as it is on disk has problems: %+v", same.Problems)
	}
	if same.Diff.Added != nil || same.Diff.RemovedBefore != nil || same.Diff.RemovedAtEnd {
		t.Errorf("the document as it is on disk has a diff: %+v", same.Diff)
	}

	draft := saved + "      - id: LEAD-2\n        text: 足す。\n"
	report := reportFor(f, RoleInput{Name: "lead", Text: draft})
	parsed, err := rules.Parse(draft, f.leadRolePath())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Markdown.Lines) != len(parsed.Lines()) {
		t.Fatalf("lines = %d, want %d", len(report.Markdown.Lines), len(parsed.Lines()))
	}
	if report.Markdown.Chars != rules.CharCount(parsed.Markdown()) {
		t.Errorf("chars = %d, want %d", report.Markdown.Chars, rules.CharCount(parsed.Markdown()))
	}
	lines := report.Markdown.Lines
	if lines[0].Part != rules.PartTitle || lines[0].Text != "# lead" {
		t.Errorf("the first line = %+v, want the title", lines[0])
	}
	if last := lines[len(lines)-1]; last.Rule != 1 || last.Text != "- **LEAD-2** 足す。" {
		t.Errorf("the last line = %+v, want the rule the draft added", last)
	}
	want := len(lines) - 1
	if len(report.Diff.Added) != 1 || report.Diff.Added[0] != want {
		t.Errorf("added = %v, want the one line at the end (%d)", report.Diff.Added, want)
	}
	if report.Diff.RemovedBefore != nil || report.Diff.RemovedAtEnd {
		t.Errorf("the diff says a line was removed: %+v", report.Diff)
	}
}

// TestRoleReadsTheRoleTabsDraftAsIfItWereSaved: the draft a screen holds is
// checked as if it were written, so a rule id it takes from the team's agreement
// is refused before the save (C5, C6). The problem stands on the draft's rule,
// which is where the screen draws it, and the file is left as it was.
func TestRoleReadsTheRoleTabsDraftAsIfItWereSaved(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	agreement := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	f.write(agreement, "version: 1\ntitle: agreement\nsections:\n  - heading: 約束\n    style: ids\n    rules:\n"+
		"      - id: DUP-1\n        text: 何かをする。\n")

	draft := &rules.Document{
		Version: 1, Title: "lead",
		Sections: []rules.Section{{
			Heading: "仕事", Style: rules.StyleIDs,
			Rules: []rules.Rule{
				{ID: "LEAD-1", Text: "何かをする。"},
				{ID: "DUP-1", Text: "書きかけの規則。"},
			},
		}},
	}
	report := reportFor(f, RoleInput{Name: "lead", Draft: draft})
	want := RoleProblem{
		Sev: rules.SevError, Section: 0, Rule: 1, Field: "id", Definition: "demo",
		Text: i18n.Tf("error.rules_duplicate_id", agreement, "DUP-1", f.leadRolePath()),
	}
	if len(report.Problems) != 1 || report.Problems[0] != want {
		t.Errorf("problems = %+v, want the one repeated id %+v", report.Problems, want)
	}
	onDisk, err := os.ReadFile(f.leadRolePath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "DUP-1") || !strings.Contains(string(onDisk), "LEAD-1") {
		t.Errorf("the check wrote the draft to disk:\n%s", onDisk)
	}
}

// TestRoleReadsAnEmptiedYAMLFile: the YAML tab's text is read as the draft when
// the role tab holds none, and an emptied file is the problem a person deleting
// the whole file gets (C2, C6).
func TestRoleReadsAnEmptiedYAMLFile(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	lead := f.roleText("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	report := reportFor(f, RoleInput{Name: "lead", Text: lead})
	if len(report.Problems) != 0 {
		t.Fatalf("the saved document has problems: %+v", report.Problems)
	}

	empty := reportFor(f, RoleInput{Name: "lead", Text: ""})
	if len(empty.Problems) != 1 {
		t.Fatalf("problems = %+v, want the emptied file's one", empty.Problems)
	}
	if want := i18n.Tf("error.file_empty", "lead.yaml", f.leadRolePath()); empty.Problems[0].Text != want {
		t.Errorf("problem = %q, want %q", empty.Problems[0].Text, want)
	}
	if empty.Problems[0].Sev != rules.SevError {
		t.Errorf("severity = %q, want an error", empty.Problems[0].Sev)
	}
}

// TestRoleJudgesEveryArrowOfTheDocumentForEachReader: a screen shows, for every
// arrow the rule texts write, whether each definition that reads the document
// holds it in turns:, whether it holds or not (C6, 1.6).
func TestRoleJudgesEveryArrowOfTheDocumentForEachReader(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	lead := f.roleText("lead", strings.Join([]string{
		"      - id: LEAD-1\n        text: 設計は spec に渡す（lead → spec）。\n",
		"      - id: LEAD-2\n        text: 報告は人間に渡す（lead → human）。\n",
	}, ""))
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	report := reportFor(f, RoleInput{Name: "lead", Text: lead})
	got := map[string]RoleTurn{}
	for _, turn := range report.Turns {
		got[turn.Arrow] = turn
	}
	for arrow, ok := range map[string]bool{"lead → spec": false, "lead → human": true} {
		turn, there := got[arrow]
		if !there {
			t.Errorf("no turn for %q: %+v", arrow, report.Turns)
			continue
		}
		if turn.Definition != "demo" || turn.OK != ok {
			t.Errorf("turn %q = %+v, want demo/%v", arrow, turn, ok)
		}
	}
	if lead := got["lead → spec"]; lead.Section != 0 || lead.Rule != 0 {
		t.Errorf("the first rule's turn = %+v, want (0, 0)", lead)
	}
	if human := got["lead → human"]; human.Section != 0 || human.Rule != 1 {
		t.Errorf("the second rule's turn = %+v, want (0, 1)", human)
	}
}
