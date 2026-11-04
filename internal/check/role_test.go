package check

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
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
		Sev: "warn", Section: -1, Rule: -1, Line: 0,
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

// TestRoleAnswerDoesNotChangeWithTheTeamsThatUseIt: the same role document is
// checked once with a team that plays it and once with no team at all; the
// check's problems and its size are the same (d-8bbf). Only Readers, which is
// information apart from the check, changes.
func TestRoleAnswerDoesNotChangeWithTheTeamsThatUseIt(t *testing.T) {
	withTeam := newFixture(t)
	withTeam.team("  - human -> lead\n  - lead -> human\n  - spec -> human\n")
	lead := withTeam.roleText("lead", "      - id: PAAF\n        text: 実装は impl に渡す（lead → impl）。\n")
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

	if len(a.Readers) != 1 || len(b.Readers) != 0 {
		t.Errorf("readers with a team = %+v, alone = %+v", a.Readers, b.Readers)
	}
}
