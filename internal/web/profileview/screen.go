package profileview

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// ProfilePath is the page this screen answers on. The left list and the home
// screen point at it, so the spelling lives here and nowhere else.
const ProfilePath = "/profiles"

// The two write paths of the page. EditPath takes a row the form holds and
// draws the page again, with one row added or taken out, without touching a
// file; SavePath is the one path that writes the layer's profiles.yaml. Both
// are writes: internal/web refuses a POST that carries no token of this run
// (spec/10 の 4 節), so a person always sees where pressing the button leads.
const (
	EditPath = "/profiles/edit"
	SavePath = "/profiles/save"
)

// The two layers, as the URLs carry them. They repeat internal/web's values so
// this package can be read and tested on its own.
const (
	LayerLocal  = "local"
	LayerGlobal = "global"
)

// Screen is the profiles screen: the Port it reads and writes through, and how
// it draws. It is a web.Screen, so internal/app hands it to web.New beside the
// others.
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

// Routes are the paths this screen answers on: the page, and the two writes the
// form makes (a row added or taken out, and the save).
func (s *Screen) Routes() []web.Route {
	return []web.Route{
		{Path: ProfilePath, Handler: s.show},
		{Path: EditPath, Write: true, Handler: s.edit},
		{Path: SavePath, Write: true, Handler: s.save},
	}
}

// show answers the profiles screen for the layer the URL asks for, with the
// rows as they stand on disk.
func (s *Screen) show(r *http.Request) (web.View, error) {
	body, err := s.page(pageState{layer: web.LayerOf(r), token: web.Token(r)})
	if err != nil {
		return web.View{}, err
	}
	return web.View{Title: s.Name(), Body: web.HTML(body)}, nil
}

// edit answers the add and delete buttons: the page is drawn again from the
// form the person is in, with one more empty row or one row taken out. It writes
// nothing: what the form holds reaches the file when it is saved.
func (s *Screen) edit(r *http.Request) (web.View, error) {
	body, err := s.EditBody(web.LayerOf(r), web.Token(r), r.PostForm)
	if err != nil {
		return web.View{}, err
	}
	return web.View{Title: s.Name(), Body: web.HTML(body)}, nil
}

// save answers the save button: the one write of this screen.
func (s *Screen) save(r *http.Request) (web.View, error) {
	body, err := s.SaveBody(web.LayerOf(r), web.Token(r), r.PostForm)
	if err != nil {
		return web.View{}, err
	}
	return web.View{Title: s.Name(), Body: web.HTML(body)}, nil
}

// ProfileBody draws the page as it is opened: the profiles of one layer's own
// profiles.yaml, and, for the local layer, the profiles a local team definition
// ends up with. The form carries no token, which only the server can put on it.
func (s *Screen) ProfileBody(layer string) (string, error) {
	return s.page(pageState{layer: layer})
}

// pageState is what one drawing of the page is made of.
type pageState struct {
	// layer is the layer being shown ("local" or "global").
	layer string
	// token is this run's write token, which the form carries back; it is empty
	// when the page is drawn outside a request.
	token string
	// rows are the rows the form holds. Nil means the rows as they stand in the
	// layer's file, which is how the page is opened and how it is drawn after a
	// save; the rows a person typed are passed in when a save is refused, so
	// nothing they wrote is lost.
	rows []Row
	// hash is the hash of the file the person was looking at, which the form
	// carries back to the save. Empty means the file read for this drawing.
	hash string
	// notice is what the last save did ("保存しました。"); reason is why nothing
	// was written. At most one of the two is set.
	notice string
	reason string
}

// page draws the profiles page from one state. Everything but the rows comes
// from the layer as it stands on disk, so the table a person reads is the file
// that is there.
func (s *Screen) page(state pageState) (string, error) {
	file := s.port.ReadProfiles(state.layer)
	profiles := s.port.Profiles(state.layer)
	rows := state.rows
	if rows == nil {
		rows = readRows(profiles)
	} else {
		markRows(rows, profiles)
	}
	data := Profiles{
		Layer:     state.layer,
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+state.layer)),
		LayerRoot: s.port.LayerRoot(state.layer),
		Local:     state.layer == LayerLocal,
		Rows:      rows,
		Token:     state.token,
		Hash:      state.hash,
		Broken:    file.Broken,
		Notice:    state.notice,
		Reason:    state.reason,
	}
	// The hash the form carries back is the one the file had when the page was
	// drawn: a page drawn again from the form keeps the hash it came with, so a
	// file changed under the screen is refused rather than written over. The
	// rules screen carries its hash the same way.
	if data.Hash == "" {
		data.Hash = file.Hash
	}
	// Only a local team definition lays the local layer over the global one, so
	// the "what it uses" card is drawn for the local layer alone.
	if data.Local {
		data.Effective = s.port.Uses(state.layer)
	}
	return s.render.Profiles(data)
}

// readRows is the rows of a layer's profile as the form holds them: the args one
// to a line, and each environment variable as its own pair of name and value.
func readRows(profiles []Profile) []Row {
	rows := make([]Row, 0, len(profiles))
	for _, profile := range profiles {
		env := make([]EnvVar, 0, len(profile.Env))
		for _, variable := range profile.Env {
			env = append(env, EnvVar{Name: variable.Name, Value: variable.Value})
		}
		rows = append(rows, Row{
			Name:     profile.Name,
			Kind:     profile.Kind,
			Args:     joinLines(profile.Args),
			Env:      env,
			Over:     profile.Over,
			Shadowed: profile.Shadowed,
		})
	}
	return rows
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
	// Rows are the rows of the form: the profiles of the layer's own
	// profiles.yaml, as the person has them on screen.
	Rows []Row
	// Effective are the profiles a local team definition uses, once the local
	// layer is laid over the global one. Empty for the global layer.
	Effective []Profile
	// Token is this run's write token, which every form carries back; Hash is
	// the hash of the file the person was looking at.
	Token string
	Hash  string
	// Broken is why the layer's profiles.yaml does not read, empty when it does.
	// The page shows the reason instead of the form, because hergy does not
	// write over a file it cannot read.
	Broken string
	// Notice is what the last save did, and Reason why nothing was written. At
	// most one of the two is set.
	Notice string
	Reason string
}
