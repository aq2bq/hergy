package rulesview

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/rules"
)

func TestNewRuleCarriesTheThreeMarks(t *testing.T) {
	full := NewRule(0, 0, rules.Rule{
		ID: "C1", Text: "x", Why: "y",
		Origin: []rules.Origin{{Kind: rules.KindIncident}},
		Report: "R.f",
	})
	if !full.Badges.Origin || !full.Badges.Reason || !full.Badges.Check {
		t.Errorf("the marks of a full rule = %+v, want all three", full.Badges)
	}
	bare := NewRule(0, 0, rules.Rule{ID: "C2", Text: "x"})
	if bare.Badges.Origin || bare.Badges.Reason || bare.Badges.Check {
		t.Errorf("the marks of a bare rule = %+v, want none", bare.Badges)
	}
}

func TestNewRulesDropsTheSectionsTheFilterEmpties(t *testing.T) {
	doc := sampleDoc(t, LayerLocal)
	all := NewRules(doc, FilterAll)
	if len(all.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(all.Sections))
	}
	if all.Sections[0].Heading != "通信" || all.Sections[0].Hash != "##" || all.Sections[0].StyleLabel == "" {
		t.Errorf("the first section = %+v", all.Sections[0])
	}
	if all.Sections[1].Heading != "" {
		t.Errorf("a section with no heading is not shown as a continuation: %+v", all.Sections[1])
	}
	if only := NewSection(0, rules.Section{Heading: "H"}, FilterAll); !only.HeadingOnly {
		t.Errorf("a section with a heading and no rules is not marked heading-only: %+v", only)
	}
	if cont := NewSection(0, rules.Section{}, FilterAll); cont.HeadingOnly {
		t.Errorf("a section with no heading and no rules is marked heading-only: %+v", cont)
	}
	noOrigin := NewRules(doc, FilterNoOrigin)
	if len(noOrigin.Sections) != 2 {
		t.Fatalf("the no-origin filter kept %d sections, want 2", len(noOrigin.Sections))
	}
	if got := len(noOrigin.Sections[0].Rules); got != 1 {
		t.Errorf("the first section kept %d rules under the no-origin filter, want 1", got)
	}
	if !strings.Contains(noOrigin.Filters[1].Href, "filter=origin") {
		t.Errorf("the no-origin chip's link = %s", noOrigin.Filters[1].Href)
	}
	if !strings.HasPrefix(noOrigin.PanelPrefix, PathPanel+"?") {
		t.Errorf("the panel prefix = %s", noOrigin.PanelPrefix)
	}
	// An unknown filter is all, not a filter that keeps nothing.
	unknown := NewRules(doc, "nonsense")
	if unknown.Filter != FilterAll || unknown.NoMatch {
		t.Errorf("an unknown filter was not read as all: %+v", unknown)
	}
}

func TestCountRules(t *testing.T) {
	doc := sampleDoc(t, LayerLocal)
	c := CountRules(doc.Document)
	want := Counts{Rules: 3, Origin: 1, Reason: 1, Check: 1, NoOrigin: 2, NoReason: 2, NoCheck: 2}
	if c != want {
		t.Errorf("CountRules = %+v, want %+v", c, want)
	}
}

func TestKeepsReadsTheThreeFilters(t *testing.T) {
	withOrigin := rules.Rule{ID: "R1", Text: "x", Origin: []rules.Origin{{Kind: rules.KindIncident}}}
	withReason := rules.Rule{ID: "R2", Text: "x", Why: "because"}
	withCheck := rules.Rule{ID: "R3", Text: "x", Report: "A.f"}
	cases := []struct {
		name   string
		rule   rules.Rule
		filter string
		want   bool
	}{
		{"all keeps everything", withOrigin, FilterAll, true},
		{"no-origin drops a rule with one", withOrigin, FilterNoOrigin, false},
		{"no-origin keeps a rule without one", withReason, FilterNoOrigin, true},
		{"no-reason drops a rule with one", withReason, FilterNoReason, false},
		{"no-check drops a rule with a report", withCheck, FilterNoCheck, false},
		{"an unknown filter keeps a rule", withOrigin, "nonsense", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Keeps(tc.rule, tc.filter); got != tc.want {
				t.Errorf("Keeps(%+v, %q) = %v, want %v", tc.rule, tc.filter, got, tc.want)
			}
		})
	}
}

// TestEditRuleRoundTrips: what the form holds and what the file holds are the
// same rule, so a save writes back what the person sees.
func TestEditRuleRoundTrips(t *testing.T) {
	document, err := rules.Parse(`version: 1
sections:
  - heading: 通信
    rules:
      - id: P-TURN-1
        text: |-
          本文である。
          続きである。
        why: 理由である。
        send_why: false
        note: メモである。
        report: 監査.scope
        tests: [T-1, T-2]
        origin:
          - kind: incident
            date: 2026-09-25
            refs: ["#6100"]
            note: 起きたこと
          - kind: derived
            refs: [C1]
reports:
  監査:
    where: 記録
    fields:
      scope: 範囲
`, "protocol.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, want := range []rules.Rule{
		document.Sections[0].Rules[0],
		{ID: "R", Text: "x"}, // the defaults: no why, the reason sent
	} {
		form := NewEditRule(0, 0, want)
		if got := form.Rule(); !sameRule(t, got, want) {
			t.Errorf("the form changed the rule:\ngot  %+v\nwant %+v", got, want)
		}
	}
}

// sameRule compares two rules by writing them both out: the default send_why
// (true) may be written or left out, and means the same either way.
func sameRule(t *testing.T, a, b rules.Rule) bool {
	t.Helper()
	write := func(rule rules.Rule) string {
		doc := &rules.Document{
			Version:  1,
			Sections: []rules.Section{{Rules: []rules.Rule{rule}}},
		}
		text, err := rules.Emit(doc)
		if err != nil {
			t.Fatalf("Emit: %v", err)
		}
		parsed, err := rules.Parse(text, "rules.yaml")
		if err != nil {
			t.Fatalf("the rule the form made does not read back: %v\n%s", err, text)
		}
		return mustEmit(t, parsed)
	}
	return write(a) == write(b)
}

func mustEmit(t *testing.T, doc *rules.Document) string {
	t.Helper()
	text, err := rules.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	return text
}

func TestEditRuleDropsAnOriginWithNoKind(t *testing.T) {
	form := EditRule{
		ID:   "R1",
		Text: "x",
		Send: true,
		Origins: []OriginRow{
			{Kind: "", Date: "2026-01-01", Note: "no kind chosen"},
			{Kind: rules.KindInstruction, Date: "2026-01-02", Refs: " a , b "},
		},
	}
	rule := form.Rule()
	if len(rule.Origin) != 1 {
		t.Fatalf("origins = %+v, want the row with no kind dropped", rule.Origin)
	}
	if got := strings.Join(rule.Origin[0].Refs, ","); got != "a,b" {
		t.Errorf("refs = %q, want a,b", got)
	}
	if rule.SendWhy != nil {
		t.Errorf("a rule whose reason is sent carries send_why = %v, want nil", rule.SendWhy)
	}
}

func TestSplitList(t *testing.T) {
	got := SplitList(" a , ,b, c ")
	if strings.Join(got, "|") != "a|b|c" {
		t.Errorf("SplitList = %v, want a, b, c", got)
	}
	if SplitList("") != nil {
		t.Errorf("SplitList of an empty field = %v, want nothing", SplitList(""))
	}
}

func TestReportOptionsListsTheFieldsWithTheirDocument(t *testing.T) {
	doc := sampleDoc(t, LayerLocal)
	opts := ReportOptions([]Doc{doc, {Layer: LayerLocal}})
	if len(opts) != 1 {
		t.Fatalf("options = %+v, want one", opts)
	}
	if opts[0].Ref != "監査報告.scope" || !strings.Contains(opts[0].Label, "TEAM PROTOCOL") {
		t.Errorf("the option = %+v", opts[0])
	}
}

func TestOriginLabel(t *testing.T) {
	cases := []struct {
		name   string
		origin rules.Origin
		want   string
	}{
		{"an incident with a date and a reference", rules.Origin{Kind: rules.KindIncident, Date: "2026-09-25", Refs: []string{"#6100"}}, "an incident 2026-09-25 #6100"},
		{"an instruction with only a date", rules.Origin{Kind: rules.KindInstruction, Date: "2026-09-24"}, "an instruction 2026-09-24"},
		{"a derived rule", rules.Origin{Kind: rules.KindDerived, Refs: []string{"C1", "C2"}}, "derived ← C1 C2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OriginLabel(tc.origin); got != tc.want {
				t.Errorf("OriginLabel = %q, want %q", got, tc.want)
			}
		})
	}
}
