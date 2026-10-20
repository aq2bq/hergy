package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// apiRole is the role kind: the role documents the role screen reads and writes
// (spec-boundary 8). Its checks are all of check.Role; the kind makes no rule of
// its own.
type apiRole struct{ s *Server }

// List is GET /api/role: the same reading the screen's data island carries.
func (k apiRole) List() (any, *apiErr) {
	return k.s.r2IslandOf(), nil
}

// Get is GET /api/role/{name}?place=: one document, its text and its hash.
func (k apiRole) Get(name, place string) (any, *apiErr) {
	s := k.s
	path := s.cat.RolePath(place, name)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	doc, _ := rules.ReadAll(text, s.cat.Display(path))
	if doc == nil {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	return map[string]any{
		"doc":  r2DocOf(doc, source.Hash(text), text),
		"text": text,
		"hash": source.Hash(text),
		"path": s.cat.Display(path),
	}, nil
}

// Check is POST /api/role/check: what is wrong with a panel's draft, or with the
// YAML a person is typing.
func (k apiRole) Check(in apiIn) (any, *apiErr) {
	if len(in.Doc) == 0 {
		report := check.Role(check.RoleInput{Catalog: k.s.cat, Place: catalog.Layer(in.Place), Name: in.Name, Text: in.Text})
		return map[string]any{"problems": report.Problems}, nil
	}
	var doc r2Doc
	if err := json.Unmarshal(in.Doc, &doc); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	original := in.Original
	if original == doc.Name {
		original = ""
	}
	return check.Role(check.RoleInput{
		Catalog: k.s.cat, Place: catalog.Layer(in.Place), Name: doc.Name,
		Draft: doc.document(), Original: original,
	}), nil
}

// Save is PUT /api/role/{name}: a panel's draft, or a file's text when the
// request carries no doc. The name in the path is the row's; the doc is the one
// that says what to write.
func (k apiRole) Save(_ string, in apiIn) (any, *apiErr) {
	if len(in.Doc) == 0 {
		return k.saveYAML(in)
	}
	return k.saveDoc(in)
}

// saveDoc writes one panel's draft into its role document, in one pass: every
// value the draft changed over the lines the saved text already writes (the old
// /roles/save).
func (k apiRole) saveDoc(in apiIn) (any, *apiErr) {
	s := k.s
	var doc r2Doc
	if err := json.Unmarshal(in.Doc, &doc); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	if err := s.r2LayerDir(in.Place); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	target := s.cat.RolePath(in.Place, doc.Name)
	savedName := in.Original
	if savedName == "" {
		savedName = doc.Name
	}
	old := s.cat.RolePath(in.Place, savedName)
	if !catalog.RoleNamePattern.MatchString(doc.Name) {
		return nil, refuseAt(http.StatusUnprocessableEntity, "name", tf("error.role_name_pattern", s.cat.Display(target)))
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
			return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(old)))
		}
	case in.Hash != "":
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(old)))
	}
	rename := doc.Name != savedName
	if rename {
		if _, err := os.Stat(target); err == nil {
			return nil, refuseAt(http.StatusUnprocessableEntity, "name", tf("error.role_name_taken", doc.Name, s.cat.Display(target)))
		}
	}
	base, baseErr := text, readErr
	if basePath != old {
		base, baseErr = s.cat.Reader().Read(source.Doc{Path: basePath})
	}
	draftDoc := doc.document()
	var edited []byte
	var err error
	if baseErr != nil {
		edited, err = emitNewRole(draftDoc)
	} else {
		saved, _ := rules.ReadAll(base, s.cat.Display(basePath))
		if saved == nil {
			saved = &rules.Document{Path: basePath}
		}
		edited, err = rules.Rewrite([]byte(base), saved, rules.Draft{Doc: draftDoc, From: doc.froms(), SectionFrom: doc.sectionFroms()})
	}
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
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
			return nil, refuse(http.StatusInternalServerError, err.Error())
		}
	} else if err := source.WriteIfUnchanged(old, writeHash, string(edited)); err != nil {
		return r2WriteError(err, s.cat.Display(old))
	}
	if rename && in.RenameUsers {
		s.r2RenameUsers(savedName, doc.Name)
	}
	got, _ := rules.ReadAll(string(edited), s.cat.Display(target))
	if got == nil {
		got = &rules.Document{Path: target}
	}
	return map[string]any{
		"doc":   r2DocOf(got, source.Hash(string(edited)), string(edited)),
		"text":  string(edited),
		"hash":  source.Hash(string(edited)),
		"rules": len(got.Rules()),
		"chars": rules.CharCount(got.Markdown()),
		"list":  s.r2IslandOf(),
	}, nil
}

// saveYAML writes one file's text as the person wrote it (the old
// /roles/yaml/save). It is refused only when a problem the saved document did
// not have has appeared (spec-roles X3).
func (k apiRole) saveYAML(in apiIn) (any, *apiErr) {
	s := k.s
	if err := s.r2LayerDir(in.Place); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	path := s.cat.RolePath(in.Place, in.Name)
	display := s.cat.Display(path)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	hash := source.Hash("")
	if err == nil {
		hash = source.Hash(text)
	}
	if in.Hash != hash {
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", display))
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
			return nil, refuse(http.StatusUnprocessableEntity, problem.Text)
		}
	}
	if err := source.WriteIfUnchanged(path, hash, in.Text); err != nil {
		return r2WriteError(err, display)
	}
	doc, _ := rules.ReadAll(in.Text, display)
	if doc == nil {
		doc = &rules.Document{Path: path}
	}
	return map[string]any{
		"hash": source.Hash(in.Text),
		"doc":  r2DocOf(doc, source.Hash(in.Text), in.Text),
	}, nil
}

// Delete is DELETE /api/role/{name}?place=&hash=: take one role document out of
// its layer.
func (k apiRole) Delete(name, place, hash string) (any, *apiErr) {
	s := k.s
	path := s.cat.RolePath(place, name)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	pathHash := source.Hash("")
	if err == nil {
		pathHash = source.Hash(text)
	}
	if hash != pathHash {
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(path)))
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) { //nolint:gosec // G703: path is the catalog's own RolePath, which joins the layer's own roles directory with the name
		return nil, refuse(http.StatusInternalServerError, err.Error())
	}
	return s.r2IslandOf(), nil
}
