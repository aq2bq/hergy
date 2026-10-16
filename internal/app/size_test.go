package app

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// writeSizedTeam writes one local team definition whose one member reads a
// protocol of exactly n rules and an empty role file, so the size the member is
// handed is the protocol's.
func writeSizedTeam(t *testing.T, f *fixture, name string, n int) {
	t.Helper()
	var body strings.Builder
	body.WriteString("version: 1\nsections:\n  - rules:\n")
	for i := range n {
		body.WriteString("      - id: R")
		body.WriteString(strconv.Itoa(i))
		body.WriteString("\n        text: rule ")
		body.WriteString(strconv.Itoa(i))
		body.WriteString("\n")
	}
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "protocol.yaml"), body.String())
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "team.yaml"),
		"version: 1\ndocs:\n  - protocol.yaml\nroles: [roles]\nmembers:\n  - name: lead\n    role: lead\n    kind: claude\n")
	writeFile(t, filepath.Join(f.proj, ".hergy", "teams", name, "roles", "lead.yaml"), "version: 1\nsections:\n  - rules: []\n")
}

// TestValidateCountsTheSizeAndWarnsWithoutFailing: hergy validate prints what
// each member is handed and what each team keeps, warns over the provisional
// limits and still exits 0, because a large team is a team to look at rather
// than a broken one (r-40a2, d-f2bc).
func TestValidateCountsTheSizeAndWarnsWithoutFailing(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	writeSizedTeam(t, f, "small", rules.MemberRuleLimit)   // exactly the limit
	writeSizedTeam(t, f, "large", rules.MemberRuleLimit+1) // one over

	got, _, code := run("validate")
	if code != 0 {
		t.Fatalf("a team over the size limit is a break?\n%s", got)
	}

	// The numbers are the ones of the Markdown hergy renders, rule by rule and
	// character by character.
	protocol := filepath.Join(f.proj, ".hergy", "teams", "large", "protocol.yaml")
	role := filepath.Join(f.proj, ".hergy", "teams", "large", "roles", "lead.yaml")
	document, err := rules.Parse(readRule(t, protocol), protocol)
	if err != nil {
		t.Fatal(err)
	}
	memberChars := rules.CharCount(document.Markdown())
	sourceChars := rules.CharCount(readRule(t, protocol)) + rules.CharCount(readRule(t, role))

	for _, want := range []string{
		i18n.Tf("validate.size_member", "small-lead", rules.MemberRuleLimit, memberCharsFor(t, f, "small")),
		i18n.Tf("validate.size_member", "large-lead", rules.MemberRuleLimit+1, memberChars),
		i18n.Tf("validate.size_team", filepath.Join(".hergy", "teams", "large"), 2, sourceChars),
		i18n.Tf("validate.size_warn_member_rules", "large-lead", rules.MemberRuleLimit+1, rules.MemberRuleLimit),
		i18n.T("validate.size_note"),
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("validate is missing %q:\n%s", want, got)
		}
	}
	// The team exactly on the limit warns about nothing.
	if strings.Contains(got, i18n.Tf("validate.size_warn_member_rules", "small-lead", rules.MemberRuleLimit, rules.MemberRuleLimit)) {
		t.Errorf("a member handed exactly the limit is warned about:\n%s", got)
	}
	if strings.Contains(got, "small") && strings.Contains(got, i18n.Tf("validate.size_warn_member_chars", "small-lead", 0, 0)) {
		t.Errorf("a member well inside the character limit is warned about:\n%s", got)
	}
}

// memberCharsFor is what one sized team's member is handed: the protocol's
// render (the role file holds no rule, so it renders to nothing).
func memberCharsFor(t *testing.T, f *fixture, name string) int {
	t.Helper()
	path := filepath.Join(f.proj, ".hergy", "teams", name, "protocol.yaml")
	document, err := rules.Parse(readRule(t, path), path)
	if err != nil {
		t.Fatal(err)
	}
	return rules.CharCount(document.Markdown())
}
