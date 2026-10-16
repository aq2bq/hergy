package profileview

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// ProfilePath is the page this screen answers on. The left list and the home
// screen point at it, so the spelling lives here and nowhere else.
const ProfilePath = "/profiles"

// The two layers, as the URLs carry them. They repeat internal/web's values so
// this package can be read and tested on its own.
const (
	LayerLocal  = "local"
	LayerGlobal = "global"
)

// Screen is the profiles screen: the Port it reads through, and how it draws.
// It is a web.Screen, so internal/app hands it to web.New beside the others.
type Screen struct {
	port   Port
	render *Renderer
}

// New builds the screen for one run, parsing its templates with the WebUI's
// functions (web.Funcs), so the words are the same here as everywhere else.
func New(port Port) (*Screen, error) {
	render, err := NewRenderer(web.Funcs())
	if err != nil {
		return nil, err
	}
	return &Screen{port: port, render: render}, nil
}

// Name is what the page is called, in the language in use.
func (s *Screen) Name() string { return i18n.T("web.profiles.title") }

// InNav says the navigation does not list this screen (r-bb9c): it is opened
// from the left list, the way the team definition screen is, so a tab for it
// would show the same place twice.
func (s *Screen) InNav() bool { return false }

// Routes are the paths this screen answers on: the page alone. It is read-only,
// so it has no write route.
func (s *Screen) Routes() []web.Route {
	return []web.Route{{Path: ProfilePath, Handler: s.show}}
}

// show answers the profiles screen for the layer the URL asks for.
func (s *Screen) show(r *http.Request) (web.View, error) {
	body, err := s.ProfileBody(web.LayerOf(r))
	if err != nil {
		return web.View{}, err
	}
	return web.View{Title: s.Name(), Body: web.HTML(body)}, nil
}

// ProfileBody draws the profiles of one layer: the layer's own profiles.yaml,
// and, for the local layer, the profiles a local team definition ends up with.
func (s *Screen) ProfileBody(layer string) (string, error) {
	return s.render.Profiles(s.data(layer))
}

// data is what the screen is drawn from: the layer being shown, its root, the
// profiles its own file holds, and, for the local layer, the profiles a team
// definition there uses.
func (s *Screen) data(layer string) Profiles {
	out := Profiles{
		Layer:     layer,
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		LayerRoot: s.port.LayerRoot(layer),
		Local:     layer == LayerLocal,
		Rows:      s.port.Profiles(layer),
	}
	// Only a local team definition lays the local layer over the global one, so
	// the "what it uses" card is drawn for the local layer alone.
	if out.Local {
		out.Effective = s.port.Uses(layer)
	}
	return out
}

// Profiles is what the profiles page is drawn from.
type Profiles struct {
	// Layer is the layer being shown ("local" or "global"), LayerName what a
	// person calls it with its 層 ("ローカル層"), and LayerRoot where its files
	// live (".hergy"), which the heading spells out with profiles.yaml.
	Layer     string
	LayerName string
	LayerRoot string
	// Local says the local layer is being shown, which is when the card that
	// says what a local team definition uses is drawn.
	Local bool
	// Rows are the profiles of the layer's own profiles.yaml.
	Rows []Profile
	// Effective are the profiles a local team definition uses, once the local
	// layer is laid over the global one. Empty for the global layer.
	Effective []Profile
}
