package app

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// sizedTeam is what writeSizedTeam wrote, so a test can build the lines validate
// must print from the files themselves.
type sizedTeam struct {
	dir       string // the definition directory, as a person writes it
	agreement string
	role      string
	teamChars int
	roleChars int
}

// writeSizedTeam writes one team definition whose own agreement holds n rules of
// the text it is given, and a layer's role document with one harmless rule. The
// size of a team definition is the prompt part of its own rules (d-00a9): its
// agreement's Markdown, with no template written. The size of a role document is
// the part of it that goes into the prompt.
func writeSizedTeam(t *testing.T, f *fixture, global bool, name string, n int, text string) sizedTeam {
	t.Helper()
	base := filepath.Join(f.proj, ".hergy")
	display := filepath.Join(".hergy", "teams", name)
	if global {
		base = filepath.Join(f.cfg, "hergy")
		display = filepath.Join(base, "teams", name)
	}
	var body strings.Builder
	body.WriteString("version: 1\nsections:\n  - rules:\n")
	for i := range n {
		body.WriteString("      - id: R")
		body.WriteString(strconv.Itoa(i))
		body.WriteString("\n        text: ")
		body.WriteString(text)
		body.WriteString("\n")
	}
	agreement := filepath.Join(base, "teams", name, "agreement.yaml")
	role := filepath.Join(base, "roles", "lead.yaml")
	roleText := "version: 1\nsections:\n  - heading: 仕事\n    rules:\n      - id: PAAF\n        text: 何かをする。\n"
	writeFile(t, agreement, body.String())
	writeFile(t, filepath.Join(base, "profiles.yaml"), "version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	writeFile(t, filepath.Join(base, "teams", name, "team.yaml"),
		"version: 1\ndescription: a description for the test\nmembers:\n  - name: lead\n    role: lead\n    profile: claude-default\nturns:\n  - human -> lead\n  - lead -> human\n")
	writeFile(t, role, roleText)
	teamDoc, err := rules.Parse(readRule(t, agreement), agreement)
	if err != nil {
		t.Fatal(err)
	}
	roleDoc, err := rules.Parse(roleText, role)
	if err != nil {
		t.Fatal(err)
	}
	return sizedTeam{
		dir:       display,
		agreement: agreement,
		role:      role,
		teamChars: rules.CharCount(teamDoc.Markdown()),
		roleChars: rules.CharCount(roleDoc.Markdown()),
	}
}

// TestValidateCountsTheSizeAndWarnsWithoutFailing: hergy validate prints each
// team definition's own size and each role document's own size, with the
// provisional budget beside them, and warns over the budget without failing
// (d-00a9, d-b87a). A per-member total is nowhere.
func TestValidateCountsTheSizeAndWarnsWithoutFailing(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	small := writeSizedTeam(t, f, false, "small", 2, "short")
	large := writeSizedTeam(t, f, false, "large", 30, strings.Repeat("あ", 120))

	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a team over the size limit is a break?\n%s", got)
	}
	for _, want := range []string{
		i18n.Tf("validate.size_team", small.dir, small.teamChars, check.TeamCharLimit),
		i18n.Tf("validate.size_team", large.dir, large.teamChars, check.TeamCharLimit),
		i18n.Tf("validate.size_role", ".hergy/roles/lead.yaml", large.roleChars, check.RoleCharLimit),
		i18n.T("validate.size_note"),
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("validate does not print %q:\n%s", want, got)
		}
	}
	warning := i18n.Tf("size.team_chars", large.teamChars, check.TeamCharLimit)
	if n := strings.Count(got, warning); n != 1 {
		t.Errorf("the large team's size warning stands %d times, want once:\n%s", n, got)
	}
	if small.teamChars > check.TeamCharLimit {
		t.Errorf("the small team is %d characters, want under the budget %d", small.teamChars, check.TeamCharLimit)
	}
	if large.teamChars <= check.TeamCharLimit {
		t.Fatalf("the large team is %d characters, want over the budget %d", large.teamChars, check.TeamCharLimit)
	}
	if strings.Contains(got, i18n.T("validate.size_member")) || strings.Contains(got, "member small-lead") {
		t.Errorf("validate still prints a member's total:\n%s", got)
	}
}

// TestValidateCountsOnlyThePromptPart: a rule kept out of the prompt
// (in_prompt: false) is not part of the team definition's size either: the
// number is the Markdown a member reads, not the file's bytes (d-00a9). No
// per-member count line is printed (d-b87a).
func TestValidateCountsOnlyThePromptPart(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	team := writeSizedTeam(t, f, false, "small", 2, "rule")
	writeFile(t, team.agreement, "version: 1\nsections:\n  - rules:\n"+
		"      - id: R0\n        text: rule 0\n        in_prompt: false\n"+
		"      - id: R1\n        text: rule 1\n")
	document, err := rules.Parse(readRule(t, team.agreement), team.agreement)
	if err != nil {
		t.Fatal(err)
	}
	chars := rules.CharCount(document.Markdown())

	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a team with a rule kept out of the prompt is a break?\n%s", got)
	}
	if want := i18n.Tf("validate.size_team", team.dir, chars, check.TeamCharLimit); !strings.Contains(got, want+"\n") {
		t.Errorf("validate does not print %q:\n%s", want, got)
	}
	if strings.Contains(got, "member ") {
		t.Errorf("validate prints a member's total:\n%s", got)
	}
}

// TestValidateTellsTwoDefinitionsOfTheSameNameApart: hergy allows one definition
// name in both layers, so each definition's size line names the file it belongs
// to, and a warning stands under the definition it is about (S1).
func TestValidateTellsTwoDefinitionsOfTheSameNameApart(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	global := writeSizedTeam(t, f, true, "dup", 3, "short")
	local := writeSizedTeam(t, f, false, "dup", 30, strings.Repeat("あ", 120))

	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a team over the size limit is a break?\n%s", got)
	}
	if !strings.Contains(got, i18n.T("validate.size_heading")+"\n") {
		t.Errorf("validate does not head the size list:\n%s", got)
	}
	if want := i18n.Tf("validate.size_team", local.dir, local.teamChars, check.TeamCharLimit); !strings.Contains(got, want+"\n") {
		t.Errorf("validate does not print the effective definition's size %q:\n%s", want, got)
	}
	if hidden := i18n.Tf("validate.size_team", global.dir, global.teamChars, check.TeamCharLimit); strings.Contains(got, hidden) {
		t.Errorf("validate prints the hidden definition's size %q:\n%s", hidden, got)
	}
	warning := i18n.Tf("size.team_chars", local.teamChars, check.TeamCharLimit)
	if n := strings.Count(got, warning); n != 1 {
		t.Errorf("the local definition's warning stands %d times, want once:\n%s", n, got)
	}
	if !strings.Contains(blockOf(t, got, "file: .hergy/teams/dup/team.yaml"), warning) {
		t.Errorf("the local definition's block does not hold its warning:\n%s", got)
	}
	if strings.Contains(blockOf(t, got, "file: "+filepath.Join(global.dir, "team.yaml")), warning) {
		t.Errorf("the global definition's block holds the local warning:\n%s", got)
	}
}

// TestValidateSizesADefinitionThatDoesNotRead: a team definition whose check
// stops (a worker names a profile that is not there) still has its size in the
// list: the file reads, so the size can be measured (T13 R1). The size is the
// effective definition's, the one the screen's list shows.
func TestValidateSizesADefinitionThatDoesNotRead(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	team := writeSizedTeam(t, f, false, "broken", 2, "short")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", "broken", "team.yaml"),
		"version: 1\ndescription: a description for the test\nmembers:\n  - name: lead\n    role: lead\n    profile: claude-default\nworkers:\n  - from: lead\n    purpose: 調査\n    profile: absent\nturns:\n  - human -> lead\n  - lead -> human\n")

	got, _, code := run("validate")
	if code == 0 {
		t.Fatalf("a worker with a missing profile is not a break:\n%s", got)
	}
	if want := i18n.Tf("validate.size_team", team.dir, team.teamChars, check.TeamCharLimit); !strings.Contains(got, want+"\n") {
		t.Errorf("validate does not print the broken definition's size %q:\n%s", want, got)
	}
}

// blockOf is the part of the output that belongs to one file: its line and the
// details and warnings under it, up to the next file's line.
func blockOf(t *testing.T, output, header string) string {
	t.Helper()
	lines := strings.Split(output, "\n")
	start := -1
	for i, line := range lines {
		if line == header {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("the output has no line %q:\n%s", header, output)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "file: ") || strings.HasPrefix(lines[i], "checked: ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// TestValidateHidesWarningsOfAnUnusedDefinition: a definition the project's
// own hides gets only the line that says it is not used, and what stops it when
// it is broken; its warnings are not printed, because a person cannot fix them
// there (T13 R3).
func TestValidateHidesWarningsOfAnUnusedDefinition(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	global := writeSizedTeam(t, f, true, "dup", 2, "short")
	writeSizedTeam(t, f, false, "dup", 2, "short")

	// The hidden definition has a warning (a rule whose arrow no turn holds) and
	// a stop (a worker whose profile is not there).
	writeFile(t, global.agreement, "version: 1\nsections:\n  - rules:\n"+
		"      - id: G-1\n        text: a → b\n")
	writeFile(t, filepath.Join(f.cfg, "hergy", "teams", "dup", "team.yaml"),
		"version: 1\ndescription: a description for the test\nmembers:\n  - name: lead\n    role: lead\n    profile: claude-default\nworkers:\n  - from: lead\n    purpose: 調査\n    profile: absent\nturns:\n  - human -> lead\n  - lead -> human\n")

	got, _, code := run("validate")
	if code == 0 {
		t.Fatalf("a hidden definition with a missing worker profile is not a break:\n%s", got)
	}
	hidden := blockOf(t, got, "file: "+filepath.Join(global.dir, "team.yaml"))
	if !strings.Contains(hidden, i18n.Tf("validate.global_hidden", "dup")) {
		t.Errorf("the hidden definition does not say it is not used:\n%s", hidden)
	}
	if !strings.Contains(hidden, "workers[0]") {
		t.Errorf("the hidden definition does not show what stops it:\n%s", hidden)
	}
	if strings.Contains(hidden, "G-1") {
		t.Errorf("the hidden definition shows a warning that cannot be fixed there:\n%s", hidden)
	}
}

// TestValidateSaysWhenAGlobalRoleIsNotUsed: a global role document the project's
// own hides gets the not-used line, and its warnings are not printed (T13 R4).
func TestValidateSaysWhenAGlobalRoleIsNotUsed(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	project := filepath.Join(f.proj, ".hergy", "roles", "scout.yaml")
	writeFile(t, project, "version: 1\ntitle: scout\nsections:\n  - rules:\n      - id: S-1\n        text: 何かをする。\n")
	global := filepath.Join(f.cfg, "hergy", "roles", "scout.yaml")
	writeFile(t, global, "version: 1\ntitle: scout\nsections:\n  - rules:\n      - id: S-1\n        text: claude に任せる。\n")

	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a hidden role document is a break?\n%s", got)
	}
	hidden := blockOf(t, got, "file: "+global)
	if !strings.Contains(hidden, i18n.Tf("validate.role_hidden", "scout")) {
		t.Errorf("the hidden role does not say it is not used:\n%s", hidden)
	}
	if strings.Contains(hidden, "claude") {
		t.Errorf("the hidden role shows a warning that cannot be fixed there:\n%s", hidden)
	}
	projectBlock := blockOf(t, got, "file: .hergy/roles/scout.yaml")
	if strings.Contains(projectBlock, i18n.Tf("validate.role_hidden", "scout")) {
		t.Errorf("the effective role says it is not used:\n%s", projectBlock)
	}
}
