package rulesview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// roleSample is a role file with a report shape, so the report picker has more
// than one document to choose from (d-0c48).
const roleSample = `version: 1
sections:
  - rules:
      - id: L1
        text: 指示を待つ。
reports:
  完了報告:
    where: Issue のコメント
    fields:
      next_turn: 次に手番を持つ人
`

// fakePort is the app behind the screen in a test: the team definitions and the
// files of one layer, with the writes recorded.
type fakePort struct {
	teams      map[string][]Team
	files      map[string]string
	preview    Preview
	previewErr error
	writeErr   error
	wrote      []edit
}

type edit struct {
	Path, Text, Hash string
}

func newFake() *fakePort {
	return &fakePort{
		teams: map[string][]Team{
			LayerLocal: {{
				Name: "agile-dev",
				Members: []Member{
					{Name: "lead", Files: []File{{Path: "protocol.yaml"}, {Path: "roles/lead.yaml", Role: true}}},
					{Name: "impl", Files: []File{{Path: "protocol.yaml"}, {Path: "roles/impl.yaml", Role: true}}},
				},
			}},
		},
		files: map[string]string{
			"protocol.yaml":   sample,
			"roles/lead.yaml": roleSample,
			"roles/impl.yaml": "version: 1\nsections:\n  - rules:\n      - id: I1\n        text: 実装する。\n",
		},
	}
}

func hashOf(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func (p *fakePort) RuleTeams(layer string) []Team { return p.teams[layer] }

func (p *fakePort) ReadRule(path string) (Source, error) {
	text, ok := p.files[path]
	if !ok {
		return Source{}, fmt.Errorf("no rule file %s", path)
	}
	return Source{Path: path, Text: text, Hash: hashOf(text)}, nil
}

func (p *fakePort) WriteRule(path, text, hash string) error {
	if p.writeErr != nil {
		return p.writeErr
	}
	if hash != hashOf(p.files[path]) {
		return fmt.Errorf("the file changed on disk")
	}
	p.files[path] = text
	p.wrote = append(p.wrote, edit{Path: path, Text: text, Hash: hash})
	return nil
}

func (p *fakePort) Preview(_, _ string, _ int, _ string) (Preview, error) {
	if p.previewErr != nil {
		return Preview{}, p.previewErr
	}
	return p.preview, nil
}

func newScreen(t *testing.T, port Port) *Screen {
	t.Helper()
	screen, err := New(port, funcs())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return screen
}

func TestRulesBodyListsTheFilesAndShowsTheFirst(t *testing.T) {
	screen := newScreen(t, newFake())
	body, err := screen.RulesBody(LayerLocal, "", FilterAll)
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	for _, want := range []string{
		i18n.T("web.rules.group.doc"),
		i18n.T("web.rules.group.role"),
		"protocol.yaml", "lead.yaml", "impl.yaml",
		"roles%2Flead.yaml", // the role file's link, with the path escaped
		"TEAM PROTOCOL",     // the first document, shown by default
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the rules body is missing %q:\n%s", want, body)
		}
	}
	// The doc list is ordered by path, and the panel links carry the layer and
	// the document.
	if !strings.Contains(body, "/rules/panel?layer=local&amp;doc=protocol.yaml") {
		t.Errorf("the panel links do not carry the layer and the document:\n%s", body)
	}
}

func TestPanelBodyOpensARuleAndItsScope(t *testing.T) {
	port := newFake()
	port.preview = Preview{Section: "## 通信", Break: ""}
	screen := newScreen(t, port)
	body, err := screen.PanelBody(LayerLocal, "protocol.yaml", url.Values{"sec": {"0"}, "i": {"0"}, "token": {"tok"}})
	if err != nil {
		t.Fatalf("PanelBody: %v", err)
	}
	for _, want := range []string{
		"C1",
		"手番を渡す。",
		"監査報告.scope",     // a report of the same document
		"完了報告.next_turn", // a report of a role file the same member reads
		"## 通信",          // the preview
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the panel is missing %q:\n%s", want, body)
		}
	}
}

func TestPanelBodySuggestsAFreeIDForANewRule(t *testing.T) {
	screen := newScreen(t, newFake())
	body, err := screen.PanelBody(LayerLocal, "protocol.yaml", url.Values{"new": {"1"}, "sec": {"0"}, "token": {"tok"}})
	if err != nil {
		t.Fatalf("PanelBody: %v", err)
	}
	if !strings.Contains(body, i18n.T("web.rules.panel.new_note")) {
		t.Errorf("a new-rule panel does not say what it does:\n%s", body)
	}
	if !strings.Contains(body, `name="i" value="-1"`) {
		t.Errorf("a new rule is not marked as one:\n%s", body)
	}
}

func TestPanelBodyCloseAndAPlaceThatIsNotThere(t *testing.T) {
	screen := newScreen(t, newFake())
	body, err := screen.PanelBody(LayerLocal, "protocol.yaml", url.Values{"close": {"1"}})
	if err != nil {
		t.Fatalf("PanelBody close: %v", err)
	}
	if !strings.Contains(body, i18n.T("web.rules.panel.empty")) {
		t.Errorf("the closed panel does not show the placeholder: %s", body)
	}
	if _, err := screen.PanelBody(LayerLocal, "protocol.yaml", url.Values{"sec": {"9"}, "i": {"0"}}); err == nil {
		t.Error("a rule that is not there was not refused")
	}
	if _, err := screen.RulesBody(LayerGlobal, "", FilterAll); err != nil {
		t.Errorf("a layer with no team definition: %v", err)
	}
}

func TestSaveBodyWritesTheEditedRule(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{
		"token": {"tok"}, "layer": {LayerLocal}, "doc": {"protocol.yaml"},
		"section": {"0"}, "i": {"1"}, "id": {"C2"}, "text": {"直した本文。"},
		"send": {"1"}, "why": {"理由。"},
	}
	body, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if len(port.wrote) != 1 {
		t.Fatalf("wrote %d files, want 1", len(port.wrote))
	}
	written := port.wrote[0]
	if written.Path != "protocol.yaml" || written.Hash != hashOf(sample) {
		t.Errorf("the write = %+v", written)
	}
	back, err := rules.Parse(written.Text, "protocol.yaml")
	if err != nil {
		t.Fatalf("what was written does not read back: %v\n%s", err, written.Text)
	}
	got := back.Sections[0].Rules[1]
	if got.ID != "C2" || got.Text != "直した本文。" || got.Why != "理由。" {
		t.Errorf("the written rule = %+v", got)
	}
	if !strings.Contains(body, "直した本文。") {
		t.Errorf("the screen after the save does not show the new text:\n%s", body)
	}
}

func TestSaveBodyAddsARuleAtTheEndOfItsSection(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{
		"token": {"tok"}, "section": {"0"}, "i": {"-1"}, "id": {"C7"}, "text": {"足した規則。"}, "send": {"1"},
	}
	if _, err := screen.SaveBody(LayerLocal, "protocol.yaml", form); err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	back, err := rules.Parse(port.wrote[0].Text, "protocol.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if ids := ruleIDs(back); ids != "C1,C2,C7,C3" {
		t.Errorf("the rules after the add = %s, want C1,C2,C7,C3", ids)
	}
}

func TestSaveBodyComesBackToThePanelWhenTheWriteIsRefused(t *testing.T) {
	port := newFake()
	port.writeErr = fmt.Errorf("the file changed on disk")
	screen := newScreen(t, port)
	form := url.Values{"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"}, "text": {"x"}, "send": {"1"}}
	body, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if len(port.wrote) != 0 {
		t.Errorf("a refused save wrote %d files", len(port.wrote))
	}
	if !strings.Contains(body, "the file changed on disk") {
		t.Errorf("the panel does not carry the reason back:\n%s", body)
	}
}

func TestDeleteBodyTakesTheRuleOut(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"}, "text": {"x"}}
	if _, err := screen.DeleteBody(LayerLocal, "protocol.yaml", form); err != nil {
		t.Fatalf("DeleteBody: %v", err)
	}
	back, err := rules.Parse(port.wrote[0].Text, "protocol.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if ids := ruleIDs(back); ids != "C2,C3" {
		t.Errorf("the rules after the delete = %s, want C2,C3", ids)
	}
}

func TestReportBodyMakesTheShapeAndPointsTheRuleAtIt(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{
		"token": {"tok"}, "section": {"1"}, "i": {"0"}, "id": {"C3"}, "text": {"続きの節である。"}, "send": {"1"},
		"report_name": {"完了報告"}, "report_where": {"Issue のコメント"},
		"field_name": {"next_turn"}, "field_description": {"次に手番を持つ人"},
	}
	if _, err := screen.ReportBody(LayerLocal, "protocol.yaml", form); err != nil {
		t.Fatalf("ReportBody: %v", err)
	}
	back, err := rules.Parse(port.wrote[0].Text, "protocol.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var added *rules.Report
	for i := range back.Reports {
		if back.Reports[i].Name == "完了報告" {
			added = &back.Reports[i]
		}
	}
	if added == nil {
		t.Fatalf("the new report shape is not there: %+v", back.Reports)
	}
	if got := back.Sections[1].Rules[0].Report; got != "完了報告.next_turn" {
		t.Errorf("the rule points at %q, want 完了報告.next_turn", got)
	}
	if len(added.Fields) != 1 || added.Fields[0].Description != "次に手番を持つ人" {
		t.Errorf("the field = %+v", added.Fields)
	}
}

func TestReportBodyRefusesAFieldWithoutAName(t *testing.T) {
	screen := newScreen(t, newFake())
	if _, err := screen.ReportBody(LayerLocal, "protocol.yaml", url.Values{"section": {"0"}}); err == nil {
		t.Error("a report shape with no name was not refused")
	}
}

func TestPreviewBodyCarriesTheBreak(t *testing.T) {
	port := newFake()
	port.preview = Preview{Section: "## 通信", Break: "C1: text is blank"}
	screen := newScreen(t, port)
	body, err := screen.PreviewBody(LayerLocal, "protocol.yaml", url.Values{"section": {"0"}, "i": {"0"}})
	if err != nil {
		t.Fatalf("PreviewBody: %v", err)
	}
	for _, want := range []string{"C1: text is blank", "## 通信"} {
		if !strings.Contains(body, want) {
			t.Errorf("the preview is missing %q:\n%s", want, body)
		}
	}
}

func TestPanelFormBodyAddsAndRemovesAnOrigin(t *testing.T) {
	screen := newScreen(t, newFake())
	base := url.Values{
		"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"}, "text": {"手番を渡す。"}, "send": {"1"},
		"origin_kind": {"incident"}, "origin_date": {"2026-09-25"}, "origin_refs": {"#6100"}, "origin_note": {"起きたこと"},
	}
	more := cloneValues(base)
	more.Set("add_origin", "1")
	body, err := screen.PanelFormBody(LayerLocal, "protocol.yaml", more)
	if err != nil {
		t.Fatalf("PanelFormBody add: %v", err)
	}
	if got := strings.Count(body, `name="origin_kind"`); got != 2 {
		t.Errorf("the form holds %d origin rows after the add, want 2:\n%s", got, body)
	}

	fewer := cloneValues(base)
	fewer.Set("remove_origin", "0")
	body, err = screen.PanelFormBody(LayerLocal, "protocol.yaml", fewer)
	if err != nil {
		t.Fatalf("PanelFormBody remove: %v", err)
	}
	if got := strings.Count(body, `name="origin_kind"`); got != 0 {
		t.Errorf("the form holds %d origin rows after the remove, want 0:\n%s", got, body)
	}

	ask := cloneValues(base)
	ask.Set("delete_ask", "1")
	body, err = screen.PanelFormBody(LayerLocal, "protocol.yaml", ask)
	if err != nil {
		t.Fatalf("PanelFormBody ask: %v", err)
	}
	if !strings.Contains(body, i18n.T("web.rules.panel.delete_confirm")) {
		t.Errorf("the delete ask does not offer the confirm:\n%s", body)
	}
}

func TestFormOfReadsThePanelForm(t *testing.T) {
	values := url.Values{
		"section": {"2"}, "i": {"3"}, "id": {"C1"}, "text": {"t"}, "why": {"w"},
		"send": {"1"}, "note": {"n"}, "report": {"A.f"}, "tests": {"T-1, T-2"},
		"origin_kind": {"incident", "derived"},
		"origin_date": {"2026-01-01", ""},
		"origin_refs": {"#1", "C1"},
		"origin_note": {"one", "two"},
	}
	edit := formOf(values)
	if edit.Section != 2 || edit.Index != 3 || edit.ID != "C1" || !edit.Send {
		t.Errorf("the form = %+v", edit)
	}
	if edit.Report != "A.f" || edit.Tests != "T-1, T-2" {
		t.Errorf("the form = %+v", edit)
	}
	if len(edit.Origins) != 2 || !edit.Origins[1].Derived() {
		t.Fatalf("the origins = %+v", edit.Origins)
	}
	if edit.Origins[0].Kind != rules.KindIncident || edit.Origins[0].Refs != "#1" {
		t.Errorf("the first origin = %+v", edit.Origins[0])
	}
	// A form with no i at all means a new rule.
	if got := formOf(url.Values{"id": {"C1"}}).Index; got != -1 {
		t.Errorf("Index of a form with no i = %d, want -1", got)
	}
}

func TestSuggestIDRaisesAndStepsPastTakenIDs(t *testing.T) {
	document, err := rules.Parse(sample, "protocol.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// The last rule of the section is C2, so the offer is C3; C3 is in the
	// document, so the free one beside it is C3-2.
	if got := suggestID(document, 0); got != "C3-2" {
		t.Errorf("suggestID = %q, want C3-2", got)
	}
	if got := suggestID(&rules.Document{Version: 1, Sections: []rules.Section{{}}}, 0); got != "NEW" {
		t.Errorf("suggestID of an empty document = %q, want NEW", got)
	}
	if got := bump("P-TURN-1"); got != "P-TURN-2" {
		t.Errorf("bump(P-TURN-1) = %q, want P-TURN-2", got)
	}
	if got := bump("C"); got != "C-2" {
		t.Errorf("bump(C) = %q, want C-2", got)
	}
}

func ruleIDs(doc *rules.Document) string {
	var ids []string
	for _, rule := range doc.Rules() {
		ids = append(ids, rule.ID)
	}
	return strings.Join(ids, ",")
}

func cloneValues(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, list := range values {
		out[key] = append([]string(nil), list...)
	}
	return out
}
