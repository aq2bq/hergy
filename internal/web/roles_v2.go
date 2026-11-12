package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
)

// This file is the role screen's own answers (r-778c): the page the three tabs
// are drawn on, the check a field sends while it is typed, and the writes of
// role documents, team.yaml and the YAML tab. Every write carries the hash of
// the text the page read, so a file that moved under the page is refused.
// The checks are all of check.Role: the screen makes no rule of its own.

// r2PageData is one tab's page.
type r2PageData struct {
	Lang  string
	Nav   navData
	Tab   string
	Token string
	Data  template.JS
}

// r2Page draws one tab of the role screen. The page is its own, not the frame's:
// the client script draws the tabs, the rows and the three tab bodies from the
// data island.
func (s *Server) r2Page(w http.ResponseWriter, r *http.Request, tab string) {
	scoped, refusal := s.withProjectInput(projectQuery(r))
	if refusal != nil {
		writeJSON(w, refusal.status, refusal)
		return
	}
	s = scoped
	raw, err := json.Marshal(s.r2PageIsland())
	if err != nil {
		s.problem(w, r, err)
		return
	}
	data := r2PageData{
		Lang: i18n.Language(), Nav: s.navOf("roles"), Tab: tab,
		Token: s.token, Data: template.JS(raw), //nolint:gosec // raw is json.Marshal output, which escapes <, > and &, so it cannot close the script element that carries it
	}
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, "r2_page", data); err != nil {
		s.problem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// r2OldRole moves a role's old page link to the row it names, opened: the old
// screen had a page per role, and this one opens the row in the list.
func (s *Server) r2OldRole(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/roles?open="+url.QueryEscape(r.PathValue("role")), http.StatusMovedPermanently)
}

// r2OldRoleNew moves the old "make a rule" link to the opened row too.
func (s *Server) r2OldRoleNew(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/roles?open="+url.QueryEscape(r.PathValue("role")), http.StatusMovedPermanently)
}

// r2OldRoleRule moves an old one-rule link to the row and the rule it names.
// The place is the layer the name resolves to: the project's document when it
// has one, the global one otherwise.
func (s *Server) r2OldRoleRule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("role")
	spec := name + "." + s.r2PlaceOf(name) + "." + r.PathValue("rule")
	http.Redirect(w, r, "/roles?open="+url.QueryEscape(name)+"&rule="+url.QueryEscape(spec), http.StatusMovedPermanently)
}

// r2PlaceOf is the place a role name resolves to: the project's layer when it
// holds a document, the shared one otherwise.
func (s *Server) r2PlaceOf(name string) string {
	for _, entry := range s.cat.Roles() {
		if entry.Name != name || !entry.HasDocument {
			continue
		}
		if entry.Outside {
			return catalog.PlaceGlobal
		}
		return catalog.PlaceProject
	}
	return catalog.PlaceProject
}
