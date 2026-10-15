package rulesview

import (
	"fmt"
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/web"
)

// The rules screen draws no goroutine work, so this TestMain only loads the
// locales. When the screen grows handlers, the goleak check goes here with the
// WebUI's (d-b28a), the way internal/web does it.

func TestMain(m *testing.M) {
	if err := i18n.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// funcs are the template functions a test renders with: the WebUI's own list
// (so the guide bar and the paths are the ones hergy draws with), with escaping
// only text in place of web.Inline, which also draws **bold** and `code`
// (spec/10 の 4 節).
func funcs() template.FuncMap {
	merged := web.Funcs()
	merged["inline"] = func(text string) template.HTML {
		return template.HTML(template.HTMLEscapeString(text))
	}
	return merged
}

func renderer(t *testing.T) *Renderer {
	t.Helper()
	r, err := NewRenderer(funcs())
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	return r
}

// marked is a message a button carries, with the leading mark trimmed: the
// template writes a "+" as &#43;, so an assertion on the words does not have to
// know how html/template spells it.
func marked(key string) string {
	return strings.TrimSpace(strings.TrimLeft(i18n.T(key), "+＋"))
}

// sample is a document with a heading, a rule with everything, a rule with
// nothing, a section with no heading, and a report shape.
const sample = `version: 1
title: TEAM PROTOCOL
revision: 3
sections:
  - heading: 通信
    style: ids
    rules:
      - id: C1
        text: 手番を渡す。
        why: 完了を記録する主体がいなくなる。
        note: 保守のメモ
        report: 監査報告.scope
        tests: [T-1]
        origin:
          - kind: incident
            date: 2026-09-25
            refs: ["#6100"]
            note: 記録されないまま残った
      - id: C2
        text: 返事を待つ。
  - rules:
      - id: C3
        text: 続きの節である。
reports:
  監査報告:
    where: 記録のコメント
    fields:
      scope: 変更の範囲
`

func sampleDoc(t *testing.T, layer string) Doc {
	t.Helper()
	document, err := rules.Parse(sample, "protocol.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return Doc{Path: "protocol.yaml", Display: ".hergy/protocol.yaml", Group: GroupDoc, GroupLabel: "COMMON", Layer: layer, Document: document, UsedBy: []string{"agile-dev"}}
}

func TestRulesRendersTheCountsFiltersAndSections(t *testing.T) {
	doc := sampleDoc(t, LayerLocal)
	data := NewRules(doc, FilterAll)
	got, err := renderer(t).Rules(data)
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	for _, want := range []string{
		"TEAM PROTOCOL",        // the title
		"COMMON",               // the group the app names for the file
		".hergy/protocol.yaml", // the file as a person writes it
		i18n.Tf("web.rules.tally.rules", 3),
		i18n.Tf("web.rules.tally.origin", 1),
		i18n.Tf("web.rules.tally.reason", 1),
		i18n.Tf("web.rules.tally.check", 1),
		i18n.Tf("web.rules.version", 3),
		"C1", "C2", "C3",
		"手番を渡す。",
		i18n.Tf("web.rules.rule.report", "監査報告.scope"),
		i18n.Tf("web.rules.rule.tests", "T-1"),
		i18n.T("web.rules.rule.note"),
		i18n.T("web.rules.section.no_heading"),
		i18n.T("web.rules.style.ids"),
		marked("web.rules.rule.add"),
		i18n.Tf("web.rules.doc.used_by", "agile-dev"),
		// The four steps stand above the screen (the guide bar).
		i18n.T("web.guide.step.fix"),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the rules body is missing %q:\n%s", want, got)
		}
	}
	// The rule with a reason that is sent shows it in the list.
	if !strings.Contains(got, "完了を記録する主体がいなくなる。") {
		t.Errorf("the reason of a rule that sends it is not shown:\n%s", got)
	}
	// The row carries the three marks of a rule, and the open one is marked.
	if !strings.Contains(got, `class="ev o on"`) || !strings.Contains(got, `class="ev v on"`) {
		t.Errorf("the rule rows do not carry the origin and check marks:\n%s", got)
	}
}

func TestRulesFilterKeepsOnlyWhatItSays(t *testing.T) {
	doc := sampleDoc(t, LayerLocal)
	data := NewRules(doc, FilterNoOrigin)
	got, err := renderer(t).Rules(data)
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	if strings.Contains(got, ">C1<") {
		t.Errorf("a rule with an origin is shown under the no-origin filter:\n%s", got)
	}
	for _, id := range []string{"C2", "C3"} {
		if !strings.Contains(got, id) {
			t.Errorf("the no-origin filter dropped %s:\n%s", id, got)
		}
	}
	if data.Counts.NoOrigin != 2 {
		t.Errorf("NoOrigin = %d, want 2", data.Counts.NoOrigin)
	}
}

// TestRulesSaysWhenTheFilterKeepsNothing: a filter that keeps no rule says so,
// and a layer with no document says that instead.
func TestRulesSaysWhenTheFilterKeepsNothing(t *testing.T) {
	everyRuleHasACheck := sampleDoc(t, LayerLocal)
	addCheckToEveryRule(everyRuleHasACheck.Document)
	data := NewRules(everyRuleHasACheck, FilterNoCheck)
	if !data.NoMatch {
		t.Errorf("a filter that keeps nothing is not reported: %+v", data.Sections)
	}
	got, err := renderer(t).Rules(data)
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	if !strings.Contains(got, i18n.T("web.rules.filter.empty")) {
		t.Errorf("the emptied screen does not say so:\n%s", got)
	}

	empty := NewRules(Doc{Layer: LayerLocal}, FilterAll)
	if !empty.Empty {
		t.Error("a document that could not be read is not reported as empty")
	}
	body, err := renderer(t).Rules(empty)
	if err != nil {
		t.Errorf("Rules of an empty document: %v", err)
	}
	if !strings.Contains(body, i18n.T("web.rules.empty")) {
		t.Errorf("the empty layer does not say so:\n%s", body)
	}
}

// addCheckToEveryRule gives every rule a report field, so the no-check filter
// keeps nothing.
func addCheckToEveryRule(doc *rules.Document) {
	for i := range doc.Sections {
		for j := range doc.Sections[i].Rules {
			doc.Sections[i].Rules[j].Report = "監査報告.scope"
		}
	}
}

func TestPanelRendersTheFieldsTheOriginAndThePreview(t *testing.T) {
	doc := sampleDoc(t, LayerLocal)
	rule := doc.Document.Sections[0].Rules[0]
	data := Panel{
		Layer: LayerLocal,
		Doc:   doc,
		Edit:  NewEditRule(0, 0, rule),
		Reports: []ReportOption{
			{Ref: "監査報告.scope", Label: "監査報告.scope (TEAM PROTOCOL)"},
		},
		Preview: Preview{Section: "## 通信\n\n- **C1** 手番を渡す。"},
		Token:   "tok",
	}
	got, err := renderer(t).Panel(data)
	if err != nil {
		t.Fatalf("Panel: %v", err)
	}
	for _, want := range []string{
		i18n.T("web.rules.panel.zone.send"),
		i18n.T("web.rules.panel.zone.keep"),
		i18n.T("web.rules.field.id"),
		i18n.T("web.rules.field.text"),
		i18n.T("web.rules.field.why"),
		i18n.T("web.rules.field.send_why"),
		i18n.T("web.rules.field.origin"),
		i18n.T("web.rules.origin.what"),
		i18n.T("web.rules.origin.kind.incident"),
		marked("web.rules.origin.add"),
		i18n.T("web.rules.origin.remove"),
		i18n.T("web.rules.field.report"),
		i18n.T("web.rules.field.tests"),
		i18n.T("web.rules.panel.preview"),
		i18n.T("web.rules.panel.save"),
		i18n.T("web.rules.panel.delete"),
		"監査報告.scope",
		"手番を渡す。",
		"#6100", // the references of the origin, as they were read
		`action="/rules/save"`,
		`hx-target="#drawer"`, // the right panel's id (web.DrawerID)
		`value="tok"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the panel is missing %q:\n%s", want, got)
		}
	}
}

func TestPanelShowsTheBreakAndTheNewRuleForm(t *testing.T) {
	doc := sampleDoc(t, LayerGlobal)
	data := Panel{
		Layer:        LayerGlobal,
		Doc:          doc,
		Edit:         EditRule{Section: 0, Index: -1, ID: "C4"},
		Global:       true,
		New:          true,
		Token:        "tok",
		Broken:       true,
		Preview:      Preview{Break: "C4: text is blank"},
		AskingDelete: true,
	}
	got, err := renderer(t).Panel(data)
	if err != nil {
		t.Fatalf("Panel: %v", err)
	}
	for _, want := range []string{
		i18n.T("web.rules.panel.new_note"),
		i18n.T("web.rules.panel.global"),
		i18n.T("web.rules.panel.add"),
		i18n.T("web.rules.panel.delete_ask"),
		i18n.T("web.rules.panel.delete_confirm"),
		i18n.T("web.rules.panel.delete_cancel"),
		`formaction="/rules/delete"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the new-rule panel is missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, i18n.T("web.rules.panel.new_note")) {
		t.Errorf("the new-rule panel does not say what it does:\n%s", got)
	}
	preview, err := renderer(t).Preview(data)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !strings.Contains(preview, "C4: text is blank") {
		t.Errorf("the preview does not show the break: %s", preview)
	}
}

func TestPreviewShowsTheSectionAsAMemberReadsIt(t *testing.T) {
	got, err := renderer(t).Preview(Panel{Preview: Preview{Section: "## 通信\n\n- **C1** 手番を渡す。"}})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !strings.Contains(got, "手番を渡す。") {
		t.Errorf("the preview does not show the section: %s", got)
	}
	empty, err := renderer(t).Preview(Panel{})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !strings.Contains(empty, i18n.T("web.rules.panel.preview.empty")) {
		t.Errorf("an empty preview does not say so: %s", empty)
	}
}
