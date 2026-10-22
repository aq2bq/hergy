package rules

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// builderRole is the shape the roles fixture writes: two sections with heading
// and a rule list, and the rules of one section.
const builderRole = `version: 1
title: 設計実装
revision: 5
sections:
  - heading: 作り方
    rules:
      - id: BUILD-1
        text: 最初の本文。
      - id: BUILD-2
        text: 2 番目の本文。
      - id: BUILD-3
        text: 3 番目の本文。
  - heading: 確かめ方
    rules:
      - id: BUILD-4
        text: 4 番目の本文。
      - id: BUILD-5
        text: 5 番目の本文。
`

// fromByID is the place each draft rule came from, matched by id: the way a
// screen holds its draft. A rule id the saved document does not hold is new.
func fromByID(t *testing.T, saved, want *Document) []Place {
	t.Helper()
	place := map[string]Place{}
	for si, sec := range saved.Sections {
		for ri, rule := range sec.Rules {
			place[rule.ID] = Place{Section: si, Rule: ri}
		}
	}
	var from []Place
	for _, rule := range want.Rules() {
		if p, ok := place[rule.ID]; ok {
			from = append(from, p)
			continue
		}
		from = append(from, Place{Section: -1, Rule: -1})
	}
	return from
}

// mustParse reads a document or fails.
func mustParse(t *testing.T, text, path string) *Document {
	t.Helper()
	doc, err := Parse(text, path)
	if err != nil {
		t.Fatalf("Parse(%s): %v", path, err)
	}
	return doc
}

// TestRewriteChangesTitleAndRevision: the two top values are written on their own
// lines, and every other line stays.
func TestRewriteChangesTitleAndRevision(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Title = "実装"
	*want.Revision = 6

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "title: 実装\n") || !strings.Contains(got, "revision: 6\n") {
		t.Errorf("the write does not hold the new values:\n%s", got)
	}
	if strings.Contains(got, "title: 設計実装") || strings.Contains(got, "revision: 5") {
		t.Errorf("the old values stayed:\n%s", got)
	}
	// The document still holds the same rule values.
	back := mustParse(t, got, "roles/builder.yaml")
	if len(back.Rules()) != len(saved.Rules()) {
		t.Errorf("rules changed: %d, want %d", len(back.Rules()), len(saved.Rules()))
	}
}

// TestRewriteChangesASectionHeading: only the heading line changes (F3).
func TestRewriteChangesASectionHeading(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Sections[0].Heading = "進め方"

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	wantText := strings.Replace(builderRole, "heading: 作り方", "heading: 進め方", 1)
	if string(out) != wantText {
		t.Errorf("Rewrite wrote\n%q\nwant\n%q", string(out), wantText)
	}
}

// TestRewriteTakesOutALeftoverStyle: a file that still writes a section's style:
// loses the line on the first save, even when the draft changes nothing else
// (d-f4f6).
func TestRewriteTakesOutALeftoverStyle(t *testing.T) {
	const text = "version: 1\nsections:\n  - heading: 使命\n    style: ids\n    rules:\n      - id: R1\n        text: 本文。\n"
	saved := mustParse(t, text, "roles/lead.yaml")
	want := mustParse(t, text, "roles/lead.yaml")

	out, err := Rewrite([]byte(text), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "style") {
		t.Errorf("the style: line stayed:\n%s", out)
	}
	if wantText := "version: 1\nsections:\n  - heading: 使命\n    rules:\n      - id: R1\n        text: 本文。\n"; string(out) != wantText {
		t.Errorf("Rewrite wrote\n%q\nwant\n%q", string(out), wantText)
	}
}

// TestRewriteRewritesOneRuleAndKeepsTheRest: 段 4 の golden, through Rewrite: the
// changed rule's own lines are written and every other line is byte for byte.
func TestRewriteRewritesOneRuleAndKeepsTheRest(t *testing.T) {
	saved := parseHand(t)
	want := parseHand(t)
	want.Sections[0].Rules[1].Note = "書き換えたメモ"

	out, err := Rewrite([]byte(handWritten), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	wantText := strings.Replace(handWritten,
		"note: T2 のメモ", "note: 書き換えたメモ", 1)
	if string(out) != wantText {
		t.Errorf("Rewrite wrote\n%q\nwant\n%q", string(out), wantText)
	}
	for _, kept := range []string{"why: '理由はシングルクォートで書いてある'", "refs: [d-1, d-2]", "# T3 は最後の規則"} {
		if !strings.Contains(string(out), kept) {
			t.Errorf("the write changed %q:\n%s", kept, out)
		}
	}
}

// TestRewriteKeepsARuleOutOfThePrompt: the switch the screen does not offer yet
// (d-f187) travels in the draft, so the line stays where it is when the rule is
// written back, and a draft that brings the rule into the prompt takes the line
// out.
func TestRewriteKeepsARuleOutOfThePrompt(t *testing.T) {
	const source = `version: 1
title: 設計実装
sections:
  - heading: 作り方
    rules:
      - id: BUILD-1
        text: 最初の本文。
        in_prompt: false
      - id: BUILD-2
        text: 2 番目の本文。
`
	saved := mustParse(t, source, "roles/builder.yaml")

	// A draft that edits the other rule leaves the line alone.
	want := mustParse(t, source, "roles/builder.yaml")
	want.Sections[0].Rules[1].Text = "2 番目を書き換えた本文。"
	out, err := Rewrite([]byte(source), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "in_prompt: false") {
		t.Errorf("the write dropped in_prompt: false from the rule it did not touch:\n%s", out)
	}
	if mustParse(t, string(out), "roles/builder.yaml").Sections[0].Rules[0].Included() {
		t.Error("the rule came back into the prompt")
	}

	// A draft that edits the rule itself writes the line back with it.
	want = mustParse(t, source, "roles/builder.yaml")
	want.Sections[0].Rules[0].Text = "最初を書き換えた本文。"
	out, err = Rewrite([]byte(source), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "in_prompt: false") {
		t.Errorf("the write dropped in_prompt: false from the rule it rewrote:\n%s", out)
	}

	// A draft that brings the rule into the prompt takes the line out.
	want = mustParse(t, source, "roles/builder.yaml")
	want.Sections[0].Rules[0].Text = "最初を書き換えた本文。"
	want.Sections[0].Rules[0].InPrompt = nil
	out, err = Rewrite([]byte(source), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "in_prompt") {
		t.Errorf("the write kept in_prompt for a rule the draft brought into the prompt:\n%s", out)
	}
}

// TestRewriteAddsARuleAtTheEnd: 足す writes the new rule after the last one.
func TestRewriteAddsARuleAtTheEnd(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Sections[0].Rules = append(want.Sections[0].Rules, Rule{ID: "BUILD-6", Text: "足した本文。"})

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "      - id: BUILD-6\n        text: 足した本文。\n") {
		t.Errorf("the added rule is not there:\n%s", out)
	}
	back := mustParse(t, string(out), "roles/builder.yaml")
	if len(back.Rules()) != 6 {
		t.Errorf("the written document holds %d rules, want 6", len(back.Rules()))
	}
}

// TestRewriteRemovesARule: 消す takes the rule's own lines out.
func TestRewriteRemovesARule(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Sections[0].Rules = append(want.Sections[0].Rules[:1], want.Sections[0].Rules[2])
	// BUILD-2 is gone; BUILD-3 keeps its place in the draft.

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "BUILD-2") {
		t.Errorf("the removed rule is still there:\n%s", out)
	}
	back := mustParse(t, string(out), "roles/builder.yaml")
	if back.Sections[0].Rules[0].ID != "BUILD-1" || back.Sections[0].Rules[1].ID != "BUILD-3" {
		t.Errorf("the section holds %+v", back.Sections[0].Rules)
	}
}

// TestRewriteMovesARuleWithinItsSection: F2 — BUILD-2 moves down, and only the
// lines of BUILD-2's item move; the other rules keep every byte.
func TestRewriteMovesARuleWithinItsSection(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	rules := want.Sections[0].Rules
	want.Sections[0].Rules = []Rule{rules[0], rules[2], rules[1]}

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	wantText := strings.Replace(builderRole,
		"      - id: BUILD-2\n        text: 2 番目の本文。\n      - id: BUILD-3\n        text: 3 番目の本文。\n",
		"      - id: BUILD-3\n        text: 3 番目の本文。\n      - id: BUILD-2\n        text: 2 番目の本文。\n", 1)
	if string(out) != wantText {
		t.Errorf("Rewrite wrote\n%q\nwant\n%q", string(out), wantText)
	}
}

// TestRewriteMovesARuleBetweenItsNeighbours: F2, R4 — a rule swapped with the
// one after it in the middle of a section, neither of them the section's last:
// the two items' own lines trade places and nothing else changes. A neighbour
// that is the section's last rule is the easy case; this is the one that was
// refused before the run was read as the longest one that stays.
func TestRewriteMovesARuleBetweenItsNeighbours(t *testing.T) {
	const role = `version: 1
title: 進行管理
revision: 1
sections:
  - heading: 使命
    rules:
      - id: LEAD-1
        text: 最初の本文。
        origin:
          - kind: instruction
            date: "2026-09-14"
            refs: [d-4c11]
      - id: LEAD-2
        text: 2 番目の本文。
      - id: LEAD-3
        text: 3 番目の本文。
      - id: LEAD-4
        text: 4 番目の本文。
`
	saved := mustParse(t, role, "roles/lead.yaml")
	want := mustParse(t, role, "roles/lead.yaml")
	r := want.Sections[0].Rules
	want.Sections[0].Rules = []Rule{r[0], r[2], r[1], r[3]}

	out, err := Rewrite([]byte(role), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	wantText := strings.Replace(role,
		"      - id: LEAD-2\n        text: 2 番目の本文。\n      - id: LEAD-3\n        text: 3 番目の本文。\n",
		"      - id: LEAD-3\n        text: 3 番目の本文。\n      - id: LEAD-2\n        text: 2 番目の本文。\n", 1)
	if string(out) != wantText {
		t.Errorf("Rewrite wrote\n%q\nwant\n%q", string(out), wantText)
	}
}

// TestRewriteMovesARuleToTheNextSection: a rule moved to another section keeps
// its values and the section it left loses it.
func TestRewriteMovesARuleToTheNextSection(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	moved := want.Sections[0].Rules[2]
	want.Sections[0].Rules = want.Sections[0].Rules[:2]
	want.Sections[1].Rules = append([]Rule{moved}, want.Sections[1].Rules...)

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	back := mustParse(t, string(out), "roles/builder.yaml")
	if len(back.Sections[0].Rules) != 2 {
		t.Errorf("the first section holds %d rules, want 2", len(back.Sections[0].Rules))
	}
	if len(back.Sections[1].Rules) != 3 || back.Sections[1].Rules[0].ID != "BUILD-3" {
		t.Errorf("the second section holds %+v", back.Sections[1].Rules)
	}
}

// TestRewriteAddsASection: 節を足す writes the whole section at the end.
func TestRewriteAddsASection(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Sections = append(want.Sections, Section{Heading: "新しい節"})

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "  - heading: 新しい節\n") {
		t.Errorf("the new section is not there:\n%s", out)
	}
	back := mustParse(t, string(out), "roles/builder.yaml")
	if len(back.Sections) != 3 || back.Sections[2].Heading != "新しい節" {
		t.Errorf("the written document holds %+v", back.Sections)
	}
}

// TestRewriteRemovesAMiddleSection: a section taken out from the middle takes
// its rules with it, and the sections after it keep their values.
func TestRewriteRemovesAMiddleSection(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Sections = want.Sections[1:]

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	back := mustParse(t, string(out), "roles/builder.yaml")
	if len(back.Sections) != 1 || back.Sections[0].Heading != "確かめ方" {
		t.Errorf("the written document holds %+v", back.Sections)
	}
	if strings.Contains(string(out), "作り方") || strings.Contains(string(out), "BUILD-1") {
		t.Errorf("the removed section is still there:\n%s", out)
	}
}

// TestRewriteWithNoChangeKeepsTheFile: a draft whose values are the ones the file
// holds is not written at all.
func TestRewriteWithNoChangeKeepsTheFile(t *testing.T) {
	saved := parseHand(t)
	out, err := Rewrite([]byte(handWritten), saved, Draft{Doc: parseHand(t), From: fromByID(t, saved, parseHand(t))})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != handWritten {
		t.Errorf("Rewrite rewrote a document whose values did not change:\n%q", out)
	}
}

// TestRewriteRefusesAFlowShape: a document whose sections are one flow sequence
// is refused with error.edit_shape, naming the document.
func TestRewriteRefusesAFlowShape(t *testing.T) {
	const text = "version: 1\nsections: [{rules: [{id: T1, text: x}]}]\n"
	saved := mustParse(t, text, "roles/lead.yaml")
	want := mustParse(t, text, "roles/lead.yaml")
	want.Sections[0].Rules[0].Text = "y"

	_, err := Rewrite([]byte(text), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err == nil {
		t.Fatal("a flow-shaped document was written")
	}
	if got := err.Error(); !strings.Contains(got, "roles/lead.yaml") {
		t.Errorf("error = %q, want the document's name", got)
	}
}

// TestRewriteAddsASectionKeyItDidNotHold: a section written without a heading
// gets one at the head of its item, and the rules: key moves to its own line.
func TestRewriteAddsASectionKeyItDidNotHold(t *testing.T) {
	const text = "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: 本文。\n"
	saved := mustParse(t, text, "roles/lead.yaml")
	want := mustParse(t, text, "roles/lead.yaml")
	want.Sections[0].Heading = "使命"

	out, err := Rewrite([]byte(text), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	wantText := "version: 1\nsections:\n  - heading: 使命\n    rules:\n      - id: R1\n        text: 本文。\n"
	if string(out) != wantText {
		t.Errorf("Rewrite wrote\n%q\nwant\n%q", string(out), wantText)
	}
}

// TestRewriteEmptiesATitle: a title the draft clears comes out of the file.
func TestRewriteEmptiesATitle(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Title = ""

	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "title:") {
		t.Errorf("the title stayed:\n%s", out)
	}
	if !strings.Contains(string(out), "revision: 5\n") {
		t.Errorf("another line went with it:\n%s", out)
	}
}

// TestWriteRefusalStillNamesTheDocument keeps the shape: Write's refusal names
// the file the caller shows.
func TestWriteRefusalStillNamesTheDocument(t *testing.T) {
	doc := parseHand(t)
	_, err := Write([]byte(handWritten), doc, Edit{Section: 9, Rule: 0})
	if err == nil {
		t.Fatal("an impossible place was written")
	}
	if want := i18n.Tf("error.edit_shape", "agreement.yaml", "sections"); err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

// TestRewriteCarriesAMovedSection: a section the draft moved keeps its item's
// lines and is written where the draft puts it; every other line stays byte for
// byte (C7, spec-roles I4).
func TestRewriteCarriesAMovedSection(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Sections[0], want.Sections[1] = want.Sections[1], want.Sections[0]
	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want), SectionFrom: []int{1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	const expect = `version: 1
title: 設計実装
revision: 5
sections:
  - heading: 確かめ方
    rules:
      - id: BUILD-4
        text: 4 番目の本文。
      - id: BUILD-5
        text: 5 番目の本文。
  - heading: 作り方
    rules:
      - id: BUILD-1
        text: 最初の本文。
      - id: BUILD-2
        text: 2 番目の本文。
      - id: BUILD-3
        text: 3 番目の本文。
`
	if string(out) != expect {
		t.Errorf("the moved sections are not the two items swapped:\n%s", out)
	}
}

// TestRewriteCarriesTheLastSectionToTheFront: the section written last is moved
// to the front, with the section that held a comment keeping it (C7).
func TestRewriteCarriesTheLastSectionToTheFront(t *testing.T) {
	const text = `version: 1
title: T
sections:
  # 最初の節
  - heading: A
    rules:
      - id: A-1
        text: one
  - heading: B
    rules:
      - id: B-1
        text: two
`
	saved := mustParse(t, text, "roles/x.yaml")
	want := mustParse(t, text, "roles/x.yaml")
	want.Sections[0], want.Sections[1] = want.Sections[1], want.Sections[0]
	out, err := Rewrite([]byte(text), saved, Draft{Doc: want, From: fromByID(t, saved, want), SectionFrom: []int{1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	const expect = `version: 1
title: T
sections:
  - heading: B
    rules:
      - id: B-1
        text: two
  # 最初の節
  - heading: A
    rules:
      - id: A-1
        text: one
`
	if string(out) != expect {
		t.Errorf("the last section did not land first:\n%s", out)
	}
}

// TestRewriteCarriesAMovedSectionAndWritesIt: a section moved and changed inside
// is carried to its new place with the change written on its own lines (C7).
func TestRewriteCarriesAMovedSectionAndWritesIt(t *testing.T) {
	saved := mustParse(t, builderRole, "roles/builder.yaml")
	want := mustParse(t, builderRole, "roles/builder.yaml")
	want.Sections[0], want.Sections[1] = want.Sections[1], want.Sections[0]
	want.Sections[1].Rules[0].Text = "1 番目の本文を直した。"
	out, err := Rewrite([]byte(builderRole), saved, Draft{Doc: want, From: fromByID(t, saved, want), SectionFrom: []int{1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	i4 := strings.Index(got, "確かめ方")
	i1 := strings.Index(got, "作り方")
	if i4 < 0 || i1 < 0 || i4 > i1 {
		t.Errorf("the sections are not in the draft's order:\n%s", got)
	}
	if !strings.Contains(got, "- id: BUILD-1\n        text: 1 番目の本文を直した。\n") {
		t.Errorf("the change inside the moved section was not written:\n%s", got)
	}
	if !strings.Contains(got, "- id: BUILD-2\n        text: 2 番目の本文。\n") {
		t.Errorf("a line the draft did not touch changed:\n%s", got)
	}
}

// TestAFailedSaveNamesTheScreenTheDocumentSavesFrom: a write hergy cannot place
// is refused with error.internal_bug, and its way around names the YAML tab of
// the screen the person is on. A team's agreement saves from the team
// definition's screen; a role document saves from the roles screen (r-dfaa,
// d-040e E3).
func TestAFailedSaveNamesTheScreenTheDocumentSavesFrom(t *testing.T) {
	for _, tc := range []struct {
		name string
		from SaveScreen
		path string
		want string
	}{
		{"team agreement", SaveFromTeamDefinition, ".hergy/teams/demo/agreement.yaml", "bug.workaround_team_yaml"},
		{"role document", SaveFromRoles, ".hergy/roles/lead.yaml", "bug.workaround_role_yaml"},
	} {
		doc, err := Parse(builderRole, tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		// The written text reads as another document, which is what the read-back
		// catches; the sentence is the one the screen handed in.
		err = sameValuesAs(doc, []byte("version: 1\nsections: []\n"), tc.from)
		if err == nil {
			t.Fatalf("%s: a write that reads as another document was accepted", tc.name)
		}
		if want := i18n.T(tc.want); !strings.Contains(err.Error(), want) {
			t.Errorf("%s: the refusal does not say %q:\n%s", tc.name, want, err)
		}
		other := "bug.workaround_team_yaml"
		if tc.want == other {
			other = "bug.workaround_role_yaml"
		}
		if strings.Contains(err.Error(), i18n.T(other)) {
			t.Errorf("%s: the refusal names the other screen:\n%s", tc.name, err)
		}
	}
}
