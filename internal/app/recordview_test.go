package app

import (
	"net/url"
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/homeview"
)

// recordPort is what the screens of the launch record see for the fixture's
// project.
func recordPort(t *testing.T) servePort {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	return servePort{env: env}
}

// docAt is the home's entry for one document, by its absolute path.
func docAt(t *testing.T, home homeview.Home, path string) homeview.Doc {
	t.Helper()
	env, err := team.NewEnv()
	if err != nil {
		t.Fatal(err)
	}
	want := env.Display(path)
	for _, doc := range home.Docs {
		if doc.Path == want {
			return doc
		}
	}
	t.Fatalf("the home has no entry for %s:\n%v", want, home.Docs)
	return homeview.Doc{}
}

// idsOf is a []homeview.Rule as the ids alone, for comparing.
func idsOf(rules []homeview.Rule) []string {
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rule.ID)
	}
	return out
}

// urlEscape is how the app's rule link carries a path.
func urlEscape(path string) string { return url.QueryEscape(path) }

func equalIDs(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestTheHomeReadsTheFilesAgainstTheLastLaunch: a document that moved since the
// launch shows the rules it added, dropped and changed, and its version against
// the one the launch read.
func TestTheHomeReadsTheFilesAgainstTheLastLaunch(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n"+
		"      - id: P1\n        text: one\n"+
		"      - id: P2\n        text: two\n")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	writeFile(t, protocol, "version: 1\nrevision: 2\nsections:\n  - rules:\n"+
		"      - id: P1\n        text: one changed\n"+
		"      - id: P3\n        text: three\n")

	home := recordPort(t).Home(web.LayerLocal)
	if !home.Launched {
		t.Fatal("the home does not see the record of the launch that ran")
	}
	if home.Problem != "" {
		t.Fatalf("the home read the record as broken: %s", home.Problem)
	}
	doc := docAt(t, home, protocol)
	if !equalIDs(doc.Added, "P3") || !equalIDs(doc.Removed, "P2") || !equalIDs(doc.Changed, "P1") {
		t.Errorf("added/removed/changed = %v/%v/%v, want [P3]/[P2]/[P1]",
			doc.Added, doc.Removed, doc.Changed)
	}
	if !equalIDs(idsOf(doc.NoCheck), "P1", "P3") {
		t.Errorf("the document lists %v with no check, want [P1 P3]", idsOf(doc.NoCheck))
	}
	if doc.Version != 2 || !doc.Bumped {
		t.Errorf("the document is at version %d (bumped %v), want 2 and bumped", doc.Version, doc.Bumped)
	}

	// The role files did not move, and every document shows once.
	role := docAt(t, home, filepath.Join(f.proj, ".hergy", "teams", "demo", "roles", "lead.yaml"))
	if role.Changes() {
		t.Errorf("a document nobody touched shows %v/%v/%v",
			role.Added, role.Removed, role.Changed)
	}
	if len(home.Docs) != 3 {
		t.Errorf("the home holds %d documents, want the protocol and the two role files", len(home.Docs))
	}
}

// TestTheHomeCarriesTheRulesWithNoCheckWithoutARecord: the rules with no check
// are read from the files, so they show on a machine that has not launched
// anything yet, and each one links into the rules screen.
func TestTheHomeCarriesTheRulesWithNoCheckWithoutARecord(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	writeFile(t, protocol, "version: 1\nreports:\n  audit:\n    where: the ledger\n    fields:\n      scope: the changed files\nsections:\n  - rules:\n"+
		"      - id: P1\n        text: one\n        report: audit.scope\n"+
		"      - id: P2\n        text: two\n")

	home := recordPort(t).Home(web.LayerLocal)
	if home.Launched {
		t.Error("the home sees a launch on a machine that has not launched anything")
	}
	doc := docAt(t, home, protocol)
	if !equalIDs(idsOf(doc.NoCheck), "P2") {
		t.Errorf("the document lists %v with no check, want [P2]", idsOf(doc.NoCheck))
	}
	if want := "/rules?doc=" + urlEscape(protocol) + "&layer=local&rule=P2"; doc.NoCheck[0].Href != want {
		t.Errorf("the rule links to %q, want %q", doc.NoCheck[0].Href, want)
	}
}

// TestTheDiffLinesUpThePromptsOfTheLastLaunch: a member whose documents moved
// shows the lines that changed, and a member whose documents did not stays the
// same.
func TestTheDiffLinesUpThePromptsOfTheLastLaunch(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	// Only the lead reads this file, so only the lead's prompt moves.
	role := filepath.Join(f.proj, ".hergy", "teams", "demo", "roles", "lead.yaml")
	writeFile(t, role, "version: 1\nsections:\n  - style: ids\n    rules:\n"+
		"      - id: L1\n        text: A rule the lead reads.\n        why: Because.\n        send_why: false\n")

	diff := recordPort(t).Diff(web.LayerLocal)
	if len(diff.Definitions) != 1 || diff.Definitions[0].Name != "demo" {
		t.Fatalf("the diff holds %v, want the demo definition", diff.Definitions)
	}
	def := diff.Definitions[0]
	if !def.Launched || len(def.Members) != 2 {
		t.Fatalf("the definition is launched %v with %d members, want both", def.Launched, len(def.Members))
	}
	lead, builder := def.Members[0], def.Members[1]
	if lead.Name != "demo-lead" || builder.Name != "demo-builder" {
		t.Fatalf("the members are %s and %s, want demo-lead and demo-builder", lead.Name, builder.Name)
	}
	if lead.Same() {
		t.Errorf("the lead's prompt did not move, its file changed:\n%v", lead.Lines)
	}
	var plus, minus int
	for _, line := range lead.Lines {
		switch line.Kind {
		case homeview.Plus:
			plus++
		case homeview.Minus:
			minus++
		}
	}
	if plus == 0 || minus == 0 {
		t.Errorf("the lead's diff has %d added and %d dropped lines, want both", plus, minus)
	}
	if !builder.Launched || !builder.Same() {
		t.Errorf("the builder's prompt moved although its files did not:\n%v", builder.Lines)
	}
}

// TestTheDiffWithoutARecord: a definition that was never launched has no prompt
// to compare with, and says so rather than showing an empty diff.
func TestTheDiffWithoutARecord(t *testing.T) {
	newFixture(t)
	mustRun(t, "new", "demo", "--local")

	diff := recordPort(t).Diff(web.LayerLocal)
	if len(diff.Definitions) != 1 {
		t.Fatalf("the diff holds %v, want the demo definition", diff.Definitions)
	}
	if diff.Definitions[0].Launched {
		t.Error("the diff finds a launch on a machine that has not launched anything")
	}
	for _, member := range diff.Definitions[0].Members {
		if member.Launched || len(member.Lines) != 0 {
			t.Errorf("%s shows a prompt that was never handed over: %v", member.Name, member.Lines)
		}
	}
}

// TestTheTeamListCarriesTheLastLaunch: the row shows when the definition was
// launched and the hash that launch wrote, and says whether the files build that
// same hash now.
func TestTheTeamListCarriesTheLastLaunch(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo", "--local")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	rows := recordPort(t).Teams(web.LayerLocal)
	if len(rows) != 1 {
		t.Fatalf("the team list holds %d rows, want one", len(rows))
	}
	row := rows[0]
	if row.Launched == "" || len(row.Hash) != 7 {
		t.Fatalf("the row carries launch %q and hash %q, want both", row.Launched, row.Hash)
	}
	if !row.Same {
		t.Error("the row says the files changed although the launch just ran")
	}

	// A rule the members read moves, so the files no longer build the hash the
	// launch wrote.
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "protocol.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n      - id: P1\n        text: moved\n")
	if again := recordPort(t).Teams(web.LayerLocal)[0]; again.Same {
		t.Error("the row says the files build the launched hash after a rule moved")
	}
}

// TestTheRuleLinkOpensTheRuleInTheRulesScreen: the home links into the rules
// screen with the document and the rule, and the path travels escaped.
func TestTheRuleLinkOpensTheRuleInTheRulesScreen(t *testing.T) {
	got := servePort{}.RuleHref(web.LayerLocal, "/tmp/.hergy/teams/demo/protocol.yaml", "C6")
	want := "/rules?doc=%2Ftmp%2F.hergy%2Fteams%2Fdemo%2Fprotocol.yaml&layer=local&rule=C6"
	if got != want {
		t.Errorf("the rule links to %q, want %q", got, want)
	}
}

// TestATimeIsWrittenForReading: the team list shows a recorded time the way a
// person reads it, and leaves one that does not read as it was written.
func TestATimeIsWrittenForReading(t *testing.T) {
	if got := timeOf("2026-10-15T14:03:00+09:00"); got != "2026-10-15 14:03" {
		t.Errorf("a recorded time reads %q, want 2026-10-15 14:03", got)
	}
	if got := timeOf("whenever"); got != "whenever" {
		t.Errorf("a time that does not read came back as %q, want it as written", got)
	}
}
