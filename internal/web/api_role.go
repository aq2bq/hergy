package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// apiRole is the role kind: the role documents the role screen reads and writes
// (spec-boundary 8). Its checks are all of check.Role; the kind makes no rule of
// its own.
type apiRole struct{ s *Server }

// apiRoleSummary is the role list's columns: how many rules the document puts in
// a prompt and how much one person reads, and the row a person still has to
// write a document for (spec-boundary 5).
type apiRoleSummary struct {
	Rules    int    `json:"rules"`
	Chars    int    `json:"chars"`
	Markdown string `json:"markdown,omitempty"`
	NoDoc    bool   `json:"noDoc,omitempty"`
	IsNew    bool   `json:"isNew,omitempty"`
}

// apiEffect is the effect of a document's change (spec-boundary 5): how the
// Markdown a person reads moved, and the arrows the rules write judged for each
// definition that reads them.
type apiEffect struct {
	Diff  check.RoleDiff   `json:"diff"`
	Turns []check.RoleTurn `json:"turns"`
}

// List is GET /api/role: one item per role name, with the documents behind it.
func (k apiRole) List() (any, *apiErr) {
	return k.s.r2List(), nil
}

// r2List is the role list answer: the rows the screen reads, as items.
func (s *Server) r2List() apiList {
	return s.r2ListOf(s.r2ReadingOf())
}

// r2ListOf is the list answer of one reading.
func (s *Server) r2ListOf(reading r2Reading) apiList {
	out := apiList{Hashes: map[string]string{}}
	for _, row := range reading.Roles {
		summary := r2SummaryOf(row, apiRoleSummary{Markdown: row.Markdown, NoDoc: row.NoDoc, IsNew: row.IsNew})
		item := apiItem{
			Name:      r2RowName(row),
			Places:    map[string]apiPlace{},
			Effective: r2Effective(row),
			Summary:   summary,
		}
		if s.r2RowLive(row, catalog.PlaceProject) && s.r2RowLive(row, catalog.PlaceGlobal) {
			item.Hidden = []string{catalog.PlaceGlobal}
		}
		for _, place := range []struct {
			name string
			doc  *r2Doc
		}{{catalog.PlaceProject, row.Project}, {catalog.PlaceGlobal, row.Global}} {
			if place.doc == nil {
				continue
			}
			doc := *place.doc
			doc.Text, doc.Hash = "", ""
			item.Places[place.name] = apiPlace{
				Doc: doc, Text: place.doc.Text, Hash: place.doc.Hash,
				Path:     s.cat.Display(s.cat.RolePath(place.name, place.doc.Name)),
				Problems: r2DocCounts(place.doc.Text, s.cat.Display(s.cat.RolePath(place.name, place.doc.Name))),
			}
			out.Hashes[place.name+"."+place.doc.Name] = place.doc.Hash
		}
		item.Summary = summary
		out.Items = append(out.Items, item)
	}
	return out
}

// r2SummaryOf fills the columns only a document can answer: its rules in a
// prompt and the amount one person reads.
func r2SummaryOf(row r2Row, summary apiRoleSummary) apiRoleSummary {
	doc := r2EffectiveDoc(row)
	if doc == nil {
		return summary
	}
	parsed := doc.document()
	if parsed == nil {
		return summary
	}
	summary.Rules = len(parsed.IncludedRules())
	summary.Chars = rules.CharCount(parsed.Markdown())
	return summary
}

// r2DocCounts is one document's own problems, counted from its text.
func r2DocCounts(text, display string) apiCounts {
	_, problems := rules.ReadAll(text, display)
	var out apiCounts
	for _, problem := range problems {
		if problem.Sev == rules.SevError {
			out.Err++
		} else {
			out.Warn++
		}
	}
	return out
}

// r2RowName is the name a row carries: the one it was read with, or the name of
// the document behind it.
func r2RowName(row r2Row) string {
	if row.Name != "" {
		return row.Name
	}
	if row.Project != nil {
		return row.Project.Name
	}
	if row.Global != nil {
		return row.Global.Name
	}
	return ""
}

// r2RowLive is whether one place holds a written document for a row.
func (s *Server) r2RowLive(row r2Row, place string) bool {
	doc := r2PlaceDoc(row, place)
	return doc != nil && doc.Name != "" && !row.NoDoc
}

// r2PlaceDoc is the document one place of a row holds, if either does.
func r2PlaceDoc(row r2Row, place string) *r2Doc {
	if place == catalog.PlaceGlobal {
		return row.Global
	}
	return row.Project
}

// r2Effective is the place in effect for a row: the project's document when it
// has one, the shared one otherwise, the place a new document would go.
func r2Effective(row r2Row) string {
	if row.Global != nil && row.Project == nil {
		return catalog.PlaceGlobal
	}
	return catalog.PlaceProject
}

// r2EffectiveDoc is the document in effect for a row.
func r2EffectiveDoc(row r2Row) *r2Doc {
	return r2PlaceDoc(row, r2Effective(row))
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
	effective, hidden := s.r2RowPlaces(name)
	return map[string]any{
		"doc":       r2DocOf(doc, source.Hash(text), text),
		"text":      text,
		"hash":      source.Hash(text),
		"path":      s.cat.Display(path),
		"effective": effective,
		"hidden":    hidden,
	}, nil
}

// r2RowPlaces is which place of a name is in effect and which is hidden.
func (s *Server) r2RowPlaces(name string) (string, []string) {
	project := s.cat.RolePath(catalog.PlaceProject, name)
	shared := s.cat.RolePath(catalog.PlaceGlobal, name)
	_, projectErr := s.cat.Reader().Read(source.Doc{Path: project})
	_, sharedErr := s.cat.Reader().Read(source.Doc{Path: shared})
	hasProject, hasShared := projectErr == nil, sharedErr == nil
	switch {
	case hasProject && hasShared:
		return catalog.PlaceProject, []string{catalog.PlaceGlobal}
	case hasShared:
		return catalog.PlaceGlobal, nil
	}
	return catalog.PlaceProject, nil
}

// Check is POST /api/role/check: what is wrong with a panel's draft, or with the
// YAML a person is typing. The answer is the report as the boundary reads it.
func (k apiRole) Check(in apiIn) (any, *apiErr) {
	if len(in.Doc) == 0 {
		report := check.Role(check.RoleInput{Catalog: k.s.cat, Place: catalog.Layer(in.Place), Name: in.Name, Text: in.Text})
		return r2CheckAnswer(report, true), nil
	}
	var doc r2Doc
	if err := json.Unmarshal(in.Doc, &doc); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	original := in.Original
	if original == doc.Name {
		original = ""
	}
	return r2CheckAnswer(check.Role(check.RoleInput{
		Catalog: k.s.cat, Place: catalog.Layer(in.Place), Name: doc.Name,
		Draft: doc.document(), Original: original,
	}), false), nil
}

// r2CheckAnswer is one check report in the boundary's shape. fromText says the
// check read a whole text rather than a draft's fields, so every problem stands
// on its line.
func r2CheckAnswer(report check.DocReport, fromText bool) apiCheck {
	return apiCheck{
		Problems: r2Problems(report.Problems, fromText),
		Preview:  report.Markdown,
		Effect:   apiEffect{Diff: report.Diff, Turns: report.Turns},
		Readers:  report.Readers,
		Limit:    report.Limit,
		Size:     report.Markdown.Chars,
	}
}

// r2Problems is a report's problems in the boundary's shape (spec-boundary 5):
// the document's field each one belongs to is where a screen shows it, and a
// problem of a whole text, or of the document as a whole, stands on its line.
// field travels beside at so the screen stands a message under the box it names.
func r2Problems(problems []check.RoleProblem, fromText bool) []apiProblem {
	out := make([]apiProblem, 0, len(problems))
	for _, problem := range problems {
		at := "line"
		if problem.Field == "name" {
			// The name stands in the panel's own field, not on a line of the
			// document: the screen draws the sentence under 名前 (d-841e).
			at = "name"
		} else if !fromText && problem.Section >= 0 && problem.Rule >= 0 {
			at = fmt.Sprintf("sections.%d.rules.%d", problem.Section, problem.Rule)
			if problem.Field != "" {
				at += "." + problem.Field
			}
		}
		out = append(out, apiProblem{
			Sev: problem.Sev, At: at, Field: problem.Field, Text: problem.Text, Line: problem.Line,
			Section: problem.Section, Rule: problem.Rule, Definition: problem.Definition,
		})
	}
	return out
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
	if err := s.r2RolesDir(in.Place); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	target := s.cat.RolePath(in.Place, doc.Name)
	savedName := in.Original
	if savedName == "" {
		savedName = doc.Name
	}
	old := s.cat.RolePath(in.Place, savedName)
	if err := catalog.RoleNameProblem(doc.Name); err != nil {
		return nil, refuseAt(http.StatusUnprocessableEntity, "name", err.Error())
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
	var saved *rules.Document
	if baseErr == nil {
		saved, _ = rules.ReadAll(base, s.cat.Display(basePath))
		if saved == nil {
			saved = &rules.Document{Path: basePath}
		}
	}
	rules.FillOffDates(saved, draftDoc, time.Now().Format("2006-01-02"))
	if baseErr != nil {
		edited, err = emitNewRole(draftDoc)
	} else {
		edited, err = rules.Rewrite([]byte(base), saved, rules.Draft{
			Doc: draftDoc, From: doc.froms(), SectionFrom: doc.sectionFroms(), SaveFrom: rules.SaveFromRoles,
		})
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
	return k.savedAnswer(in.Place, doc.Name, r2DocOf(got, source.Hash(string(edited)), string(edited))), nil
}

// saveYAML writes one file's text as the person wrote it (the old
// /roles/yaml/save). It is refused only when a problem the saved document did
// not have has appeared (spec-roles X3).
func (k apiRole) saveYAML(in apiIn) (any, *apiErr) {
	s := k.s
	if err := s.r2RolesDir(in.Place); err != nil {
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
	return k.savedAnswer(in.Place, in.Name, r2DocOf(doc, source.Hash(in.Text), in.Text)), nil
}

// savedAnswer is the answer of a write: the one document as GET
// /api/role/{name} reads it now, with the whole list under it (spec-boundary 5).
func (k apiRole) savedAnswer(place, name string, doc *r2Doc) apiSaved {
	list := k.s.r2List()
	out := apiSaved{Doc: doc, Text: doc.Text, Hash: doc.Hash, Path: k.s.cat.Display(k.s.cat.RolePath(place, name)), Items: list.Items, Hashes: list.Hashes}
	out.Effective, out.Hidden = k.s.r2RowPlaces(name)
	return out
}

// Delete is DELETE /api/role/{name}?place=&hash=: take one role document out of
// its layer. Its answer is the list as it now reads.
func (k apiRole) Delete(name string, in apiIn) (any, *apiErr) {
	s := k.s
	place, hash := in.Place, in.Hash
	path := s.cat.RolePath(place, name)
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	pathHash := source.Hash("")
	if err == nil {
		pathHash = source.Hash(text)
	}
	if hash != pathHash {
		return nil, refuse(http.StatusConflict, tf("error.rules_file_changed", s.cat.Display(path)))
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, refuse(http.StatusInternalServerError, err.Error())
	}
	return k.List()
}
