package rulesview

import (
	"embed"
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/web"
)

// The screen's own stylesheet. The frame's stylesheet draws the frame; the rules
// screen's classes are the screen's, so the two never have to know each other.
//
//go:embed *.css
var assets embed.FS

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
	}
}

// Assets is the stylesheet this screen brings, served under /static/rules/.
func (s *Screen) Assets() web.Assets {
	return web.Assets{
		Name:        "rules",
		Files:       assets,
		Stylesheets: []string{"rules.css"},
	}
}

// show answers the rules screen. A link may name one rule by its id, and then
// the page opens with that rule in the right panel: that is how another screen
// points at a rule (spec/10 の 7 節).
func (s *Screen) show(r *http.Request) (web.View, error) {
	query := r.URL.Query()
	layer := web.LayerOf(r)
	doc := query.Get("doc")
	body, err := s.RulesBody(layer, doc, query.Get("filter"))
	if err != nil {
		return web.View{}, err
	}
	view := web.View{Title: s.Name(), Body: web.HTML(body)}
	if id := query.Get("rule"); id != "" {
		panel, ok, err := s.PanelFor(layer, doc, id, web.Token(r))
		if err != nil {
			return web.View{}, err
		}
		if ok {
			view.Panel = web.HTML(panel)
		}
	}
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

// answer turns one write into the page: the rules screen, with the rule that
// could not be saved still open in the right panel when that is what happened.
func (s *Screen) answer(answer Answer, err error) (web.View, error) {
	if err != nil {
		return web.View{}, err
	}
	view := web.View{Title: s.Name(), Body: web.HTML(answer.Body)}
	if answer.Panel != "" {
		view.Panel = web.HTML(answer.Panel)
	}
	return view, nil
}
