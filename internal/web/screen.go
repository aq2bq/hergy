package web

import (
	"html/template"
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

// A screen brings no file of its own: the frame carries the one stylesheet
// (internal/web/static/hergy.css, the prototype's own CSS, d-207f) and HTMX,
// and every screen draws with the classes of that stylesheet, so two screens
// cannot collide over a class name.

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
	// Drawer is what the right panel shows with this answer, as the whole
	// element it is: the <aside class="drawer"> and what it holds (the rule
	// being edited, say). Left empty, the frame writes the drawer with nothing
	// in it and no width on the page. A fragment that replaces the drawer
	// (DrawerID) renders the same element and is returned with Bare instead.
	Drawer template.HTML
	// Bare writes the body alone, with no frame around it.
	Bare bool
	// Doc is the document this answer has open, as the left list names it. The
	// frame marks that row. A screen that opens a document without the URL
	// naming it (the rules screen with no ?doc= opens the first one) says so
	// here, so the row is marked as the person sees it. Empty leaves the
	// marking to the ?doc= of the URL.
	Doc string
	// Status is the HTTP status; 0 means 200.
	Status int
}

// NavScreen is a Screen that says whether the navigation lists it. A screen the
// navigation does not list is opened from the left list instead: the team
// definition is one (d-207f), and the frame would otherwise show it twice.
type NavScreen interface {
	// InNav reports whether this screen is a tab of the navigation.
	InNav() bool
}

// DrawerID is the id of the right panel. A screen points a link or a form at it
// with hx-target ("#" + DrawerID) to show what the person picked without
// leaving the page.
const DrawerID = "drawer"
