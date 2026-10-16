package rulesview

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// clickSample is a document whose first section holds a heading and no rules at
// all: the shape that hid the click bug (spec/22). A row asked for
// section=1&i=0, the panel read sec (0) — and the place it looked at was the
// first section, which has no rule there.
const clickSample = `version: 1
title: TEAM PROTOCOL
sections:
  - heading: 進め方
    style: ids
  - heading: 境界
    style: ids
    rules:
      - id: B1
        text: 触らない。
      - id: B2
        text: 確かめる。
  - rules:
      - id: T1
        text: 続きの節である。
`

// TestTheURLARowDrawsIsTheURLThePanelReads presses each row the way a browser
// does: it takes the hx-get the list drew for that row and hands that URL, as
// it is, to the handler the screen answers /rules/panel with. The panel that
// comes back has to be the row's own rule — a second spelling of the section
// number on either side of that URL is what broke pressing a row (r-06a1).
func TestTheURLARowDrawsIsTheURLThePanelReads(t *testing.T) {
	port := newFake()
	port.files["protocol.yaml"] = clickSample
	screen := newScreen(t, port)

	body, err := screen.RulesBody(RulesQuery{Layer: LayerLocal, Path: "protocol.yaml", Filter: FilterAll})
	if err != nil {
		t.Fatalf("RulesBody: %v", err)
	}

	// The first section has no row; the rows of the second and the third do.
	for _, row := range []struct{ id, section, index string }{
		{"B1", "1", "0"},
		{"B2", "1", "1"},
		{"T1", "2", "0"},
	} {
		t.Run(row.id, func(t *testing.T) {
			asked := rowURL(t, body, row.id)
			handler := routeHandler(t, screen, asked)
			answer, err := handler(httptest.NewRequest(http.MethodGet, asked, nil))
			if err != nil {
				t.Fatalf("the panel for %s: %v", row.id, err)
			}
			panel := string(answer.Body)
			for _, want := range []string{
				`name="id" value="` + row.id + `"`,
				`name="section" value="` + row.section + `"`,
				`name="i" value="` + row.index + `"`,
			} {
				if !strings.Contains(panel, want) {
					t.Errorf("pressing %s answered a panel without %s:\n%s", row.id, want, panel)
				}
			}
		})
	}
}

// TestAPanelThatCannotOpenSaysSo: htmx keeps a failed answer out of the page,
// so a panel that cannot open the rule it was asked for answers with the reason
// in the drawer itself — 200, one line, and the drawer still a drawer, so the
// next row a person presses still finds somewhere to open (r-06a1).
func TestAPanelThatCannotOpenSaysSo(t *testing.T) {
	port := newFake()
	port.files["protocol.yaml"] = clickSample
	screen := newScreen(t, port)

	asked := "/rules/panel?layer=local&doc=protocol.yaml&section=9&i=0"
	answer, err := routeHandler(t, screen, asked)(httptest.NewRequest(http.MethodGet, asked, nil))
	if err != nil {
		t.Fatalf("a place that is not there: %v", err)
	}
	if answer.Status != 0 {
		t.Errorf("the answer is not 200: status %d", answer.Status)
	}
	if !answer.Bare {
		t.Error("the answer carries the frame: htmx swaps a fragment into the drawer")
	}
	panel := string(answer.Body)
	if !strings.Contains(panel, i18n.Tf("web.rules.panel.failed", "")) {
		t.Errorf("the panel does not say it could not open:\n%s", panel)
	}
	if !strings.Contains(panel, `id="`+web.DrawerID+`"`) {
		t.Errorf("the answer is not the drawer, so the next row has nowhere to open:\n%s", panel)
	}
}

// rowURL is the URL the row of one rule asks for, as the list drew it: the
// hx-get of the button that stands just before the rule's id.
func rowURL(t *testing.T, body, id string) string {
	t.Helper()
	at := strings.Index(body, `<span class="rid mono">`+id+`</span>`)
	if at < 0 {
		t.Fatalf("the list drew no row for %s", id)
	}
	before := body[:at]
	open := strings.LastIndex(before, `hx-get="`)
	if open < 0 {
		t.Fatalf("the row of %s asks for nothing", id)
	}
	rest := before[open+len(`hx-get="`):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("the row of %s has a URL that does not close: %s", id, rest)
	}
	return html.UnescapeString(rest[:end])
}

// routeHandler is how the server answers a URL a screen drew: the handler of
// the route whose path it names.
func routeHandler(t *testing.T, screen *Screen, raw string) func(*http.Request) (web.View, error) {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("%s is not a URL: %v", raw, err)
	}
	for _, route := range screen.Routes() {
		if route.Path == parsed.Path {
			return route.Handler
		}
	}
	t.Fatalf("the screen answers nothing on %s", parsed.Path)
	return nil
}
