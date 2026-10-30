package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
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

// emitNewRole writes a new role document in hergy's one format.
func emitNewRole(doc *rules.Document) ([]byte, error) {
	text, err := rules.Emit(doc)
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

// r2RolesDir makes the layer's roles/ ready for a write, and stops a write that
// would make a layer that is not there: a project whose .hergy is gone has no
// place to put a role document. A layer that is there but holds no role yet
// gets its roles/ here (n-86c7, d-4cba). The place is the screen's word; the
// catalog says which directory it is.
func (s *Server) r2RolesDir(place string) error {
	dir := s.cat.LayerDir(catalog.Layer(place))
	if _, err := os.Stat(dir); err != nil {
		return i18n.Errorf("error.roles_no_layer", s.cat.Display(dir))
	}
	return os.MkdirAll(filepath.Join(dir, "roles"), 0o755)
}

// r2WriteError turns a write's failure into the refusal a person reads. A write
// that lost a race with another place is the refusal a save whose read is old
// gets: the file as it is now, and the text the save would have written, so the
// screen keeps the draft and lets the person choose (r-3c77).
func (s *Server) r2WriteError(err error, display, path string, doc func(text string) any) (any, *apiErr) {
	if errors.Is(err, source.ErrChanged) {
		return nil, s.refuseChanged(display, path, doc, false)
	}
	return nil, refuse(http.StatusInternalServerError, err.Error())
}

// r2RenameUsers writes a renamed role's new name into every definition whose
// members play the old one, so the people that read it keep a document that is
// there. Every other line is left as it was.
func (s *Server) r2RenameUsers(old, next string) {
	for _, listing := range s.cat.Definitions() {
		path := listing.GlobalPath
		if listing.Local {
			path = listing.LocalPath
		}
		text, err := s.cat.Reader().Read(source.Doc{Path: path})
		if err != nil {
			continue
		}
		edited := text
		for _, member := range r2MembersOf(text) {
			if member.Saved != old {
				continue
			}
			if out, err := definition.EditYAML([]byte(edited), s.cat.Display(path), definition.MemberRole(member.Name, next)); err == nil {
				edited = string(out)
			}
		}
		if edited != text {
			_ = source.WriteBeside(path, edited)
		}
	}
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
