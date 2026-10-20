package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
)

// This file draws the team-definition screen's two pages (r-0f8c): the list,
// with the figure and the team's own agreement, and the YAML tab. The page is a
// shell: the client script draws everything from the island, and every check and
// write goes through /api/team (spec-boundary 5). The checks themselves are
// check.Team and check.Agreement, in api_team.go.

// t2PageData is one tab's page.
type t2PageData struct {
	Lang  string
	Nav   navData
	Tab   string
	Token string
	Data  template.JS
}

// t2Page draws one tab of the team screen. The client script draws the tabs,
// the rows, the figure and the agreement from the data island.
func (s *Server) t2Page(w http.ResponseWriter, r *http.Request, tab string) {
	raw, err := json.Marshal(s.t2Island())
	if err != nil {
		s.problem(w, r, err)
		return
	}
	data := t2PageData{
		Lang: i18n.Language(), Nav: s.navOf("teams"), Tab: tab,
		Token: s.token, Data: template.JS(raw), //nolint:gosec // raw is json.Marshal output, which escapes <, > and &, so it cannot close the script element that carries it
	}
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, "t2_page", data); err != nil {
		s.problem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// t2Island is everything the screen draws from: the same list answer GET
// /api/team gives, plus the words and the paths a page carries so the screen
// builds no sentence and no path of its own.
type t2Island struct {
	Team     apiList           `json:"team"`
	Project  string            `json:"project"`
	Places   map[string]string `json:"places"`
	Dirs     map[string]string `json:"dirs"`
	Texts    map[string]string `json:"texts"`
	Profiles []string          `json:"profiles"`
}

// t2Island reads the catalog into the page's island.
func (s *Server) t2Island() t2Island {
	out := t2Island{Project: filepath.Base(s.cat.Base()), Team: s.t2List()}
	out.Places = map[string]string{
		catalog.PlaceProject: "プロジェクト",
		catalog.PlaceGlobal:  "共通",
	}
	out.Dirs = map[string]string{
		catalog.PlaceProject: s.cat.Display(s.cat.LayerDir(catalog.Layer(catalog.PlaceProject))) + "/teams/",
		catalog.PlaceGlobal:  s.cat.Display(s.cat.LayerDir(catalog.Layer(catalog.PlaceGlobal))) + "/teams/",
	}
	out.Texts = map[string]string{
		"separator":    t("list.separator"),
		"member_rules": t("size.member_rules"),
		"member_chars": t("size.member_chars"),
	}
	out.Profiles = s.t2SortedNames()
	return out
}

// t2LayerDir stops a write that would make a layer that is not there: a project
// whose .hergy is gone has no place to put a team definition (spec-teams 1.7,
// C5).
func (s *Server) t2LayerDir(place string) error {
	dir := s.cat.LayerDir(catalog.Layer(place))
	// dir is one of the catalog's two layer directories: the place is the
	// screen's own word, and catalog.Layer answers a layer for any word, so no
	// request names a directory of its own.
	if _, err := os.Stat(dir); err != nil { //nolint:gosec // G703: dir is one of the catalog's two layer directories, not a path the request chose
		return i18n.Errorf("error.teams_no_layer", s.cat.Display(dir))
	}
	return nil
}
