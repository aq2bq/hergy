package rulesview

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// Name is what the navigation calls this screen.
func (s *Screen) Name() string { return i18n.T("web.rules.title") }

// Routes are the paths this screen answers on: the page a person opens first,
// then the fragments the page swaps into the right panel, then the writes. A
// write is a POST that carries the token of this run and is a full page when it
// lands, so a browser with no script still saves a rule.
func (s *Screen) Routes() []web.Route {
	return []web.Route{
		{Path: PathRules, Handler: s.show},
		{Path: PathPanel, Handler: s.panel},
		{Path: PathPanelEdit, Write: true, Handler: s.panelForm},
		{Path: PathPreview, Write: true, Handler: s.preview},
		{Path: PathSave, Write: true, Handler: s.save},
		{Path: PathDelete, Write: true, Handler: s.delete},
		{Path: PathReport, Write: true, Handler: s.report},
		{Path: PathBump, Write: true, Handler: s.bump},
	}
}

// show answers the rules screen. A link may name one rule by its id, and then
// the page opens with that rule in the right panel: that is how another screen
// points at a rule (spec/10 の 7 節).
func (s *Screen) show(r *http.Request) (web.View, error) {
	query := r.URL.Query()
	layer := web.LayerOf(r)
	doc := query.Get("doc")
	data := s.rulesData(RulesQuery{
		Layer:   layer,
		Path:    doc,
		Filter:  query.Get("filter"),
		Tab:     query.Get("tab"),
		Raw:     query.Get("raw") != "",
		Overlay: query.Get("overlay") != "",
	})
	// The changes tab raises a version through a form, and every write names the
	// token of this run (web.checkWrite).
	data.Token = web.Token(r)
	view := web.View{Title: s.Name(), Doc: data.Doc.Path}
	if id := query.Get("rule"); id != "" {
		drawer, ok, err := s.PanelFor(layer, data.Doc.Path, id, web.Token(r))
		if err != nil {
			return web.View{}, err
		}
		if ok {
			view.Drawer = web.HTML(drawer)
			data.Open = id
		}
	}
	body, err := s.render.Rules(data)
	if err != nil {
		return web.View{}, err
	}
	view.Body = web.HTML(body)
	return view, nil
}

// panel answers the edit panel, swapped into the right panel. The token of the
// run is put on the values here, so the form the panel renders carries it.
func (s *Screen) panel(r *http.Request) (web.View, error) {
	values := r.URL.Query()
	values.Set("token", web.Token(r))
	body, err := s.PanelBody(web.LayerOf(r), r.URL.Query().Get("doc"), values)
	if err != nil {
		return web.View{}, err
	}
	return web.View{Title: s.Name(), Bare: true, Body: web.HTML(body)}, nil
}

// panelForm redraws the panel from the submitted form (an origin added or
// removed, the delete ask).
func (s *Screen) panelForm(r *http.Request) (web.View, error) {
	body, err := s.PanelFormBody(web.LayerOf(r), r.PostForm.Get("doc"), r.PostForm)
	if err != nil {
		return web.View{}, err
	}
	return web.View{Title: s.Name(), Bare: true, Body: web.HTML(body)}, nil
}

// preview answers the preview of the rule being edited.
func (s *Screen) preview(r *http.Request) (web.View, error) {
	body, err := s.PreviewBody(web.LayerOf(r), r.PostForm.Get("doc"), r.PostForm)
	if err != nil {
		return web.View{}, err
	}
	return web.View{Title: s.Name(), Bare: true, Body: web.HTML(body)}, nil
}

func (s *Screen) save(r *http.Request) (web.View, error) {
	return s.answer(s.SaveBody(web.LayerOf(r), r.PostForm.Get("doc"), r.PostForm))
}

func (s *Screen) delete(r *http.Request) (web.View, error) {
	return s.answer(s.DeleteBody(web.LayerOf(r), r.PostForm.Get("doc"), r.PostForm))
}

func (s *Screen) report(r *http.Request) (web.View, error) {
	return s.answer(s.ReportBody(web.LayerOf(r), r.PostForm.Get("doc"), r.PostForm))
}

// bump raises one document's version from the changes tab (spec/10 の 7 節) and
// draws that tab again: what the version moved to, or why nothing was written.
// It is the write the home screen's /bump makes, through the same checks.
func (s *Screen) bump(r *http.Request) (web.View, error) {
	layer := web.LayerOf(r)
	path := r.PostForm.Get("doc")
	answer, err := s.port.Bump(layer, path)
	if err != nil {
		return web.View{}, err
	}
	data := s.rulesData(RulesQuery{Layer: layer, Path: path, Tab: TabChanges})
	data.Bump = answer
	data.Token = web.Token(r)
	body, err := s.render.Rules(data)
	if err != nil {
		return web.View{}, err
	}
	return web.View{Title: s.Name(), Doc: path, Body: web.HTML(body)}, nil
}

// answer turns one write into the page: the rules screen, with the rule that
// could not be saved still open in the right panel when that is what happened.
func (s *Screen) answer(answer Answer, err error) (web.View, error) {
	if err != nil {
		return web.View{}, err
	}
	// The document the write was about is the one the page shows, so the left
	// list keeps marking its row: a POST carries no ?doc= of its own.
	view := web.View{Title: s.Name(), Doc: answer.Doc, Body: web.HTML(answer.Body)}
	if answer.Panel != "" {
		view.Drawer = web.HTML(answer.Panel)
	}
	return view, nil
}
