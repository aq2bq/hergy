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
// the HTML back in a View. The team list is the first screen; the rules, the
// records and the topology are added the same way.
type Screen interface {
	// Name is what the navigation calls this screen, in the language in use.
	Name() string
	// Routes are the paths this screen answers on.
	Routes() []Route
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
	// Bare writes the body alone, with no frame around it.
	Bare bool
	// Status is the HTTP status; 0 means 200.
	Status int
}
