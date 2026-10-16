package rulesview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

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
	docChanges DocChanges
	bump       BumpAnswer
	bumpErr    error
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

func (p *fakePort) LayerRoot(layer string) string {
	if layer == LayerGlobal {
		return "~/.config/hergy"
	}
	return ".hergy"
}

func (p *fakePort) Changes(string) int { return p.changes }

func (p *fakePort) DocChanges(string, string) DocChanges { return p.docChanges }

func (p *fakePort) Bump(string, string) (BumpAnswer, error) {
	if p.bumpErr != nil {
		return BumpAnswer{}, p.bumpErr
	}
	if p.bump.Written {
		// The version the write raised is what the next read sees, the way the
		// app reads the file back: the document is now at the new version and
		// ahead of the launch that read it.
		p.docChanges.Version = p.bump.To
		p.docChanges.Bumped = true
	}
	return p.bump, nil
}

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
	body, err := screen.RulesBody(RulesQuery{Layer: LayerLocal, Filter: FilterAll})
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
	if _, err := screen.RulesBody(RulesQuery{Layer: LayerGlobal, Filter: FilterAll}); err != nil {
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

// TestThePreviewKeepsItsLines: the preview is a document as a member reads it,
// so its line breaks are part of it. The section goes in a pre, which is what
// keeps them (the prototype's "Markdown のまま表示"). Without it the whole
// document reads as one paragraph (hergy-spec's V1).
func TestThePreviewKeepsItsLines(t *testing.T) {
	port := newFake()
	port.preview = Preview{Section: "## 通信\n\n口を開ける前に、相手の様子を見る。"}
	screen := newScreen(t, port)
	body, err := screen.PreviewBody(LayerLocal, "protocol.yaml", url.Values{"section": {"0"}, "i": {"0"}})
	if err != nil {
		t.Fatalf("PreviewBody: %v", err)
	}
	if want := `<div class="render raw"><pre>## 通信` + "\n\n" + `口を開ける前に、相手の様子を見る。</pre></div>`; !strings.Contains(body, want) {
		t.Errorf("the preview does not keep its lines in a pre:\n%s", body)
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

// TestTheRulesScreenHasFourTabs: the band draws the four views of one document,
// the one ?tab= names is marked, and every tab is a link, so the tabs work with
// no script.
func TestTheRulesScreenHasFourTabs(t *testing.T) {
	screen := newScreen(t, newFake())
	body, err := screen.RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabPrompt, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	for _, key := range []string{"web.rules.tab.rules", "web.rules.tab.prompt", "web.rules.tab.changes", "web.rules.tab.yaml"} {
		if !strings.Contains(body, i18n.T(key)) {
			t.Errorf("the tab band is missing %q:\n%s", key, body)
		}
	}
	// The open tab is the marked link; the rules tab is the screen's own URL, so
	// it carries no tab value.
	for _, want := range []string{
		`href="/rules?layer=local&amp;doc=protocol.yaml&amp;tab=prompt" aria-current="true"`,
		`href="/rules?layer=local&amp;doc=protocol.yaml"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the tab band is missing %q:\n%s", want, body)
		}
	}
}

// TestThePromptTabDrawsTheDocumentAndOpensARule: the prompt tab draws the
// document the way a member reads it, and each rule opens the right panel at the
// same place the rules list opens it.
func TestThePromptTabDrawsTheDocumentAndOpensARule(t *testing.T) {
	screen := newScreen(t, newFake())
	body, err := screen.RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabPrompt, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	chars := utf8.RuneCountInString(sampleDoc(t, LayerLocal).Document.Markdown())
	for _, want := range []string{
		i18n.Tf("web.rules.prompt.lede", chars),
		`class="r-h r-h2"`,   // the section heading, at level 2
		`class="r-list ids"`, // the section says ids
		`class="r-rule"`,     // one rule
		`class="gut"`,        // its id in the margin
		`<span class="r-p">`,
		// The rule opens the panel the same way the list does, carrying the
		// document the body shows (view) so a save comes back to it.
		`hx-get="/rules/panel?layer=local&amp;doc=protocol.yaml&amp;view=protocol.yaml&amp;section=0&amp;i=0"`,
		`hx-target="#drawer" hx-swap="outerHTML"`,
		i18n.T("web.rules.prompt.raw"),
		i18n.T("web.rules.prompt.overlay"),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the prompt tab is missing %q:\n%s", want, body)
		}
	}
	// With the overlay off, nothing the prompt leaves out is drawn, and the
	// rules tab's own list is not drawn beside the prompt.
	if strings.Contains(body, `class="unsent"`) {
		t.Errorf("the overlay is off but the prompt tab draws what it leaves out:\n%s", body)
	}
	if strings.Contains(body, `class="rules"`) {
		t.Errorf("the prompt tab draws the rules list as well:\n%s", body)
	}
}

// TestThePromptTabShowsTheMarkdownAsItIs: the raw switch shows the Markdown hergy
// hands over, with its lines kept, and the overlay cannot be turned on while it
// is shown.
func TestThePromptTabShowsTheMarkdownAsItIs(t *testing.T) {
	screen := newScreen(t, newFake())
	body, err := screen.RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabPrompt, Raw: true, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	md := sampleDoc(t, LayerLocal).Document.Markdown()
	if want := `<div class="render raw"><pre>` + template.HTMLEscapeString(md) + `</pre></div>`; !strings.Contains(body, want) {
		t.Errorf("the raw view does not show the Markdown as it is:\n%s", body)
	}
	if strings.Contains(body, `class="r-rule"`) {
		t.Errorf("the raw view also draws the rules:\n%s", body)
	}
	// The Markdown keeps its lines: the heading is followed by a blank line.
	if !strings.Contains(body, "\n\n") {
		t.Errorf("the raw view lost the Markdown's blank lines:\n%s", body)
	}
	// The overlay switch is disabled while the Markdown is shown.
	if !strings.Contains(body, `aria-disabled="true"`) {
		t.Errorf("the overlay switch is not disabled while the Markdown is shown:\n%s", body)
	}
}

// unsentSample is a document whose rule carries everything the prompt leaves out:
// an origin, a reason send_why keeps out, and a note.
const unsentSample = `version: 1
sections:
  - style: bullets
    rules:
      - id: C1
        text: 送る本文。
        why: 送らない理由。
        send_why: false
        note: メモである。
        origin:
          - kind: incident
            date: 2026-09-25
            refs: ["#6100"]
`

// TestThePromptTabShowsWhatThePromptLeavesOut: the overlay draws the pieces the
// prompt leaves out under each rule — where it came from, a reason send_why keeps
// out, and the note — and says when a rule leaves nothing out.
func TestThePromptTabShowsWhatThePromptLeavesOut(t *testing.T) {
	port := newFake()
	port.files["protocol.yaml"] = unsentSample
	body, err := newScreen(t, port).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabPrompt, Overlay: true, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	for _, want := range []string{
		`class="unsent"`,
		i18n.T("web.rules.prompt.unsent.origin"),
		i18n.T("web.rules.prompt.unsent.why"),
		"送らない理由。",
		i18n.T("web.rules.prompt.unsent.note"),
		"メモである。",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the overlay is missing %q:\n%s", want, body)
		}
	}

	// A rule that leaves nothing out says so, and the switch is off by default.
	plain, err := newScreen(t, newFake()).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabPrompt, Overlay: true, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	if !strings.Contains(plain, i18n.T("web.rules.prompt.unsent.none")) {
		t.Errorf("a rule that leaves nothing out is not said so:\n%s", plain)
	}
	off, err := newScreen(t, newFake()).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabPrompt, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	if strings.Contains(off, `class="unsent"`) {
		t.Errorf("the overlay is off but what the prompt leaves out is drawn:\n%s", off)
	}
}

// TestTheChangesTabDrawsWhatMovedAndOffersTheBump: the changes tab draws the same
// rows the home screen draws, and offers to raise the version by the same write
// the home screen's /bump makes. Nothing here puts a change back.
func TestTheChangesTabDrawsWhatMovedAndOffersTheBump(t *testing.T) {
	port := newFake()
	port.docChanges = DocChanges{
		Recorded: true,
		Version:  3,
		Added:    []ChangeRule{{ID: "C7", Section: "通信", Href: "/rules?layer=local&doc=protocol.yaml&rule=C7", Diffs: []ChangePart{{After: "足した。"}}}},
		Removed:  []ChangeRule{{ID: "C8", Diffs: []ChangePart{{Before: "消した。"}}}},
		Changed:  []ChangeRule{{ID: "C1", Fields: []string{"text"}, Diffs: []ChangePart{{Before: "前。", After: "後。"}}}},
	}
	body, err := newScreen(t, port).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabChanges, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	for _, want := range []string{
		`class="ck added"`,
		`class="ck removed"`,
		`class="ck changed"`,
		"<ins>",
		"<del>",
		// An added or changed rule opens in the rules screen.
		`<a class="linkbtn" href="/rules?layer=local&amp;doc=protocol.yaml&amp;rule=C7">`,
		`action="/rules/bump?layer=local"`,
		template.HTMLEscapeString(i18n.T("web.bump.raise")),
		i18n.Tf("web.bump.version", 3, 4),
		`name="doc" value="protocol.yaml"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the changes tab is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "元に戻す") {
		t.Errorf("the changes tab offers to put a change back:\n%s", body)
	}
}

// TestTheChangesTabSaysWhenThereIsNoRecordOrTheVersionWasRaised: a document no
// launch read says so, and a document whose version was already raised offers no
// button and says the version is ahead.
func TestTheChangesTabSaysWhenThereIsNoRecordOrTheVersionWasRaised(t *testing.T) {
	port := newFake()
	port.docChanges = DocChanges{Recorded: true, Version: 3, Bumped: true, Changed: []ChangeRule{{ID: "C1", Diffs: []ChangePart{{Before: "前。", After: "後。"}}}}}
	body, err := newScreen(t, port).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabChanges, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	if !strings.Contains(body, `<p class="saved">`+i18n.Tf("web.home.changes.bumped", 3)) {
		t.Errorf("a version that was already raised is not said:\n%s", body)
	}
	if strings.Contains(body, `action="/rules/bump`) {
		t.Errorf("a version that was already raised still offers the button:\n%s", body)
	}

	port.docChanges = DocChanges{}
	body, err = newScreen(t, port).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabChanges, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	if !strings.Contains(body, noRecordLead(i18n.T("web.home.changes.no_record"))) {
		t.Errorf("a document with no record does not say so:\n%s", body)
	}
}

// noRecordLead is the head of a message up to its first `code` span: the words
// the screen shows before the command it names, so a test can find it whatever
// the inline rule does with the part after it.
func noRecordLead(text string) string {
	lead, _, _ := strings.Cut(text, "`")
	return strings.TrimSpace(lead)
}

// TestTheChangesTabBumpRaisesTheVersion: the form the changes tab writes is the
// write of the home screen's /bump, and the page comes back saying what rose.
func TestTheChangesTabBumpRaisesTheVersion(t *testing.T) {
	port := newFake()
	port.docChanges = DocChanges{Recorded: true, Version: 3, Added: []ChangeRule{{ID: "C7", Diffs: []ChangePart{{After: "足した。"}}}}}
	port.bump = BumpAnswer{Written: true, From: 3, To: 4}
	req := httptest.NewRequest(http.MethodPost, "/rules/bump?layer=local", strings.NewReader("doc=protocol.yaml&token=tok"))
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	view, err := newScreen(t, port).bump(req)
	if err != nil {
		t.Fatalf("bump: %v", err)
	}
	body := string(view.Body)
	if want := i18n.Tf("web.bump.raised", ".hergy/protocol.yaml", 3, 4); !strings.Contains(body, want) {
		t.Errorf("the bump page does not say what rose (%q):\n%s", want, body)
	}
	// The version the write raised was handed to the answer, so the button is
	// gone and the tab shows the version ahead.
	if want := i18n.Tf("web.home.changes.bumped", 4); !strings.Contains(body, template.HTMLEscapeString(want)) {
		t.Errorf("the bumped document is not said:\n%s", body)
	}
}

// TestTheYAMLTabShowsTheFileAndTheChecks: the YAML tab shows the file as it is on
// disk, with a button that copies it, and the break the reader names, or that
// there is none.
func TestTheYAMLTabShowsTheFileAndTheChecks(t *testing.T) {
	port := newFake()
	port.preview = Preview{Break: "protocol.yaml: C2: text is blank"}
	body, err := newScreen(t, port).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabYAML, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	for _, want := range []string{
		`<pre id="yaml-src">` + template.HTMLEscapeString(sample) + `</pre>`,
		`data-copy="#yaml-src"`,
		`data-copied="` + i18n.T("web.copy.done") + `"`,
		`<div class="code check"><pre>protocol.yaml: C2: text is blank</pre></div>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the YAML tab is missing %q:\n%s", want, body)
		}
	}

	// A document the reader found sound says so instead of drawing a break.
	port.preview = Preview{}
	body, err = newScreen(t, port).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabYAML, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	if !strings.Contains(body, i18n.T("web.rules.yaml.ok")) {
		t.Errorf("a sound document does not say so:\n%s", body)
	}
	if strings.Contains(body, `class="code check"`) {
		t.Errorf("a sound document draws a break:\n%s", body)
	}
}

// TestTheSearchBoxKeepsOnlyWhatItSays: the GET argument q narrows the list the
// way the prototype's passFilter does — an id, the text, the reason or the note
// that holds what was typed, without regard to case — and works beside the
// filter chip, and the screen says so when the two keep nothing.
func TestTheSearchBoxKeepsOnlyWhatItSays(t *testing.T) {
	cases := []struct {
		name  string
		query string
		keep  []string
		drop  []string
	}{
		{"by id", "C2", []string{">C2<"}, []string{">C1<", ">C3<"}},
		{"by text", "手番", []string{">C1<"}, []string{">C2<", ">C3<"}},
		{"by reason", "完了を記録する", []string{">C1<"}, []string{">C2<", ">C3<"}},
		{"by note", "保守のメモ", []string{">C1<"}, []string{">C2<", ">C3<"}},
		{"case aside", "c2", []string{">C2<"}, []string{">C1<", ">C3<"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := newScreen(t, newFake()).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Filter: FilterAll, Query: tc.query})
			if err != nil {
				t.Fatalf("RulesBody: %v", err)
			}
			for _, want := range tc.keep {
				if !strings.Contains(body, want) {
					t.Errorf("the search %q dropped %q:\n%s", tc.query, want, body)
				}
			}
			for _, unwanted := range tc.drop {
				if strings.Contains(body, unwanted) {
					t.Errorf("the search %q kept %q:\n%s", tc.query, unwanted, body)
				}
			}
		})
	}

	// The chip and the box work together: the no-origin chip keeps C2 and C3,
	// and the search narrows that to C3; C1 carries an origin, so it is gone
	// either way.
	both, err := newScreen(t, newFake()).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Filter: FilterNoOrigin, Query: "C3"})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	if !strings.Contains(both, ">C3<") || strings.Contains(both, ">C2<") || strings.Contains(both, ">C1<") {
		t.Errorf("the chip and the search do not work together:\n%s", both)
	}

	// A search that keeps nothing says so.
	none, err := newScreen(t, newFake()).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Filter: FilterAll, Query: "存在しない語"})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	if !strings.Contains(none, i18n.T("web.rules.filter.empty")) {
		t.Errorf("a search that keeps nothing does not say so:\n%s", none)
	}
	if strings.Contains(none, ">C1<") {
		t.Errorf("a search that keeps nothing still shows a rule:\n%s", none)
	}
}

// TestTheSearchFormAndTheListFragment: the search box is a plain GET form, so
// Enter narrows the list with no script, and it holds what was typed with the
// hidden layer, document and filter; the list it swaps is one #rules-list
// element, and /rules/list answers that element alone, with no frame.
func TestTheSearchFormAndTheListFragment(t *testing.T) {
	screen := newScreen(t, newFake())
	body, err := screen.RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Filter: FilterAll, Query: "C2"})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	for _, want := range []string{
		`<form class="search-form" method="get" action="/rules">`,
		`<input type="hidden" name="layer" value="local">`,
		`<input type="hidden" name="doc" value="protocol.yaml">`,
		`<input type="hidden" name="filter" value="all">`,
		`type="search" name="q" value="C2"`,
		`hx-get="/rules/list?layer=local&amp;doc=protocol.yaml"`,
		`hx-target="#rules-list" hx-swap="outerHTML"`,
		i18n.T("web.rules.search.placeholder"),
		i18n.T("web.rules.search"),
		`<div id="rules-list">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the search form is missing %q:\n%s", want, body)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/rules/list?layer=local&doc=protocol.yaml&q=C2", nil)
	view, err := screen.rulesList(req)
	if err != nil {
		t.Fatalf("rulesList: %v", err)
	}
	if !view.Bare {
		t.Error("the list fragment is not Bare: the frame would go around it")
	}
	fragment := string(view.Body)
	if !strings.Contains(fragment, `<div id="rules-list">`) || !strings.Contains(fragment, ">C2<") {
		t.Errorf("the fragment does not hold the list:\n%s", fragment)
	}
	if strings.Contains(fragment, `class="toolbar"`) || strings.Contains(fragment, `<div class="wrap">`) {
		t.Errorf("the fragment carries the frame's chrome:\n%s", fragment)
	}
}

// TestTheSearchBoxFiltersByWhatTheBoxHolds: a page opened with a query in its
// address sends the word the box holds, not the word the page was opened with.
// It went wrong when the box's address carried the page's own q as well: the
// server reads the first value of a name, so the box went on filtering by the
// word the page was opened with, and neither another word nor an empty one
// changed the list (hergy-spec's finding).
func TestTheSearchBoxFiltersByWhatTheBoxHolds(t *testing.T) {
	screen := newScreen(t, newFake())
	doc := sampleDoc(t, LayerLocal)
	body, err := screen.RulesBody(RulesQuery{Layer: LayerLocal, Path: doc.Path, Filter: FilterAll, Query: "C2"})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	// The box's address carries the layer and the document and nothing else.
	href := searchHref(doc)
	want := `hx-get="` + strings.ReplaceAll(href, "&", "&amp;") + `"`
	if !strings.Contains(body, want) {
		t.Errorf("the search box does not send %s:\n%s", want, body)
	}
	if strings.Contains(href, "q=") || strings.Contains(href, "filter=") {
		t.Fatalf("the box's address carries the page's own search: %q", href)
	}
	// What HTMX sends when a person types: the box's address, and then the
	// form's fields, the box's own word among them.
	send := func(q string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet,
			href+"&layer=local&doc="+url.QueryEscape(doc.Path)+"&filter=all&q="+url.QueryEscape(q), nil)
		view, err := screen.rulesList(req)
		if err != nil {
			t.Fatalf("rulesList: %v", err)
		}
		return string(view.Body)
	}
	cleared := send("")
	for _, id := range []string{">C1<", ">C2<", ">C3<"} {
		if !strings.Contains(cleared, id) {
			t.Errorf("clearing the box left %s out of the list:\n%s", id, cleared)
		}
	}
	other := send("続き")
	if !strings.Contains(other, ">C3<") || strings.Contains(other, ">C1<") {
		t.Errorf("typing another word did not filter by it:\n%s", other)
	}
}

// TestThePanelOffersThePromptToCopy: the panel fills the copyable prompt with
// the rule's id, the file's display path, its section, its text, its reason, the
// layer's root, and the origin shape to answer in, and offers the Copy button; a
// rule with no reason leaves the reason line out of the prompt.
func TestThePanelOffersThePromptToCopy(t *testing.T) {
	panel, err := newScreen(t, newFake()).PanelBody(LayerLocal, "protocol.yaml", url.Values{"sec": {"0"}, "i": {"0"}, "token": {"tok"}})
	if err != nil {
		t.Fatalf("PanelBody: %v", err)
	}
	for _, want := range []string{
		i18n.T("web.rules.ask_ai"),
		i18n.T("web.rules.ask_ai.copy"),
		`data-copy="#ai-prompt"`,
		`data-copied="` + i18n.T("web.copy.done") + `"`,
		`<pre id="ai-prompt" hidden>`,
		"C1",                   // the rule's id
		".hergy/protocol.yaml", // the file's display path
		"通信",                   // the section's heading
		"手番を渡す。",               // the text
		"完了を記録する主体がいなくなる。", // the reason
		"origin:", // the shape to answer in
		"- kind:",
		"YYYY-MM-DD",
	} {
		if !strings.Contains(panel, want) {
			t.Errorf("the panel's prompt is missing %q:\n%s", want, panel)
		}
	}
	if want := i18n.Tf("web.rules.ask_ai.prompt.why", "完了を記録する主体がいなくなる。"); !strings.Contains(panel, want) {
		t.Errorf("a rule with a reason does not carry the reason line %q:\n%s", want, panel)
	}

	// C2 has no reason: the reason line is left out of the prompt.
	bare, err := newScreen(t, newFake()).PanelBody(LayerLocal, "protocol.yaml", url.Values{"sec": {"0"}, "i": {"1"}, "token": {"tok"}})
	if err != nil {
		t.Fatalf("PanelBody: %v", err)
	}
	if reasonLead := strings.Split(i18n.T("web.rules.ask_ai.prompt.why"), "%")[0]; strings.Contains(bare, reasonLead) {
		t.Errorf("a rule with no reason still has the reason line %q:\n%s", reasonLead, bare)
	}
}

// TestASaveFromARelatedRuleKeepsTheBodyDocument: a rule opened from a related
// rule of another document saves into that document, but the body stays on the
// document the person was reading (view), and the left list marks that one, so
// the URL's ?doc= and the body do not part ways.
func TestASaveFromARelatedRuleKeepsTheBodyDocument(t *testing.T) {
	port := newFake()
	screen := newScreen(t, port)
	form := url.Values{
		"token": {"tok"}, "layer": {LayerLocal}, "doc": {"roles/lead.yaml"}, "view": {"protocol.yaml"},
		"section": {"0"}, "i": {"0"}, "id": {"L1"}, "text": {"直した指示。"}, "send": {"1"},
		"hash": {hashOf(roleSample)}, "orig_id": {"L1"},
	}
	answer, err := screen.SaveBody(LayerLocal, "roles/lead.yaml", form)
	if err != nil {
		t.Fatalf("SaveBody: %v", err)
	}
	if answer.Doc != "protocol.yaml" {
		t.Errorf("the left list marks %q, want the document the body shows (protocol.yaml)", answer.Doc)
	}
	if !strings.Contains(answer.Body, "TEAM PROTOCOL") {
		t.Errorf("the body left the document the person was reading:\n%s", answer.Body)
	}
	if strings.Contains(answer.Body, ">L1<") {
		t.Errorf("the body shows the document the panel edits, not the one being read:\n%s", answer.Body)
	}
	if len(port.wrote) != 1 || port.wrote[0].Path != "roles/lead.yaml" {
		t.Fatalf("the write = %+v, want roles/lead.yaml", port.wrote)
	}
}

// TestTheDocumentLineCountsTheChanges: the document's own line says how many
// rules moved in it since the last launch, and marks the version dirty.
func TestTheDocumentLineCountsTheChanges(t *testing.T) {
	port := newFake()
	port.docChanges = DocChanges{
		Recorded: true,
		Version:  3,
		Added:    []ChangeRule{{ID: "C7"}},
		Removed:  []ChangeRule{{ID: "C8"}},
		Changed:  []ChangeRule{{ID: "C1"}},
	}
	body, err := newScreen(t, port).RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Tab: TabRules, Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}
	if !strings.Contains(body, `<span class="rev dirty">`) {
		t.Errorf("the document line is not marked dirty:\n%s", body)
	}
	if want := i18n.Tf("web.rules.doc.since", 3); !strings.Contains(body, want) {
		t.Errorf("the document line does not count the changes (%q):\n%s", want, body)
	}
}
