package app

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/team"
	"github.com/aq2bq/hergy/internal/web"
	"github.com/aq2bq/hergy/internal/web/homeview"
	"github.com/aq2bq/hergy/internal/web/rulesview"
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
	mustRun(t, "new", "demo")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
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
	if !equalIDs(idsOf(doc.Added), "P3") || !equalIDs(idsOf(doc.Removed), "P2") || !equalIDs(idsOf(doc.Changed), "P1") {
		t.Errorf("added/removed/changed = %v/%v/%v, want [P3]/[P2]/[P1]",
			doc.Added, doc.Removed, doc.Changed)
	}
	// The change rows carry what the screen draws: the parts that moved, and
	// the text before and after (試作 changeHtml).
	if got := doc.Changed[0]; !equalIDs(got.Fields, "text") || len(got.Diffs) != 1 ||
		got.Diffs[0].Before != "one" || got.Diffs[0].After != "one changed" {
		t.Errorf("the changed rule is %+v, want its text before and after", got)
	}
	if got := doc.Added[0]; len(got.Diffs) != 1 || got.Diffs[0].After != "three" || got.Diffs[0].Before != "" {
		t.Errorf("the added rule is %+v, want its text after only", got)
	}
	if got := doc.Removed[0]; len(got.Diffs) != 1 || got.Diffs[0].Before != "two" || got.Diffs[0].After != "" {
		t.Errorf("the removed rule is %+v, want its text before only", got)
	}
	// The document's heading names the launch the changes are against: when it
	// ran and the head of the hash it wrote.
	if doc.Launched == "" || len(doc.Hash) != 7 || doc.Definition != "demo" {
		t.Errorf("the document names launch %q, hash %q, definition %q, want both and demo",
			doc.Launched, doc.Hash, doc.Definition)
	}
	if !equalIDs(idsOf(doc.NoCheck), "P1", "P3") {
		t.Errorf("the document lists %v with no check, want [P1 P3]", idsOf(doc.NoCheck))
	}
	if doc.Version != 2 || !doc.Bumped {
		t.Errorf("the document is at version %d (bumped %v), want 2 and bumped", doc.Version, doc.Bumped)
	}

	// The role files did not move, and every document shows once.
	role := docAt(t, home, filepath.Join(f.proj, ".hergy", "roles", "lead.yaml"))
	if role.Changes() {
		t.Errorf("a document nobody touched shows %v/%v/%v",
			role.Added, role.Removed, role.Changed)
	}
	if len(home.Docs) != 3 {
		t.Errorf("the home holds %d documents, want the agreement and the two role files", len(home.Docs))
	}
}

// TestTheHomeCarriesTheRulesWithNoCheckWithoutARecord: the rules with no check
// are read from the files, so they show on a machine that has not launched
// anything yet, and each one links into the rules screen.
func TestTheHomeCarriesTheRulesWithNoCheckWithoutARecord(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
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
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	// Only the lead reads this file, so only the lead's prompt moves.
	role := filepath.Join(f.proj, ".hergy", "roles", "lead.yaml")
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
	mustRun(t, "new", "demo")

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

// TestTheDiffKeepsThePromptOfTheLaunchBeforeAReprompt: reprompt hands a prompt to
// one member, and the record of it holds that member alone. Every other member is
// still compared with the launch that last handed it a prompt, so its diff does
// not disappear (M2).
func TestTheDiffKeepsThePromptOfTheLaunchBeforeAReprompt(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")
	mustRun(t, "reprompt", "lead", "--team", "demo")

	diff := recordPort(t).Diff(web.LayerLocal)
	if len(diff.Definitions) != 1 || len(diff.Definitions[0].Members) != 2 {
		t.Fatalf("the diff holds %+v, want the demo definition with both members", diff.Definitions)
	}
	for _, member := range diff.Definitions[0].Members {
		if !member.Launched {
			t.Errorf("%s says no prompt was handed over, although up handed it one", member.Name)
		}
		if !member.Same() {
			t.Errorf("%s differs from the prompt up handed it:\n%v", member.Name, member.Lines)
		}
	}
}

// TestTheScreensCompareWithThisProjectsLaunch: the machine's record holds every
// project's launches, and two projects can hold a local team definition of the
// same name. The screens of one project compare with that project's launch: the
// team.yaml a launch read is what tells the definitions apart (M1).
func TestTheScreensCompareWithThisProjectsLaunch(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n      - id: P1\n        text: one\n")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	// Another project with a local definition of the same name, whose documents
	// differ, launched last: its record is the newest one in the log.
	other := filepath.Join(filepath.Dir(f.proj), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", other, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	dir := filepath.Join(other, ".hergy", "teams", "demo")
	writeFile(t, filepath.Join(dir, "team.yaml"), `version: 1
mission: a mission for the test
members:
  - name: lead
    role: lead
    profile: claude-default
turns:
  - human -> lead
  - lead -> human
`)
	writeFile(t, filepath.Join(other, ".hergy", "profiles.yaml"),
		"version: 1\nprofiles:\n  claude-default:\n    kind: claude\n")
	writeFile(t, filepath.Join(dir, "agreement.yaml"), "version: 1\nrevision: 4\nsections:\n  - rules:\n      - id: Q1\n        text: from the other project\n")
	writeFile(t, filepath.Join(other, ".hergy", "roles", "lead.yaml"), "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: the role of the other project\n")
	// The fake herdr remembers the agents it started; the other project starts
	// the same names.
	for _, name := range []string{"agents.txt", "tabs.txt"} {
		if err := os.WriteFile(filepath.Join(f.cfg, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(other)
	mustRun(t, "up", "demo")
	t.Chdir(f.proj)

	port := recordPort(t)
	rows := port.Teams(web.LayerLocal)
	if len(rows) != 1 {
		t.Fatalf("the team list holds %d rows, want the demo definition", len(rows))
	}
	if !rows[0].Same {
		t.Error("the team list compares this project's files with another project's launch")
	}

	diff := port.Diff(web.LayerLocal)
	if len(diff.Definitions) != 1 {
		t.Fatalf("the diff holds %+v, want the demo definition", diff.Definitions)
	}
	for _, member := range diff.Definitions[0].Members {
		if !member.Launched || !member.Same() {
			t.Errorf("%s is compared with another project's prompt:\n%v", member.Name, member.Lines)
		}
	}

	home := port.Home(web.LayerLocal)
	if !home.Launched {
		t.Error("the home does not see this project's launch")
	}
	for _, doc := range home.Docs {
		if doc.Changes() {
			t.Errorf("the home compares %s with another project's launch: %v/%v/%v",
				doc.Path, doc.Added, doc.Removed, doc.Changed)
		}
	}
}

// TestTheTeamListCarriesTheLastLaunch: the row shows when the definition was
// launched and the hash that launch wrote, and says whether the files build that
// same hash now.
func TestTheTeamListCarriesTheLastLaunch(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
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
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n      - id: P1\n        text: moved\n")
	if again := recordPort(t).Teams(web.LayerLocal)[0]; again.Same {
		t.Error("the row says the files build the launched hash after a rule moved")
	}
}

// changeIDs is a []rulesview.ChangeRule as the ids alone, for comparing.
func changeIDs(rules []rulesview.ChangeRule) []string {
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rule.ID)
	}
	return out
}

// TestTheRulesScreenComparesAndBumpsLikeTheHome: the rules screen's reads are
// the home screen's own comparison, mapped, so the two cannot disagree about a
// document; and a bump from the changes tab is the write the home screen's /bump
// makes, through the same checks.
func TestTheRulesScreenComparesAndBumpsLikeTheHome(t *testing.T) {
	f := newFixture(t)
	mustRun(t, "new", "demo")
	protocol := filepath.Join(f.proj, ".hergy", "teams", "demo", "agreement.yaml")
	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n"+
		"      - id: P1\n        text: one\n"+
		"      - id: P2\n        text: two\n")
	f.statefulHerdr(t)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p9")
	mustRun(t, "up", "demo")

	writeFile(t, protocol, "version: 1\nrevision: 1\nsections:\n  - rules:\n"+
		"      - id: P1\n        text: one changed\n"+
		"      - id: P3\n        text: three\n")

	port := rulesPort{recordPort(t)}
	homeDoc := docAt(t, port.Home(web.LayerLocal), protocol)
	changes := port.DocChanges(web.LayerLocal, protocol)
	if !changes.Recorded || changes.Version != homeDoc.Version || changes.Bumped != homeDoc.Bumped {
		t.Errorf("the rules screen reads %+v, the home reads version %d (bumped %v)",
			changes, homeDoc.Version, homeDoc.Bumped)
	}
	if !equalIDs(changeIDs(changes.Added), idsOf(homeDoc.Added)...) ||
		!equalIDs(changeIDs(changes.Removed), idsOf(homeDoc.Removed)...) ||
		!equalIDs(changeIDs(changes.Changed), idsOf(homeDoc.Changed)...) {
		t.Errorf("the rules screen moved %v/%v/%v, the home moved %v/%v/%v",
			changeIDs(changes.Added), changeIDs(changes.Removed), changeIDs(changes.Changed),
			idsOf(homeDoc.Added), idsOf(homeDoc.Removed), idsOf(homeDoc.Changed))
	}
	// The diff a changed rule carries is the home's own.
	if got := changes.Changed[0].Diffs; len(got) != 1 || got[0].Before != "one" || got[0].After != "one changed" {
		t.Errorf("the changed rule draws %+v, want the home's before and after", got)
	}
	// Every rule the change names opens in the rules screen.
	for _, rule := range changes.Changed {
		if !strings.Contains(rule.Href, "rule="+rule.ID) || !strings.Contains(rule.Href, urlEscape(protocol)) {
			t.Errorf("the rule opens at %q, want a link into the rules screen for %s", rule.Href, rule.ID)
		}
	}
	if !changes.CanBump() {
		t.Fatalf("the rules screen does not offer the bump: %+v", changes)
	}

	// The bump is the same write the home screen's /bump makes.
	answer, err := port.Bump(web.LayerLocal, protocol)
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if !answer.Written || answer.From != 1 || answer.To != 2 {
		t.Fatalf("the bump answered %+v, want version 1 → 2", answer)
	}
	after := port.DocChanges(web.LayerLocal, protocol)
	if after.Version != 2 || !after.Bumped || after.CanBump() {
		t.Errorf("after the bump the document reads %+v, want version 2 and already raised", after)
	}
	// A second bump writes nothing and says why.
	again, err := port.Bump(web.LayerLocal, protocol)
	if err != nil {
		t.Fatalf("Bump again: %v", err)
	}
	if again.Written || again.Why == "" {
		t.Errorf("a second bump answered %+v, want nothing written and a reason", again)
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

// TestTheChangeRowNamesWhatMoved: a changed rule names the parts that moved and
// draws each one, the origin among them, with what it read before and after
// (試作 pending と changeHtml)。
func TestTheChangeRowNamesWhatMoved(t *testing.T) {
	before := ruleLine{id: "C1", section: "基本", rule: rules.Rule{
		Text:   "one",
		Why:    "because",
		Origin: []rules.Origin{{Kind: rules.KindIncident, Date: "2026-10-01"}},
	}}
	after := ruleLine{id: "C1", section: "基本", rule: rules.Rule{
		Text:   "two",
		Why:    "because",
		Origin: []rules.Origin{{Kind: rules.KindDerived, Refs: []string{"C2"}}},
	}}

	fields := changedFields(before, after)
	if !equalIDs(fields, "text", "origin") {
		t.Fatalf("the change names %v, want the text and the origin", fields)
	}
	diffs := changeDiffs(before, after, fields)
	if len(diffs) != 2 {
		t.Fatalf("the change draws %d diffs, want the text and the origin", len(diffs))
	}
	if diffs[0].Before != "one" || diffs[0].After != "two" {
		t.Errorf("the text reads %+v, want one → two", diffs[0])
	}
	incident := i18n.T("web.rules.origin.kind.incident") + " 2026-10-01"
	derived := i18n.T("web.rules.origin.kind.derived") + " ← C2"
	if diffs[1].Before != incident || diffs[1].After != derived {
		t.Errorf("the origin reads %+v, want %q → %q", diffs[1], incident, derived)
	}
	if got := originText(nil); got != i18n.T("web.home.changes.origin_none") {
		t.Errorf("a rule with no origin reads %q, want it said so", got)
	}
}
