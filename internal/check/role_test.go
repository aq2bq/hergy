package check

import (
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
		Sev: rules.SevWarning, Section: 0, Rule: 0, Field: "text", Line: 7,
		Text: i18n.Tf("validate.rule_model_name", "LEAD-1", "claude"+i18n.T("list.separator")+"opus"),
	}
	if len(report.Problems) != 1 || report.Problems[0] != want {
		t.Errorf("problems = %+v, want the one model warning %+v", report.Problems, want)
	}
}

// TestRoleDoesNotJudgeATeamTurn: a rule whose arrow names a team's turn is the
// team definition's problem, not the role document's (d-a575, d-8bbf). The role
// check stays clean with no team's answer in it; the team check names the role
// document, its rule and the role, so a screen can open it.
func TestRoleDoesNotJudgeATeamTurn(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	lead := f.roleText("lead", "      - id: LEAD-1\n        text: 実装は impl に渡す（lead → impl）。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	report := reportFor(f, RoleInput{Name: "lead", Text: lead})
	if len(report.Problems) != 0 {
		t.Errorf("the role document got a team's problem: %+v", report.Problems)
	}
	if len(report.Turns) != 0 {
		t.Errorf("turns = %+v, want none in a role document's check", report.Turns)
	}

	team := Team(TeamInput{Catalog: f.cat, Name: "demo", Layer: source.LayerLocal})
	var found *Finding
	for i := range team.Problems {
		if team.Problems[i].RuleID == "LEAD-1" && strings.Contains(team.Problems[i].Text, "impl") {
			found = &team.Problems[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("the team check does not name the role's turn: %+v", team.Problems)
	}
	if found.Role != "lead" || found.Doc != f.leadRolePath() {
		t.Errorf("finding = %+v, want the role and its document", found)
	}
	if found.RoleDoc != f.leadRolePath() {
		t.Errorf("role doc = %q, want %q", found.RoleDoc, f.leadRolePath())
	}
	want := i18n.Tf("validate.turn_unknown_name", "LEAD-1", "lead → impl", "impl")
	if found.Text != want {
		t.Errorf("text = %q, want %q", found.Text, want)
	}
}

// TestRoleWarnsAboutItsOwnSize: a role document over the provisional budget
// warns about the document itself, not about a member's amount (d-00a9,
// d-b87a). Readers says who uses it, by name only.
func TestRoleWarnsAboutItsOwnSize(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	var rulesText strings.Builder
	for i := range 30 {
		rulesText.WriteString("      - id: LEAD-" + string(rune('A'+i%26)) + strings.Repeat("A", i/26) + "\n")
		rulesText.WriteString("        text: " + strings.Repeat("あ", 120) + "\n")
	}
	lead := f.roleText("lead", rulesText.String())
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	report := reportFor(f, RoleInput{Name: "lead", Text: lead})
	if report.Markdown.Chars <= RoleCharLimit {
		t.Fatalf("chars = %d, want over the limit %d", report.Markdown.Chars, RoleCharLimit)
	}
	want := RoleProblem{
		Sev: rules.SevWarning, Section: -1, Rule: -1, Line: 1,
		Text: i18n.Tf("size.role_chars", report.Markdown.Chars, RoleCharLimit),
	}
	if len(report.Problems) != 1 || report.Problems[0] != want {
		t.Errorf("problems = %+v, want the one size warning %+v", report.Problems, want)
	}
	if report.Limit != RoleCharLimit {
		t.Errorf("limit = %d, want %d", report.Limit, RoleCharLimit)
	}
	if len(report.Readers) != 1 {
		t.Fatalf("readers = %+v, want the one member that reads it", report.Readers)
	}
	reader := report.Readers[0]
	if reader.Definition != "demo" || reader.Member != "demo-lead" {
		t.Errorf("reader = %+v, want demo/demo-lead", reader)
	}
}

// TestRoleReportsTheMarkdownAndTheDiffFromTheSavedDocument: a screen draws the
// Markdown the draft stands for, and marks the lines it added and removed
// against the document on disk (C1, C6).
func TestRoleReportsTheMarkdownAndTheDiffFromTheSavedDocument(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	saved := "version: 1\ntitle: lead\nsections:\n  - heading: 仕事\n    rules:\n" +
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

// TestRoleLeavesADuplicateWithTheAgreementToTheTeam: an id the role document and
// the team's agreement both hold is a combination problem, so it belongs to the
// team definition's check, which names the role and the rule (d-a575, d-8bbf).
// The role's own check is clean; the check writes nothing to disk.
func TestRoleLeavesADuplicateWithTheAgreementToTheTeam(t *testing.T) {
	f := newFixture(t)
	f.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	f.role("lead", "      - id: LEAD-1\n        text: 何かをする。\n")
	f.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")
	agreement := f.proj + "/.hergy/teams/demo/agreement.yaml"
	f.write(agreement, "version: 1\ntitle: agreement\nsections:\n  - heading: 約束\n    rules:\n"+
		"      - id: DUP-1\n        text: 何かをする。\n")

	draft := &rules.Document{
		Version: 1, Title: "lead",
		Sections: []rules.Section{{
			Heading: "仕事",
			Rules: []rules.Rule{
				{ID: "LEAD-1", Text: "何かをする。"},
				{ID: "DUP-1", Text: "書きかけの規則。"},
			},
		}},
	}
	report := reportFor(f, RoleInput{Name: "lead", Draft: draft})
	if len(report.Problems) != 0 {
		t.Errorf("the role document got the agreement's problem: %+v", report.Problems)
	}

	text, err := rules.Emit(draft)
	if err != nil {
		t.Fatal(err)
	}
	cat := f.cat.With(f.leadRolePath(), text)
	team := Team(TeamInput{Catalog: cat, Name: "demo", Layer: source.LayerLocal})
	var found *Finding
	for i := range team.Problems {
		if team.Problems[i].RuleID == "DUP-1" {
			found = &team.Problems[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("the team check does not name the repeated id: %+v", team.Problems)
	}
	if found.Role != "lead" || !strings.HasPrefix(found.Field, "rule:") || found.At != "id" {
		t.Errorf("finding = %+v, want the role's id", found)
	}
	if !strings.Contains(found.Text, "DUP-1") {
		t.Errorf("text = %q, want the repeated id", found.Text)
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

// TestRoleAnswerDoesNotChangeWithTheTeamsThatUseIt: the same role document is
// checked once with a team that plays it and once with no team at all; the
// check's problems and its size are the same (d-8bbf). Only Readers, which is
// information apart from the check, changes.
func TestRoleAnswerDoesNotChangeWithTheTeamsThatUseIt(t *testing.T) {
	withTeam := newFixture(t)
	withTeam.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	lead := withTeam.roleText("lead", "      - id: LEAD-1\n        text: 実装は impl に渡す（lead → impl）。\n")
	withTeam.role("spec", "      - id: SPEC-1\n        text: 何かをする。\n")

	alone := newFixture(t)
	alone.write(alone.leadRolePath(), lead)

	a := reportFor(withTeam, RoleInput{Name: "lead", Text: lead})
	b := reportFor(alone, RoleInput{Name: "lead", Text: lead})
	if len(a.Problems) != len(b.Problems) {
		t.Fatalf("problems with a team = %+v, alone = %+v", a.Problems, b.Problems)
	}
	for i := range a.Problems {
		if a.Problems[i] != b.Problems[i] {
			t.Errorf("problem %d with a team = %+v, alone = %+v", i, a.Problems[i], b.Problems[i])
		}
	}
	if a.Markdown.Chars != b.Markdown.Chars || a.Limit != b.Limit {
		t.Errorf("size with a team = %d/%d, alone = %d/%d", a.Markdown.Chars, a.Limit, b.Markdown.Chars, b.Limit)
	}
	if len(a.Turns) != 0 || len(b.Turns) != 0 {
		t.Errorf("turns with a team = %+v, alone = %+v", a.Turns, b.Turns)
	}
	if len(a.Readers) != 1 || len(b.Readers) != 0 {
		t.Errorf("readers with a team = %+v, alone = %+v", a.Readers, b.Readers)
	}
}
