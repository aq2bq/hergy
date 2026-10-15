package rulesview

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
)

// relatedSample is one document whose rules point at one another: A1's text
// names B1, E1 was derived from A1, and C1's text holds the longer id C12, so a
// shorter id that only stands inside a longer one is not read as a reference.
const relatedSample = `version: 1
title: TEAM PROTOCOL
sections:
  - rules:
      - id: A1
        text: B1 に従う。
      - id: B1
        text: 返事を待つ。
      - id: E1
        text: 派生した規則。
        origin:
          - kind: derived
            refs: [A1]
      - id: C1
        text: C12 という別の規則を指す。
      - id: C12
        text: 境界の確認。
      - id: Z1
        text: 誰も指さない。
`

// relatedPort is a layer whose documents point at one another: protocol.yaml
// (relatedSample) and the role file of one member that reads it, so a reference
// reaches across the two.
func relatedPort() *fakePort {
	port := newFake()
	port.files["protocol.yaml"] = relatedSample
	port.files["roles/lead.yaml"] = "version: 1\nsections:\n  - rules:\n      - id: R1\n        text: A1 の写し。\n"
	port.files["roles/impl.yaml"] = "version: 1\nsections:\n  - rules:\n      - id: I1\n        text: 実装する。\n"
	return port
}

// panelFor opens the panel at one rule of the first section of one document,
// with the document the body shows when view is not empty.
func panelFor(t *testing.T, screen *Screen, doc string, index int, view string) string {
	t.Helper()
	values := url.Values{
		"sec": {"0"}, "i": {strconv.Itoa(index)}, "token": {"tok"},
	}
	if view != "" {
		values.Set("view", view)
	}
	body, err := screen.PanelBody(LayerLocal, doc, values)
	if err != nil {
		t.Fatalf("PanelBody(%s, 0, %d): %v", doc, index, err)
	}
	return body
}

// TestThePanelListsTheRelatedRules: the panel lists what the open rule's text
// points at, what points at it, and what was derived from it, each as its id and
// a link that opens that rule in the panel; the open rule is not listed as
// related to itself, and a rule with nothing related shows no such list.
func TestThePanelListsTheRelatedRules(t *testing.T) {
	screen := newScreen(t, relatedPort())
	// A1: its text names B1, R1 (the role rule the same member reads) names it,
	// and E1 was derived from it — all three lists.
	panel := panelFor(t, screen, "protocol.yaml", 0, "")
	for _, want := range []string{
		i18n.T("web.rules.related"),
		i18n.T("web.rules.related.out") + ": ",
		i18n.T("web.rules.related.back") + ": ",
		i18n.T("web.rules.related.kids") + ": ",
		">B1</a>", // what A1's text points at
		">R1</a>", // what points at A1
		">E1</a>", // what was derived from A1
		// A related rule of another document opens there, carrying the body's
		// document (view) so a save comes back to it.
		`hx-get="/rules/panel?layer=local&amp;doc=roles%2Flead.yaml&amp;view=protocol.yaml&amp;section=0&amp;i=0"`,
		// The rule that stands elsewhere says which document it is.
		`<span class="fine">lead.yaml</span>`,
	} {
		if !strings.Contains(panel, want) {
			t.Errorf("the panel is missing %q:\n%s", want, panel)
		}
	}
	if strings.Contains(panel, ">A1</a>") {
		t.Errorf("the panel lists the open rule as related to itself:\n%s", panel)
	}

	// Z1 points at nothing and nothing points at it: no list stands.
	alone := panelFor(t, screen, "protocol.yaml", 5, "")
	if strings.Contains(alone, i18n.T("web.rules.related")) {
		t.Errorf("a rule with nothing related still shows the list:\n%s", alone)
	}
}

// TestThePanelDoesNotReadAShorterIDOutOfALongerOne: C1's text holds C12, which
// is another rule, so the panel lists C12 and does not read C1 out of it.
func TestThePanelDoesNotReadAShorterIDOutOfALongerOne(t *testing.T) {
	screen := newScreen(t, relatedPort())
	panel := panelFor(t, screen, "protocol.yaml", 3, "") // C1
	if !strings.Contains(panel, ">C12</a>") {
		t.Errorf("the panel does not list the rule C1's text points at:\n%s", panel)
	}
	if strings.Contains(panel, ">C1</a>") {
		t.Errorf("the panel read C1 out of the text C12:\n%s", panel)
	}
}

// TestThePanelCarriesTheViewDocument: the panel carries the document the body
// shows as a hidden field, so a save from a rule of another document comes back
// to the document the person was reading.
func TestThePanelCarriesTheViewDocument(t *testing.T) {
	screen := newScreen(t, relatedPort())
	panel := panelFor(t, screen, "roles/lead.yaml", 0, "protocol.yaml")
	if value, ok := hiddenValue(panel, "view"); !ok || value != "protocol.yaml" {
		t.Errorf("the panel's view field = %q (ok %v), want protocol.yaml", value, ok)
	}
}

// TestAskAIFillsTheRuleAndTheRootAndLeavesOutAnEmptyReason: the prompt carries
// the rule's own words, the section, the file's display path and the layer's
// root, and drops the reason line when the rule has no reason.
func TestAskAIFillsTheRuleAndTheRootAndLeavesOutAnEmptyReason(t *testing.T) {
	doc := sampleDoc(t, LayerLocal)
	rule := doc.Document.Sections[0].Rules[0] // C1, with a reason
	prompt := askAI("~/.config/hergy", doc, 0, NewEditRule(0, 0, rule))
	for _, want := range []string{
		"C1",
		".hergy/protocol.yaml",
		"通信",
		"手番を渡す。",
		"完了を記録する主体がいなくなる。",
		"~/.config/hergy",
		"origin:",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt is missing %q:\n%s", want, prompt)
		}
	}
	if want := i18n.Tf("web.rules.ask_ai.prompt.why", rule.Why); !strings.Contains(prompt, want) {
		t.Errorf("the prompt is missing the reason line %q:\n%s", want, prompt)
	}

	// C2 has no reason, so the reason line goes.
	bare := askAI(".hergy", doc, 0, NewEditRule(0, 1, doc.Document.Sections[0].Rules[1]))
	reasonLead := strings.Split(i18n.T("web.rules.ask_ai.prompt.why"), "%")[0]
	if strings.Contains(bare, reasonLead) {
		t.Errorf("a rule with no reason still has the reason line %q:\n%s", reasonLead, bare)
	}
}

// TestMatchIDsKeepsTheBoundary: an id that only stands inside a longer one is
// not a match, so a rule C1 is not read out of the text C12 (試作 idRe の
// lookaround, looked at by hand because Go's regexp has none).
func TestMatchIDsKeepsTheBoundary(t *testing.T) {
	ids := []string{"C12", "C1", "C"}
	cases := []struct {
		text string
		want string
	}{
		{"C12 を指す。", "C12"},
		{"C1 を指す。", "C1"},
		{"C12 と C1 を指す。", "C12,C1"},
		{"AC1 は当たらない。", ""},
		{"C1x は当たらない。", ""},
		{"C-1 は当たらない。", ""},
		{"C1 番は当たる。", "C1"},
	}
	for _, tc := range cases {
		if got := strings.Join(matchIDs(tc.text, ids), ","); got != tc.want {
			t.Errorf("matchIDs(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}
