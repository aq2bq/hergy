package homeview

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// esc is a locale string the way the page shows it: the screen escapes the text
// it writes, so a comparison has to escape too.
func esc(text string) string { return template.HTMLEscapeString(text) }

// bumpedDoc is one document that moved since the last launch, the way the app
// hands it to the screen.
func bumpedDoc() Doc {
	return Doc{
		Path:       ".hergy/teams/demo/protocol.yaml",
		File:       "/proj/.hergy/teams/demo/protocol.yaml",
		Layer:      web.LayerLocal,
		Rules:      8,
		Added:      []string{"C9"},
		Version:    2,
		Definition: "demo",
	}
}

// newBump builds the bump screen for a port a test fills in.
func newBump(t *testing.T, port Port) web.Screen {
	t.Helper()
	screen, err := NewBump(port)
	if err != nil {
		t.Fatal(err)
	}
	return screen
}

// writeHandler is the handler the screen answers one path with, so a test can
// post a form the way the server hands it over.
func writeHandler(t *testing.T, screen web.Screen, path string) func(*http.Request) (web.View, error) {
	t.Helper()
	for _, route := range screen.Routes() {
		if route.Path == path {
			return route.Handler
		}
	}
	t.Fatalf("the screen answers no %s", path)
	return nil
}

// TestTheBumpScreenListsWhatMovedAndWhatABumpWrites: the screen names each
// document that moved, the version it holds and the one a bump writes, and the
// form that writes it carries the document's path on disk and this run's token.
func TestTheBumpScreenListsWhatMovedAndWhatABumpWrites(t *testing.T) {
	port := fakePort{home: Home{Launched: true, Docs: []Doc{bumpedDoc()}}}
	body := render(t, newBump(t, port), "/bump?layer=local")

	mustContain(t, "the bump screen", body,
		".hergy/teams/demo/protocol.yaml",
		esc(i18n.Tf("web.bump.version", 2, 3)),
		esc(i18n.T("web.bump.raise")),
		`action="/bump/save?layer=local"`,
		`name="token"`,
		`name="doc" value="/proj/.hergy/teams/demo/protocol.yaml"`,
	)
	mustNotContain(t, "the bump screen", body, esc(i18n.T("web.bump.already")))
}

// TestTheBumpScreenSaysWhenTheVersionWasAlreadyRaised: a document whose version
// is ahead of the last launch's is listed and marked, with no button: pressing it
// again would write the same version twice.
func TestTheBumpScreenSaysWhenTheVersionWasAlreadyRaised(t *testing.T) {
	doc := bumpedDoc()
	doc.Bumped = true
	body := render(t, newBump(t, fakePort{home: Home{Launched: true, Docs: []Doc{doc}}}), "/bump?layer=local")

	mustContain(t, "the bump screen", body, esc(i18n.T("web.bump.already")), ".hergy/teams/demo/protocol.yaml")
	mustNotContain(t, "the bump screen", body, esc(i18n.T("web.bump.raise")), "<form")
}

// TestTheBumpScreenWarnsAboutGlobalLayerDocuments: a document of the global layer
// reaches every project, and the screen says so where the button is.
func TestTheBumpScreenWarnsAboutGlobalLayerDocuments(t *testing.T) {
	doc := bumpedDoc()
	doc.Layer = web.LayerGlobal
	body := render(t, newBump(t, fakePort{home: Home{Launched: true, Docs: []Doc{doc}}}), "/bump?layer=global")

	mustContain(t, "the bump screen", body, esc(i18n.T("web.bump.global")))
}

// TestTheBumpScreenSaysWhenThereIsNothingToRaise: a layer whose documents did not
// move has no version to raise, and a layer that was never launched has nothing
// to compare with: the screen says which, and names the command that records a
// launch.
func TestTheBumpScreenSaysWhenThereIsNothingToRaise(t *testing.T) {
	moved := fakePort{home: Home{Launched: true, Docs: []Doc{{Path: "protocol.yaml", File: "/proj/protocol.yaml", Layer: web.LayerLocal}}}}
	body := render(t, newBump(t, moved), "/bump?layer=local")
	mustContain(t, "the bump screen", body, esc(i18n.T("web.bump.none")))

	never := fakePort{home: Home{}}
	body = render(t, newBump(t, never), "/bump?layer=local")
	mustContain(t, "the bump screen", body, "hergy up &lt;name&gt;")
	mustNotContain(t, "the bump screen", body, esc(i18n.T("web.bump.none")))
}

// TestTheBumpScreenRaisesTheDocumentTheFormNames: the write hands the layer of
// the page and the path on disk to the app, and answers the page with the version
// it wrote and the command that hands the change over.
func TestTheBumpScreenRaisesTheDocumentTheFormNames(t *testing.T) {
	var asked []string
	port := fakePort{
		home:   Home{Launched: true, Docs: []Doc{bumpedDoc()}},
		bumped: &asked,
		bump: Bump{
			Path:       ".hergy/teams/demo/protocol.yaml",
			File:       "/proj/.hergy/teams/demo/protocol.yaml",
			Layer:      web.LayerLocal,
			From:       2,
			To:         3,
			Definition: "demo",
		},
	}
	screen := newBump(t, port)

	// The write route, with the form's own fields, the way the server hands it
	// to the screen.
	form := url.Values{"doc": {"/proj/.hergy/teams/demo/protocol.yaml"}, "token": {"t"}}
	req := httptest.NewRequest(http.MethodPost, "/bump/save?layer=local", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	view, err := writeHandler(t, screen, "/bump/save")(req)
	if err != nil {
		t.Fatal(err)
	}

	if len(asked) != 1 || asked[0] != "local /proj/.hergy/teams/demo/protocol.yaml" {
		t.Fatalf("the app was asked to raise %v, want the layer and the path of the form", asked)
	}
	body := string(view.Body)
	mustContain(t, "the bump screen after a write", body,
		esc(i18n.Tf("web.bump.raised", ".hergy/teams/demo/protocol.yaml", 2, 3)),
		"hergy up demo",
	)
}

// TestTheBumpWriteIsAWrite: the page and the write behind it are separate paths,
// and the write is marked as one, so the server asks for the token of the run
// before the screen sees it.
func TestTheBumpWriteIsAWrite(t *testing.T) {
	routes := newBump(t, fakePort{}).Routes()
	if len(routes) != 2 {
		t.Fatalf("the bump screen answers %d paths, want the page and the write", len(routes))
	}
	if routes[0].Path != "/bump" || routes[0].Write {
		t.Errorf("the page is %+v, want a read at /bump", routes[0])
	}
	if routes[1].Path != "/bump/save" || !routes[1].Write {
		t.Errorf("the write is %+v, want a write at /bump/save", routes[1])
	}
}

// TestTheBumpScreenReadsTheLayerOfTheRequest: the layer travels in the query, and
// the screen asks the app for that layer.
func TestTheBumpScreenReadsTheLayerOfTheRequest(t *testing.T) {
	var asked []string
	port := fakePort{layers: &asked}
	render(t, newBump(t, port), "/bump?layer=global")
	if len(asked) != 1 || asked[0] != web.LayerGlobal {
		t.Errorf("the bump screen asked for %v, want the global layer", asked)
	}
}

// TestBumpsCarryTheVersionAndTheDefinition: what the screen lists is the
// documents whose content moved, each with the version it holds, the one a bump
// writes, and the definition to run after it.
func TestBumpsCarryTheVersionAndTheDefinition(t *testing.T) {
	home := Home{Docs: []Doc{
		{Path: "a.yaml", File: "/p/a.yaml", Layer: web.LayerLocal, Version: 2, Added: []string{"A9"}, Definition: "demo"},
		{Path: "b.yaml", File: "/p/b.yaml", Layer: web.LayerLocal, Version: 0, Changed: []string{"B1"}},
		{Path: "c.yaml", File: "/p/c.yaml", Layer: web.LayerLocal, Version: 4},
		{Path: "d.yaml", File: "/p/d.yaml", Layer: web.LayerLocal, Version: 4, Missing: true},
	}}
	bumps := home.Bumps()
	if len(bumps) != 2 {
		t.Fatalf("the screen would list %+v, want the two documents that moved", bumps)
	}
	first := bumps[0]
	if first.Path != "a.yaml" || first.File != "/p/a.yaml" || first.From != 2 || first.To != 3 || first.Definition != "demo" {
		t.Errorf("the first bump is %+v, want a.yaml at 2 → 3 for demo", first)
	}
	if second := bumps[1]; second.From != 0 || second.To != 1 {
		t.Errorf("a document with no version is at %+v, want 0 → 1", second)
	}
}

// TestADocumentRaisedSinceTheLaunchIsMarkedAndStillListed: the version screen
// keeps a document whose change is still not handed over, and says its version is
// already raised.
func TestADocumentRaisedSinceTheLaunchIsMarkedAndStillListed(t *testing.T) {
	home := Home{Docs: []Doc{{
		Path: "a.yaml", File: "/p/a.yaml", Layer: web.LayerLocal,
		Version: 3, Added: []string{"A9"}, Bumped: true,
	}}}
	bumps := home.Bumps()
	if len(bumps) != 1 || !bumps[0].Raised {
		t.Fatalf("the screen would list %+v, want the raised document marked", bumps)
	}
}
