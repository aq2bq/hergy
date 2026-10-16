package app

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// sizedTeam is what writeSizedTeam wrote, so a test can build the lines validate
// must print from the files themselves.
type sizedTeam struct {
	dir         string // the definition directory, as a person writes it
	protocol    string
	role        string
	memberChars int
	sourceChars int
}

// writeSizedTeam writes one team definition whose one member reads a protocol of
// exactly n rules and an empty role file, so the size the member is handed is
// the protocol's.
func writeSizedTeam(t *testing.T, f *fixture, global bool, name string, n int) sizedTeam {
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
		body.WriteString("\n        text: rule ")
		body.WriteString(strconv.Itoa(i))
		body.WriteString("\n")
	}
	protocol := filepath.Join(base, "teams", name, "protocol.yaml")
	role := filepath.Join(base, "teams", name, "roles", "lead.yaml")
	writeFile(t, protocol, body.String())
	writeFile(t, filepath.Join(base, "teams", name, "team.yaml"),
		"version: 1\ndocs:\n  - protocol.yaml\nroles: [roles]\nmembers:\n  - name: lead\n    role: lead\n    kind: claude\n")
	writeFile(t, role, "version: 1\nsections:\n  - rules: []\n")
	document, err := rules.Parse(readRule(t, protocol), protocol)
	if err != nil {
		t.Fatal(err)
	}
	return sizedTeam{
		dir:         display,
		protocol:    protocol,
		role:        role,
		memberChars: rules.CharCount(document.Markdown()),
		sourceChars: rules.CharCount(readRule(t, protocol)) + rules.CharCount(readRule(t, role)),
	}
}

// TestValidateCountsTheSizeAndWarnsWithoutFailing: hergy validate prints each
// team definition with what it keeps and what each of its members is handed,
// warns over the provisional limits, and still exits 0, because a large team is
// a team to look at rather than a broken one (r-40a2, d-f2bc).
func TestValidateCountsTheSizeAndWarnsWithoutFailing(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	small := writeSizedTeam(t, f, false, "small", rules.MemberRuleLimit)   // exactly the limit
	large := writeSizedTeam(t, f, false, "large", rules.MemberRuleLimit+1) // one over

	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a team over the size limit is a break?\n%s", got)
	}

	// A definition comes first; its members and their warnings stand under it.
	wantLarge := strings.Join([]string{
		i18n.Tf("validate.size_team", large.dir, 2, large.sourceChars),
		i18n.Tf("validate.size_member", "large-lead", rules.MemberRuleLimit+1, large.memberChars),
		i18n.Tf("validate.size_warn_member_rules", "large-lead", rules.MemberRuleLimit+1, rules.MemberRuleLimit),
	}, "\n")
	if !strings.Contains(got, wantLarge+"\n") {
		t.Errorf("validate does not print the large definition with its member under it:\n%s", got)
	}
	if !strings.Contains(got, i18n.T("validate.size_note")+"\n") {
		t.Errorf("validate warns without saying what the limit is:\n%s", got)
	}

	// The definition exactly on the limit warns about nothing. The block is what
	// belongs to it: the next definition's line ends it.
	wantSmall := strings.Join([]string{
		i18n.Tf("validate.size_team", small.dir, 2, small.sourceChars),
		i18n.Tf("validate.size_member", "small-lead", rules.MemberRuleLimit, small.memberChars),
	}, "\n")
	if !strings.Contains(got, wantSmall+"\n") {
		t.Errorf("validate does not print the small definition with its member under it:\n%s", got)
	}
	if smallBlock := block(t, got, i18n.Tf("validate.size_team", small.dir, 2, small.sourceChars)); strings.Contains(smallBlock, "warning") {
		t.Errorf("a definition inside every limit is warned about:\n%s", smallBlock)
	}
}

// TestValidateTellsTwoDefinitionsOfTheSameNameApart: hergy allows one definition
// name in both layers, so the members stand under the definition they belong to
// and a warning says which file to look at (S1).
func TestValidateTellsTwoDefinitionsOfTheSameNameApart(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	global := writeSizedTeam(t, f, true, "dup", 3)
	local := writeSizedTeam(t, f, false, "dup", rules.MemberRuleLimit+1)

	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a team over the size limit is a break?\n%s", got)
	}
	// Both members carry the same name, and each stands under its own
	// definition exactly once.
	for _, want := range []string{
		i18n.Tf("validate.size_member", "dup-lead", 3, global.memberChars),
		i18n.Tf("validate.size_member", "dup-lead", rules.MemberRuleLimit+1, local.memberChars),
	} {
		if n := strings.Count(got, want); n != 1 {
			t.Errorf("validate prints %q %d times, want once:\n%s", want, n, got)
		}
	}

	localBlock := block(t, got, i18n.Tf("validate.size_team", local.dir, 2, local.sourceChars))
	if !strings.Contains(localBlock, i18n.Tf("validate.size_warn_member_rules", "dup-lead", rules.MemberRuleLimit+1, rules.MemberRuleLimit)) {
		t.Errorf("the local definition's warning is not in its block:\n%s", localBlock)
	}
	globalBlock := block(t, got, i18n.Tf("validate.size_team", global.dir, 2, global.sourceChars))
	if strings.Contains(globalBlock, "warning") {
		t.Errorf("the global definition, inside the limit, carries a warning:\n%s", globalBlock)
	}
	if !strings.Contains(globalBlock, i18n.Tf("validate.size_member", "dup-lead", 3, global.memberChars)) {
		t.Errorf("the global definition's member is not in its block:\n%s", globalBlock)
	}
}

// block is the part of the output that belongs to one team definition: its line
// and the member lines and warnings under it, up to the next definition's line.
// A definition line is indented with two spaces; a member line, with four.
func block(t *testing.T, output, header string) string {
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
		if len(lines[i]) > 2 && lines[i][0] == ' ' && lines[i][1] == ' ' && lines[i][2] != ' ' {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}
