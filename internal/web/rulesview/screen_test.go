package rulesview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	changes    int
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
					{Name: "lead", Files: []File{{Path: "protocol.yaml", Display: ".hergy/protocol.yaml", GroupLabel: "COMMON"}, {Path: "roles/lead.yaml", Display: ".hergy/roles/lead.yaml", Role: true, GroupLabel: "ROLE"}}},
					{Name: "impl", Files: []File{{Path: "protocol.yaml", Display: ".hergy/protocol.yaml", GroupLabel: "COMMON"}, {Path: "roles/impl.yaml", Display: ".hergy/roles/impl.yaml", Role: true, GroupLabel: "ROLE"}}},
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

func (p *fakePort) Changes(string) int { return p.changes }

func newScreen(t *testing.T, port Port) *Screen {
	t.Helper()
	screen, err := New(port)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return screen
}

func TestRulesBodyShowsTheDocumentAndItsCounts(t *testing.T) {
	screen := newScreen(t, newFake())
	body, err := screen.RulesBody(LayerLocal, "", FilterAll)
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	for _, want := range []string{
		"COMMON",               // the group the file falls into, as the app names it
		".hergy/protocol.yaml", // the file as a person writes it
		"TEAM PROTOCOL",        // the first document, shown by default
		// The panel links carry the layer and the document.
		"/rules/panel?layer=local&amp;doc=protocol.yaml",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the rules body is missing %q:\n%s", want, body)
		}
	}
	// The files of the layer stand in the frame's left list now, not in the
	// screen's own body: the rules screen shows one document.
	if strings.Contains(body, "roles%2Flead.yaml") {
		t.Errorf("the rules body still lists the other files:\n%s", body)
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
	// The closed panel is the drawer element itself, hidden, so a fragment can
	// put the frame back the way it was.
	if !strings.Contains(body, `id="drawer"`) || !strings.Contains(body, "hidden") {
		t.Errorf("the closed panel is not the hidden drawer: %s", body)
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
		"send": {"1"}, "why": {"理由。"}, "hash": {hashOf(sample)}, "orig_id": {"C2"},
	}
	answer, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	body := answer.Body
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
		"hash": {hashOf(sample)},
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
	form := url.Values{"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"}, "text": {"x"}, "send": {"1"},
		"hash": {hashOf(sample)}, "orig_id": {"C1"}}
	answer, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if len(port.wrote) != 0 {
		t.Errorf("a refused save wrote %d files", len(port.wrote))
	}
	if !strings.Contains(answer.Panel, "the file changed on disk") {
		t.Errorf("the panel does not carry the reason back:\n%s", answer.Panel)
	}
	if answer.Body == "" {
		t.Error("a refused save does not show the rules screen beside the panel")
	}
}

func TestDeleteBodyTakesTheRuleOut(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"}, "text": {"x"},
		"hash": {hashOf(sample)}, "orig_id": {"C1"}}
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
		"hash": {hashOf(sample)}, "orig_id": {"C3"},
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

// TestSaveBodyRefusesAWriteWithNoHash: the promise "the file has not changed"
// cannot be kept without the version that was read, so a form that carries none
// is refused rather than saved on the newest read (K1).
func TestSaveBodyRefusesAWriteWithNoHash(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{
		"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"}, "text": {"x"}, "send": {"1"},
	}
	answer, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if len(port.wrote) != 0 {
		t.Errorf("a write with no hash wrote %d files", len(port.wrote))
	}
	if !strings.Contains(answer.Panel, i18n.T("web.rules.error.no_hash")) {
		t.Errorf("the panel does not say why:\n%s", answer.Panel)
	}
}

// TestSaveBodyRefusesWhenThePlaceIsNotTheRuleThatWasOpened: the edit points by
// place, so a rule added before it (by an AI through the hergy-rules skill) would
// make the edit land on another rule. The id the panel was drawn for is what
// stops that (K1).
func TestSaveBodyRefusesWhenThePlaceIsNotTheRuleThatWasOpened(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{
		"token": {"tok"}, "section": {"0"}, "i": {"1"}, "id": {"C9"}, "text": {"x"}, "send": {"1"},
		"hash": {hashOf(sample)}, "orig_id": {"C9"},
	}
	answer, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if len(port.wrote) != 0 {
		t.Errorf("a write at a place that moved wrote %d files", len(port.wrote))
	}
	if !strings.Contains(answer.Panel, i18n.T("web.rules.error.moved")) {
		t.Errorf("the panel does not say why:\n%s", answer.Panel)
	}
}

// TestPanelFormCarriesTheHashItCameWith: drawing the panel again (an origin
// added, the delete ask) must not refresh the hash from disk, or a file changed
// under the screen would pass the check (K1).
func TestPanelFormCarriesTheHashItCameWith(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{
		"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"}, "text": {"手番を渡す。"}, "send": {"1"},
		"hash": {"the-hash-the-panel-was-drawn-with"}, "orig_id": {"C1"},
		"origin_kind": {"incident"}, "origin_date": {"2026-09-25"}, "origin_refs": {"#6100"}, "origin_note": {"x"},
		"add_origin": {"1"},
	}
	body, err := screen.PanelFormBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("PanelFormBody: %v", err)
	}
	if !strings.Contains(body, `name="hash" value="the-hash-the-panel-was-drawn-with"`) {
		t.Errorf("the redrawn panel lost the hash it came with:\n%s", body)
	}
	if !strings.Contains(body, `name="orig_id" value="C1"`) {
		t.Errorf("the redrawn panel lost the id it came with:\n%s", body)
	}
}

// TestARedrawDoesNotLetAChangedFileThrough is the chain K1 describes: the panel
// is opened, the file changes under it (an AI through the hergy-rules skill),
// the person adds an origin, and the save is refused. The hash the redraw carries
// is the one the panel was first drawn with, not the new file's.
func TestARedrawDoesNotLetAChangedFileThrough(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	first := hashOf(sample)
	form := url.Values{
		"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"}, "text": {"手番を渡す。"}, "send": {"1"},
		"hash": {first}, "orig_id": {"C1"},
		"origin_kind": {"incident"}, "origin_date": {"2026-09-25"}, "origin_refs": {"#6100"}, "origin_note": {"x"},
		"add_origin": {"1"},
	}
	body, err := screen.PanelFormBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("PanelFormBody: %v", err)
	}
	hash, ok := hiddenValue(body, "hash")
	if !ok || hash != first {
		t.Fatalf("the redrawn panel carries hash %q, want the one it was drawn with", hash)
	}

	// The file changes under the screen.
	port.files["protocol.yaml"] = sample + "\n# changed by someone else\n"
	form.Set("hash", hash)
	answer, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if len(port.wrote) != 0 {
		t.Errorf("a save over a file changed under the screen wrote %d files", len(port.wrote))
	}
	if !strings.Contains(answer.Panel, "the file changed on disk") {
		t.Errorf("the panel does not carry the refusal back:\n%s", answer.Panel)
	}
}

// hiddenValue is the value of one hidden field of a rendered panel.
func hiddenValue(body, name string) (string, bool) {
	marker := `name="` + name + `" value="`
	_, after, ok := strings.Cut(body, marker)
	if !ok {
		return "", false
	}
	rest := after
	before, _, ok := strings.Cut(rest, "\"")
	if !ok {
		return "", false
	}
	return before, true
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

func TestSaveBodyRefusesARuleTheReaderWouldBreak(t *testing.T) {
	port := newFake()
	port.preview = Preview{Break: "protocol.yaml: rule id C2 is already in protocol.yaml"}
	screen := newScreen(t, port)
	form := url.Values{
		"token": {"tok"}, "section": {"0"}, "i": {"1"}, "id": {"C2"},
		"text": {"x"}, "send": {"1"}, "hash": {hashOf(sample)}, "orig_id": {"C2"},
	}
	answer, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if len(port.wrote) != 0 {
		t.Errorf("a broken rule was written: %+v", port.wrote)
	}
	if !strings.Contains(answer.Panel, "is already in") {
		t.Errorf("the panel does not carry the break back:\n%s", answer.Panel)
	}
}

func TestSaveBodyCarriesTheHashThePanelWasDrawnWith(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	// The hash in the form is the one the panel was drawn with. A file that
	// changed since then is refused, even though this request read it fresh.
	form := url.Values{
		"token": {"tok"}, "section": {"0"}, "i": {"0"}, "id": {"C1"},
		"text": {"x"}, "send": {"1"}, "hash": {hashOf("not what is on disk")}, "orig_id": {"C1"},
	}
	answer, err := screen.SaveBody(LayerLocal, "protocol.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if len(port.wrote) != 0 {
		t.Errorf("a save with a stale hash wrote %d files", len(port.wrote))
	}
	if !strings.Contains(answer.Panel, "the file changed on disk") {
		t.Errorf("the panel does not carry the refusal back:\n%s", answer.Panel)
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

func TestPanelForOpensARuleByItsID(t *testing.T) {
	screen := newScreen(t, newFake())
	panel, ok, err := screen.PanelFor(LayerLocal, "protocol.yaml", "C2", "tok")
	if err != nil {
		t.Fatalf("PanelFor: %v", err)
	}
	if !ok || !strings.Contains(panel, "C2") || !strings.Contains(panel, "返事を待つ。") {
		t.Errorf("PanelFor did not open C2 (ok=%v):\n%s", ok, panel)
	}
	if _, ok, err := screen.PanelFor(LayerLocal, "protocol.yaml", "NOPE", "tok"); err != nil || ok {
		t.Errorf("PanelFor of a rule that is not there = ok %v, err %v, want false, nil", ok, err)
	}
	if _, ok, err := screen.PanelFor(LayerLocal, "no/such.yaml", "C1", "tok"); err != nil || ok {
		t.Errorf("PanelFor of a document that is not there = ok %v, err %v, want false, nil", ok, err)
	}
}

// TestShowOpensTheRuleNamedByTheLink: /rules?doc=..&rule=<id> is the URL another
// screen links to, and the rule comes open in the right panel (spec/10 の 7 節).
func TestShowOpensTheRuleNamedByTheLink(t *testing.T) {
	screen := newScreen(t, newFake())
	req := httptest.NewRequest(http.MethodGet, "/rules?layer=local&doc=protocol.yaml&rule=C2", nil)
	view, err := screen.show(req)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if view.Bare {
		t.Error("the page with a rule open is a fragment, not a page")
	}
	if !strings.Contains(string(view.Drawer), "C2") {
		t.Errorf("the drawer does not carry the rule the link names:\n%s", view.Drawer)
	}
	if !strings.Contains(string(view.Body), "C2") {
		t.Errorf("the body is not the rules screen:\n%s", view.Body)
	}
	// A link with no rule leaves the drawer to the frame.
	plain := httptest.NewRequest(http.MethodGet, "/rules?layer=local&doc=protocol.yaml", nil)
	if view, err := screen.show(plain); err != nil || view.Drawer != "" {
		t.Errorf("a link with no rule = drawer %q, err %v", view.Drawer, err)
	}
	// A link whose rule is not there still shows the document.
	missing := httptest.NewRequest(http.MethodGet, "/rules?layer=local&doc=protocol.yaml&rule=NOPE", nil)
	if view, err := screen.show(missing); err != nil || view.Drawer != "" {
		t.Errorf("a link with an unknown rule = drawer %q, err %v", view.Drawer, err)
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
