package web

import (
	"bytes"
	"errors"
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

// NewTeamList builds the team list screen. It is this package's own screen, but
// the caller places it in the navigation like any other: the screen hergy serve
// opens with is the home, so the frame cannot hold a fixed first screen. The
// caller that builds it also passes it to New, in the order the navigation shows.
func NewTeamList(port Port) (Screen, error) {
	if port == nil {
		return nil, errors.New("web: NewTeamList needs a Port")
	}
	templates, err := template.New("").Funcs(Funcs()).ParseFS(files, "templates/teams.html")
	if err != nil {
		return nil, err
	}
	return &teamList{port: port, templates: templates}, nil
}

// Name is what the navigation calls the screen.
func (t *teamList) Name() string { return i18n.T("web.teams.title") }

// Routes answers /teams. The root belongs to the home, which is the screen
// hergy serve opens with and the way into the daily flow (d-ff49).
func (t *teamList) Routes() []Route {
	return []Route{{Path: "/teams", Handler: t.show}}
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
