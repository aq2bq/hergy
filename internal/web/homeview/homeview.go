// Package homeview holds the screens of the launch record (spec/10 の 7 節): the
// home, which shows what moved since the last launch and which rules carry no
// check; and the prompt diff, member by member.
//
// These screens read the record and the settings through their own Port, and
// render through internal/web's frame, so they are added to hergy serve without
// that package changing (d-3a80). What they compare is computed in the app, in
// Go, with the same rules.Parse and config.Parse the commands use: the page
// holds no comparison of its own.
package homeview

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// templates and static are embedded: the screens are part of the binary, and the
// stylesheet is served under /static/homeview/, so no screen has to share
// internal/web's own file (spec/13 の G1).
//
//go:embed templates/*.html static
var files embed.FS

// Port is what the app hands these screens: the launch record read the way the
// commands read it, the same team definitions, and where the rules screen opens
// one rule. A screen asks here and never reaches around it.
type Port interface {
	// Home is what the home screen shows for one layer.
	Home(layer string) Home
	// Diff is the prompt diff of one layer's team definitions.
	Diff(layer string) Diff
	// RuleHref is where the rules screen opens one rule of one document in its
	// edit panel; empty when this build has no rules screen. The home screen
	// links into it; the path belongs to that screen, and this is the one place
	// that knows it.
	RuleHref(layer, path, id string) string
	// Bump raises one document's version by one and answers what it wrote. The
	// app reads the document, raises the version it holds (a document with no
	// version is at 0), and writes the document back the way the rules screen
	// writes one: a path no member of the layer reads is refused, and a file
	// that moved under the screen is not overwritten.
	Bump(layer, path string) (Bump, error)
}

// Home is the home screen's data.
type Home struct {
	// Launched is whether any team definition of the layer has been launched
	// from this machine. Without a launch there is nothing to differ from, and
	// the screen says so and names the command.
	Launched bool
	// Problem is why the record could not be read, as one line; empty when it
	// was read. A record that cannot be read does not hide the rest.
	Problem string
	// Docs is one entry per document a team definition of the layer reads.
	Docs []Doc
}

// Doc is one document of one layer, and what the record says about it.
type Doc struct {
	// Path is where the document is, as a person writes it.
	Path string
	// File is where the document is on disk: what the bump form sends back, so
	// a write names the file rather than the way it is shown.
	File string
	// Layer is which layer supplied it (web.LayerLocal, web.LayerGlobal).
	Layer string
	// Rules is how many rules the document holds now.
	Rules int
	// NoCheck is the rules with no check, in the written order.
	NoCheck []Rule
	// Added, Removed and Changed are the rule ids that differ from the text the
	// last launch read, in the written order of the document now.
	Added   []string
	Removed []string
	Changed []string
	// Whole is set when the texts differ but their rules could not be lined up
	// (one side does not read as a rule file), so the screen says the document
	// changed without naming ids.
	Whole bool
	// Missing is set when the last launch read this document and its text is not
	// in the record, so the two texts cannot be compared.
	Missing bool
	// Bumped is set when the document's version is ahead of the one the last
	// launch read: the next hergy up hands the change over.
	Bumped bool
	// Version is the document's own version now; 0 when the file has none.
	Version int
	// Definition is the team definition whose launch this document is compared
	// with: the newest launch that read it, among the definitions of the layer
	// that read it now. Empty when no launch of the layer read it. It is what
	// the bump screen names as the command that hands the change over.
	Definition string
}

// Changes reports whether anything moved in the document since the last launch.
func (d Doc) Changes() bool {
	return len(d.Added)+len(d.Removed)+len(d.Changed) > 0 || d.Whole || d.Missing
}

// Checked reports whether the document has a rule with no check to show.
func (d Doc) Checked() bool { return len(d.NoCheck) > 0 }

// Rule is one rule of one document, as the home screen lists it.
type Rule struct {
	// ID is the rule's id (C1, P-TURN-1).
	ID string
	// Href is where the rules screen opens this rule in its edit panel, empty
	// when there is nowhere to open it.
	Href string
}

// homeScreen is the home: the step bar, the changes since the last launch, and
// the rules with no check.
type homeScreen struct {
	port      Port
	templates *template.Template
	assets    web.Assets
}

// New builds the home screen. It parses the templates here, so a broken template
// stops hergy serve at once instead of on the first request.
func New(port Port) (web.Screen, error) {
	templates, err := template.New("").Funcs(web.Funcs()).ParseFS(files, "templates/home.html")
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(files, "static")
	if err != nil {
		return nil, err
	}
	return &homeScreen{
		port:      port,
		templates: templates,
		assets:    web.Assets{Name: "homeview", Files: static, Stylesheets: []string{"homeview.css"}},
	}, nil
}

// Name is what the navigation calls the screen.
func (s *homeScreen) Name() string { return i18n.T("web.home.title") }

// Routes answers the root, and the root exactly: the home is the screen hergy
// serve opens with, and the way into the daily flow. Every other path is another
// screen's.
func (s *homeScreen) Routes() []web.Route {
	return []web.Route{{Path: "/{$}", Handler: s.show}}
}

// Assets are this screen's stylesheet, for every screen of the record: it is
// brought once, and the path it is served under is this package's name.
func (s *homeScreen) Assets() web.Assets { return s.assets }

func (s *homeScreen) show(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	data := homeData{
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		Home:      s.port.Home(layer),
		Layer:     layer,
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, "home.html", data); err != nil {
		return web.View{}, err
	}
	return web.View{Title: i18n.T("web.home.title"), Body: web.HTML(buf.String())}, nil
}

// homeData is what the home template is given.
type homeData struct {
	// LayerName is the layer being shown, as a person says it ("ローカル層").
	LayerName string
	// Layer is the layer's name as the URLs carry it (local, global).
	Layer string
	// Home is what the app read for this layer.
	Home Home
}
