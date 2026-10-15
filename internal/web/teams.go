package web

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
)

// teamList is the team list screen (spec/10 の 5 節): one row per team definition
// of the layer being shown. The columns it holds are the name, the number of
// members, and how many of the rules those members read have no check. A
// definition that cannot be read keeps its row and shows why, in place of the
// counts, so one broken file hides nothing.
//
// The columns the parts after this one add (the launch record, the running mark)
// are columns of this same table.
type teamList struct {
	port      Port
	templates *template.Template
}

// Name is what the navigation calls the screen.
func (t *teamList) Name() string { return i18n.T("web.teams.title") }

// Routes answers the root, and the root exactly: every other path is another
// screen's, and an unclaimed one is not found rather than this list.
func (t *teamList) Routes() []Route {
	return []Route{{Path: "/{$}", Handler: t.show}}
}

func (t *teamList) show(r *http.Request) (View, error) {
	layer := LayerOf(r)
	data := teamsData{
		LayerName: i18n.Tf("web.layer.name", i18n.T("label."+layer)),
		Teams:     t.port.Teams(layer),
	}
	var buf bytes.Buffer
	if err := t.templates.ExecuteTemplate(&buf, "teams.html", data); err != nil {
		return View{}, err
	}
	return View{Title: i18n.T("web.teams.title"), Body: HTML(buf.String())}, nil
}

// teamsData is what the team list template is given.
type teamsData struct {
	// LayerName is the layer being shown, as a person says it ("ローカル層").
	LayerName string
	// Teams is one row per team definition, by name.
	Teams []Team
}
