package rules

import (
	"strconv"
	"strings"
	"testing"
)

// documentWithRules is a document of n rules, to walk the provisional limits.
func documentWithRules(n int) *Document {
	doc := &Document{Version: 1}
	section := Section{}
	for i := range n {
		section.Rules = append(section.Rules, Rule{ID: "R" + strconv.Itoa(i), Text: "x"})
	}
	doc.Sections = []Section{section}
	return doc
}

func TestMemberSizeCountsTheRulesAndTheRenderedCharacters(t *testing.T) {
	doc := parse(t, `version: 1
title: T
sections:
  - heading: H
    intro: I
    rules:
      - id: R1
        text: one
      - id: R2
        text: two
        why: because
`)
	var size MemberSize
	size.Add(doc)
	if size.Rules != 2 {
		t.Errorf("Rules = %d, want 2", size.Rules)
	}
	// The characters are the ones of the Markdown hergy renders: the title, the
	// heading, the intro, the rules and the reason, and not a prompt's frame.
	want := CharCount("# T\n\n## H\n\nI\n\none\n\ntwo because\n")
	if size.Chars != want {
		t.Errorf("Chars = %d, want %d", size.Chars, want)
	}
	if size.Chars != len([]rune(doc.Markdown())) {
		t.Errorf("Chars = %d, the render is %d characters", size.Chars, len([]rune(doc.Markdown())))
	}
}

// TestMemberSizeCountsADocumentTwiceWhenItIsHandedTwice: the same text handed
// over twice is two handovers, so a file split in two cannot hide the amount.
func TestMemberSizeCountsADocumentTwiceWhenItIsHandedTwice(t *testing.T) {
	doc := parse(t, "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: one\n")
	var size MemberSize
	size.Add(doc)
	size.Add(doc)
	if size.Rules != 2 {
		t.Errorf("Rules = %d, want the document counted twice", size.Rules)
	}
	if got, want := size.Chars, 2*CharCount(doc.Markdown()); got != want {
		t.Errorf("Chars = %d, want %d", got, want)
	}
}

// TestMemberSizeIsOverItsLimits: a value exactly on a limit is not over; one
// more is.
func TestMemberSizeIsOverItsLimits(t *testing.T) {
	cases := []struct {
		size    MemberSize
		rules   bool
		chars   bool
		howCome string
	}{
		{MemberSize{Rules: MemberRuleLimit}, false, false, "exactly the rule limit"},
		{MemberSize{Rules: MemberRuleLimit + 1}, true, false, "one rule over"},
		{MemberSize{Chars: MemberCharLimit}, false, false, "exactly the character limit"},
		{MemberSize{Chars: MemberCharLimit + 1}, false, true, "one character over"},
		{MemberSize{}, false, false, "an empty member"},
	}
	for _, tc := range cases {
		t.Run(tc.howCome, func(t *testing.T) {
			if got := tc.size.OverMemberRules(); got != tc.rules {
				t.Errorf("OverMemberRules = %v, want %v", got, tc.rules)
			}
			if got := tc.size.OverMemberChars(); got != tc.chars {
				t.Errorf("OverMemberChars = %v, want %v", got, tc.chars)
			}
		})
	}
	// The counts a member is really handed agree with the limits.
	var many MemberSize
	many.Add(documentWithRules(MemberRuleLimit))
	if many.OverMemberRules() {
		t.Error("a member handed exactly the rule limit is over it")
	}
	many.Add(documentWithRules(1))
	if !many.OverMemberRules() {
		t.Error("a member handed one rule over the limit is not over it")
	}
}

// TestTeamSizeCountsASourceOnce: a person maintains a file once, however many
// members read it.
func TestTeamSizeCountsASourceOnce(t *testing.T) {
	var size TeamSize
	size.AddSource("protocol.yaml", "abc")
	size.AddSource("roles/lead.yaml", "de")
	size.AddSource("protocol.yaml", "abc") // the second member reads the same file
	if size.Sources != 2 {
		t.Errorf("Sources = %d, want 2", size.Sources)
	}
	if size.Chars != 5 {
		t.Errorf("Chars = %d, want 5", size.Chars)
	}
}

func TestTeamSizeIsOverItsLimits(t *testing.T) {
	cases := []struct {
		size    TeamSize
		sources bool
		chars   bool
		howCome string
	}{
		{TeamSize{Sources: TeamSourceLimit}, false, false, "exactly the source limit"},
		{TeamSize{Sources: TeamSourceLimit + 1}, true, false, "one source over"},
		{TeamSize{Chars: TeamSourceCharLimit}, false, false, "exactly the character limit"},
		{TeamSize{Chars: TeamSourceCharLimit + 1}, false, true, "one character over"},
	}
	for _, tc := range cases {
		t.Run(tc.howCome, func(t *testing.T) {
			if got := tc.size.OverTeamSources(); got != tc.sources {
				t.Errorf("OverTeamSources = %v, want %v", got, tc.sources)
			}
			if got := tc.size.OverTeamChars(); got != tc.chars {
				t.Errorf("OverTeamChars = %v, want %v", got, tc.chars)
			}
		})
	}
}

// TestCharCountCountsCodePointsAndBringsCRLFToLF: a file written on Windows
// counts the same as one written here, and a character outside the BMP is one
// character.
func TestCharCountCountsCodePointsAndBringsCRLFToLF(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"a\r\nb", 3},
		{"a\nb", 3},
		{"あいう", 3},
		{"🙂", 1},
		{strings.Repeat("x", 100), 100},
	}
	for _, tc := range cases {
		if got := CharCount(tc.text); got != tc.want {
			t.Errorf("CharCount(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}
