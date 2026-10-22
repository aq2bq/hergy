package rules

import "testing"

// A screen sends, beside the draft, where each section stood in the saved file
// (SectionFrom): a section with no rules therefore reads as one that stood
// there with no rules, not as a new one.

// TestRewriteSavesASectionWithNoRules: a file whose last section is a heading
// and no rules — the shape hergy writes when a person takes the section's last
// rule out — saves from the rules screen (r-3a00).
func TestRewriteSavesASectionWithNoRules(t *testing.T) {
	const text = `version: 1
title: 約束
revision: 2
sections:
  - heading: このプロジェクトの事情
    rules:
      - id: T10
        text: 一つ目の本文。
      - id: T11
        text: 二つ目の本文。
  - heading: 残り
`
	const once = `version: 1
title: 約束
revision: 2
sections:
  - heading: このプロジェクトの事情
    rules:
      - id: T10
        text: 一つ目の本文。
  - heading: 残り
`
	saved := mustParse(t, text, "agreement.yaml")
	want := mustParse(t, text, "agreement.yaml")
	want.Sections[0].Rules = want.Sections[0].Rules[:1]
	out, err := Rewrite([]byte(text), saved, Draft{Doc: want, From: fromByID(t, saved, want), SectionFrom: []int{0, 1}})
	if err != nil {
		t.Fatalf("the screen's save was refused: %v", err)
	}
	if string(out) != once {
		t.Fatalf("taking the last rule out wrote\n%q\nwant\n%q", out, once)
	}

	// The file now ends in a heading-only section; the same screen saves it
	// again without a change, and it is not written at all.
	resaved := mustParse(t, once, "agreement.yaml")
	again, err := Rewrite([]byte(once), resaved, Draft{
		Doc:         resaved,
		From:        []Place{{Section: 0, Rule: 0}},
		SectionFrom: []int{0, 1},
	})
	if err != nil {
		t.Fatalf("the heading-only section was refused: %v", err)
	}
	if string(again) != once {
		t.Fatalf("a save with no change wrote\n%q\nwant\n%q", again, once)
	}
}

// TestRewriteAddsARuleToASectionWrittenAsEmptyMapping: a section the file
// writes as - {} has no key to stand a rule under, so a draft that gives it one
// is written whole, the way Emit writes a section with rules.
func TestRewriteAddsARuleToASectionWrittenAsEmptyMapping(t *testing.T) {
	const text = `version: 1
sections:
  - heading: 使命
    rules:
      - id: R1
        text: 本文。
  - {}
`
	const once = `version: 1
sections:
  - heading: 使命
    rules:
      - id: R1
        text: 本文。
  - rules:
      - id: R2
        text: 足した本文。
`
	saved := mustParse(t, text, "roles/x.yaml")
	want := mustParse(t, text, "roles/x.yaml")
	want.Sections[1].Rules = []Rule{{ID: "R2", Text: "足した本文。"}}
	out, err := Rewrite([]byte(text), saved, Draft{Doc: want, From: fromByID(t, saved, want), SectionFrom: []int{0, 1}})
	if err != nil {
		t.Fatalf("the screen's save was refused: %v", err)
	}
	if string(out) != once {
		t.Fatalf("the rule added to - {} wrote\n%q\nwant\n%q", out, once)
	}
}
