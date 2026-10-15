package web

import (
	"html/template"
	"io/fs"
	"net/http"
)

// Screen is one part of the UI: the paths it answers on, and the name the
// navigation shows. The frame around it (the navigation, the layer toggle and
// the right panel) comes from this package, so a screen carries no page chrome
// of its own, and a screen can be added without this package's routes changing.
//
// A screen renders its own body with its own templates (parsed with Funcs, so
// the words and the rendering rules are the same as this package's) and hands
// the HTML back in a View. The home, the rules, the records and the topology are
// all added the same way; the caller passes them in the order the navigation
// shows, and the screen it passes first is the one hergy serve opens with.
//
// A screen's body carries no style and no script of its own: every answer sets
// Content-Security-Policy: default-src 'self', so an inline <style>, <script> or
// style="..." (in an SVG too) is refused by the browser. A screen says how it
// looks with classes and with the files it brings itself (ScreenAssets).
type Screen interface {
	// Name is what the navigation calls this screen, in the language in use.
	Name() string
	// Routes are the paths this screen answers on. The first one is where the
	// navigation points, so it is a page a person can open (not a write, and no
	// "{id}" in it); a screen that only answers fragments puts its page first
	// and its fragments after it.
	Routes() []Route
}

// ScreenAssets is a Screen that brings files of its own: its stylesheet, its
// script, or something it draws with (a screen's SVG). The files are served
// under /static/<Assets.Name>/ and linked in the frame's head, so two screens
// never have to share a file and a screen is added without touching this
// package's own stylesheet. A screen that brings none has the frame's stylesheet
// and HTMX.
type ScreenAssets interface {
	// Assets are the files of this screen.
	Assets() Assets
}

// Assets is what one screen brings to the page.
type Assets struct {
	// Name is where the files are served: /static/<Name>/. It is a URL and not
	// a word, so it is the same in every language, and it is given here rather
	// than taken from Name (which is what the navigation shows, translated).
	Name string
	// Files holds the files, under the names they are served at.
	Files fs.FS
	// Stylesheets and Scripts are names in Files, in the order they load. Both
	// are looked up when the server is built, so a name that is not there stops
	// hergy serve instead of showing a page with a broken link.
	Stylesheets []string
	Scripts     []string
}

// Route is one path and how it is answered.
type Route struct {
	// Path is a net/http pattern ("/teams", "/rules/{id}"). "/{$}" is the exact
	// root: it answers "/" alone and leaves every other path to the fallback.
	Path string
	// Write says this route changes something. A write route answers only POST,
	// and the request must carry the token of this run (the hidden field of
	// every form the screens render) and, when the browser sends an Origin, come
	// from this server's own. Every other route answers GET and HEAD alone.
	Write bool
	// Handler answers the path. What it returns is written inside the frame.
	Handler func(*http.Request) (View, error)
}

// View is one answer: the body a screen wants shown, and whether the frame goes
// around it. A fragment (an HTMX swap into the right panel, say) sets Bare, and
// its body is written on its own, which is what a swap expects.
type View struct {
	// Title is the page title: the frame shows it in the tab and above the body.
	Title string
	// Body is the screen's HTML, already rendered.
	Body template.HTML
	// Panel is what the right panel shows with this answer. Left empty, the
	// panel shows the frame's guidance; filled, the page opens with what the
	// screen puts here (the rule being edited, say). A fragment that is swapped
	// into the panel alone (PanelID) is returned with Bare instead.
	Panel template.HTML
	// Bare writes the body alone, with no frame around it.
	Bare bool
	// Status is the HTTP status; 0 means 200.
	Status int
}

// PanelID is the id of the right panel. A screen points a link or a form at it
// with hx-target ("#" + PanelID) to show what the person picked without leaving
// the page.
const PanelID = "panel"
