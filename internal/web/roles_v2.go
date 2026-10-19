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
	"github.com/aq2bq/hergy/internal/check"
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
	Lang    string
	Project string
	Tab     string
	Token   string
	Data    template.JS
}

// r2Page draws one tab of the role screen. The page is its own, not the frame's:
// the client script draws the tabs, the rows and the three tab bodies from the
// data island.
func (s *Server) r2Page(w http.ResponseWriter, r *http.Request, tab string) {
	raw, err := json.Marshal(s.r2IslandOf())
	if err != nil {
		s.problem(w, r, err)
		return
	}
	data := r2PageData{
		Lang: i18n.Language(), Project: filepath.Base(s.cat.Base()), Tab: tab,
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

// r2CheckIn is what /roles/check carries.
type r2CheckIn struct {
	Place    string `json:"place"`
	Original string `json:"original"`
	Doc      r2Doc  `json:"doc"`
}

// r2Check answers what is wrong with the role document a panel holds. The draft
// is checked as if it were saved (check.Role lays it over the catalog itself).
func (s *Server) r2Check(r *http.Request) p2Answer {
	var in r2CheckIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	original := in.Original
	if original == in.Doc.Name {
		original = ""
	}
	report := check.Role(check.RoleInput{
		Catalog: s.cat, Place: catalog.Layer(in.Place), Name: in.Doc.Name,
		Draft: in.Doc.document(), Original: original,
	})
	return p2JSONAnswer(report)
}

// r2SaveIn is what /roles/save carries.
type r2SaveIn struct {
	Place       string `json:"place"`
	Original    string `json:"original"`
	Doc         r2Doc  `json:"doc"`
	RenameUsers bool   `json:"renameUsers"`
	Hash        string `json:"hash"`
	// Base is the file a duplicate or a copy is written from (spec-roles 1.2.9):
	// the original's document, which the draft keeps the places of and the
	// server writes its lines from. Empty means the file being replaced, or, for
	// a row that has none, a new document written from the draft alone.
	Base *r2Base `json:"base,omitempty"`
}

// r2Base is the file a save stands on when it is not the file it replaces.
type r2Base struct {
	Place string `json:"place"`
	Name  string `json:"name"`
}

// r2Save writes one panel's draft into its role document, in one pass: every
// value the draft changed over the lines the saved text already writes. A new
// document is written from the draft (Emit); a duplicate or a copy is written
// from the file it stands on, so the lines a person did not touch, a comment
// among them, travel to the new file (spec-roles 1.2.9).
func (s *Server) r2Save(r *http.Request) p2Answer {
	var in r2SaveIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if err := s.r2LayerDir(in.Place); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	target := s.cat.RolePath(in.Place, in.Doc.Name)
	savedName := in.Original
	if savedName == "" {
		savedName = in.Doc.Name
	}
	old := s.cat.RolePath(in.Place, savedName)
	if !catalog.RoleNamePattern.MatchString(in.Doc.Name) {
		return r2NamedAnswer(http.StatusUnprocessableEntity, "name", tf("error.role_name_pattern", s.cat.Display(target)))
	}
	// The text the draft stands on: the file being replaced, or the file a
	// duplicate or a copy is based on. A row with neither stands on nothing and
	// is a new document.
	basePath := old
	if in.Base != nil {
		basePath = s.cat.RolePath(in.Base.Place, in.Base.Name)
	}
	text, readErr := s.cat.Reader().Read(source.Doc{Path: old})
	switch {
	case readErr == nil:
		if in.Hash != source.Hash(text) {
			return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(old)))
		}
	case in.Hash != "":
		return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(old)))
	}
	rename := in.Doc.Name != savedName
	if rename {
		if _, err := os.Stat(target); err == nil {
			return r2NamedAnswer(http.StatusUnprocessableEntity, "name", tf("error.role_name_taken", in.Doc.Name, s.cat.Display(target)))
		}
	}
	base, baseErr := text, readErr
	if basePath != old {
		base, baseErr = s.cat.Reader().Read(source.Doc{Path: basePath})
	}
	draftDoc := in.Doc.document()
	var edited []byte
	var err error
	if baseErr != nil {
		edited, err = emitNewRole(draftDoc)
	} else {
		saved, _ := rules.ReadAll(base, s.cat.Display(basePath))
		if saved == nil {
			saved = &rules.Document{Path: basePath}
		}
		edited, err = rules.Rewrite([]byte(base), saved, rules.Draft{Doc: draftDoc, From: in.Doc.froms()})
	}
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	writeHash := source.Hash("")
	if readErr == nil {
		writeHash = source.Hash(text)
	}
	if rename {
		if err := source.WriteIfUnchanged(target, source.Hash(""), string(edited)); err != nil {
			return r2WriteError(err, s.cat.Display(target))
		}
		if err := os.Remove(old); err != nil {
			return p2TextAnswer(http.StatusInternalServerError, err.Error())
		}
	} else if err := source.WriteIfUnchanged(old, writeHash, string(edited)); err != nil {
		return r2WriteError(err, s.cat.Display(old))
	}
	if rename && in.RenameUsers {
		s.r2RenameUsers(savedName, in.Doc.Name)
	}
	doc, _ := rules.ReadAll(string(edited), s.cat.Display(target))
	if doc == nil {
		doc = &rules.Document{Path: target}
	}
	return p2JSONAnswer(map[string]any{
		"ok":    true,
		"hash":  source.Hash(string(edited)),
		"text":  string(edited),
		"rules": len(doc.Rules()),
		"chars": rules.CharCount(doc.Markdown()),
	})
}

// emitNewRole writes a new role document in hergy's one format.
func emitNewRole(doc *rules.Document) ([]byte, error) {
	text, err := rules.Emit(doc)
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

// r2NamedAnswer is a refusal that names the field the sentence belongs to, so
// the screen puts a document's refusal at the panel's footer and a name's under
// the name field (spec-roles 1.2.9).
func r2NamedAnswer(status int, field, text string) p2Answer {
	return p2Answer{
		status: status, ctype: "application/json; charset=utf-8",
		body: p2JSON(map[string]string{"text": text, "field": field}),
	}
}

// r2DeleteIn is what /roles/delete carries.
type r2DeleteIn struct {
	Place string `json:"place"`
	Name  string `json:"name"`
	Hash  string `json:"hash"`
}

// r2Delete takes one role document out of its layer.
func (s *Server) r2Delete(r *http.Request) p2Answer {
	var in r2DeleteIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	path := s.cat.RolePath(in.Place, in.Name)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if err == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(path)))
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return p2TextAnswer(http.StatusInternalServerError, err.Error())
	}
	return p2JSONAnswer(map[string]any{"ok": true})
}

// r2YAMLIn is what the YAML tab's checks and saves carry.
type r2YAMLIn struct {
	Place string `json:"place"`
	Name  string `json:"name"`
	Text  string `json:"text"`
	Hash  string `json:"hash"`
}

// r2YAMLCheck answers what is wrong with the YAML a person is typing: the same
// check.Role, read from the text, so a problem carries its YAML line.
func (s *Server) r2YAMLCheck(r *http.Request) p2Answer {
	var in r2YAMLIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	report := check.Role(check.RoleInput{Catalog: s.cat, Place: catalog.Layer(in.Place), Name: in.Name, Text: in.Text})
	return p2JSONAnswer(map[string]any{"problems": report.Problems})
}

// r2YAMLSave writes one file's text as the person wrote it. It is refused only
// when a problem the saved document did not have has appeared (spec-roles X3).
func (s *Server) r2YAMLSave(r *http.Request) p2Answer {
	var in r2YAMLIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if err := s.r2LayerDir(in.Place); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	path := s.cat.RolePath(in.Place, in.Name)
	display := s.cat.Display(path)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if err == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", display))
	}
	_, before := rules.ReadAll(text, display)
	known := map[string]bool{}
	for _, problem := range before {
		if problem.Sev == rules.SevError {
			known[problem.Text] = true
		}
	}
	_, after := rules.ReadAll(in.Text, display)
	for _, problem := range after {
		if problem.Sev == rules.SevError && !known[problem.Text] {
			return p2TextAnswer(http.StatusUnprocessableEntity, problem.Text)
		}
	}
	if err := source.WriteIfUnchanged(path, hash, in.Text); err != nil {
		return r2WriteError(err, display)
	}
	doc, _ := rules.ReadAll(in.Text, display)
	if doc == nil {
		doc = &rules.Document{Path: path}
	}
	return p2JSONAnswer(map[string]any{
		"hash": source.Hash(in.Text),
		"doc":  r2DocOf(doc, source.Hash(in.Text), in.Text),
	})
}

// r2AssignIn is what the assign tab carries: one card's changed picks.
type r2AssignIn struct {
	Definition string           `json:"definition"`
	Changes    []r2AssignChange `json:"changes"`
	Hash       string           `json:"hash"`
}

type r2AssignChange struct {
	Member string `json:"member"`
	Role   string `json:"role"`
}

// r2AssignSave writes one card's picks into its team.yaml.
func (s *Server) r2AssignSave(r *http.Request) p2Answer {
	var in r2AssignIn
	if err := r2Decode(r, &in); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	listing, ok := s.r2Listing(in.Definition)
	if !ok {
		return p2TextAnswer(http.StatusNotFound, tf("web.error.not_found"))
	}
	path := listing.GlobalPath
	if listing.Local {
		path = listing.LocalPath
	}
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if in.Hash != source.Hash(text) {
		return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(path)))
	}
	edited := text
	for _, change := range in.Changes {
		next, err := definition.EditYAML([]byte(edited), s.cat.Display(path), definition.MemberRole(change.Member, change.Role))
		if err != nil {
			return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
		}
		edited = string(next)
	}
	if err := source.WriteIfUnchanged(path, source.Hash(text), edited); err != nil {
		return r2WriteError(err, s.cat.Display(path))
	}
	return p2JSONAnswer(map[string]any{"ok": true, "hash": source.Hash(edited)})
}

// r2LayerDir stops a write that would make a layer that is not there: a project
// whose .hergy is gone has no place to put a role document. The place is the
// screen's word; the catalog says which directory it is.
func (s *Server) r2LayerDir(place string) error {
	dir := s.cat.LayerDir(catalog.Layer(place))
	if _, err := os.Stat(dir); err != nil {
		return i18n.Errorf("error.roles_no_layer", s.cat.Display(dir))
	}
	return nil
}

// r2WriteError turns a write's failure into the answer a person reads.
func r2WriteError(err error, display string) p2Answer {
	if errors.Is(err, source.ErrChanged) {
		return p2TextAnswer(http.StatusConflict, tf("error.rules_file_changed", display))
	}
	return p2TextAnswer(http.StatusInternalServerError, err.Error())
}

// r2Listing is one team definition as the catalog lists it.
func (s *Server) r2Listing(name string) (catalog.Listing, bool) {
	for _, listing := range s.cat.Definitions() {
		if listing.Name == name {
			return listing, true
		}
	}
	return catalog.Listing{}, false
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
