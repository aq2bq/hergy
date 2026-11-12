package web

import (
	"encoding/json"
	"net/http"

	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/check"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/prose"
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
	Chars    int    `json:"chars"`
	Markdown string `json:"markdown,omitempty"`
	NoDoc    bool   `json:"noDoc,omitempty"`
	IsNew    bool   `json:"isNew,omitempty"`
}

// apiEffect is the effect of a document's change (spec-boundary 5): how the
// Markdown a person reads moved, and the arrows the rules write judged for each
// definition that reads them.
type apiEffect struct {
	Diff check.RoleDiff `json:"diff"`
}

// List is GET /api/role: one item per role name, with the documents behind it.
func (k apiRole) List() (any, *apiErr) {
	return k.s.r2List(), nil
}

// listLayers is List over the layers given: a headquarters group is one layer's
// rows, decided by the same owner the band asks (r-ea17).
func (k apiRole) listLayers(layers []string) apiList {
	return k.s.r2ListOf(k.s.r2ReadingOf(), layers)
}

// r2List is the role list answer: the rows the screen reads, as items.
func (s *Server) r2List() apiList {
	return s.r2ListOf(s.r2ReadingOf(), []string{source.LayerLocal, source.LayerGlobal})
}

// r2ListOf is the list answer of one reading, one row per place (r-ea17): a
// name both layers hold is two rows, and the rows are the owner's
// (ListedRows/ShownRows) in the owner's order.
func (s *Server) r2ListOf(reading r2Reading, layers []string) apiList {
	out := apiList{Hashes: map[string]string{}}
	rows := make([]r2Row, 0, len(reading.Roles))
	rows = append(rows, reading.Roles...)
	for _, layer := range layers {
		place := domainplace.Selection(layer).API()
		marks := map[string]catalog.ListedRow{}
		for _, one := range ops.ListResourceRows(s.cat, catalog.KindRole, layer) {
			marks[one.Name] = one
		}
		for _, row := range rows {
			doc := r2PlaceDoc(row, place)
			problem := row.ProjectProblem
			if place == catalog.PlaceGlobal {
				problem = row.GlobalProblem
			}
			if doc == nil && problem == "" {
				// A legacy .yaml is one row of the project's side, drawn from its
				// text alone (spec-roles 1.2.10).
				if row.Markdown == "" || layer != source.LayerLocal {
					continue
				}
				summary := r2SummaryOf(row, apiRoleSummary{Markdown: row.Markdown, NoDoc: row.NoDoc, IsNew: row.IsNew})
				out.Items = append(out.Items, apiItem{
					Name: r2RowName(row), Places: map[string]apiPlace{}, Effective: place, Summary: summary,
				})
				continue
			}
			summary := r2SummaryOf(row, apiRoleSummary{Markdown: row.Markdown, NoDoc: row.NoDoc, IsNew: row.IsNew})
			item := apiItem{
				Name: r2RowName(row), Places: map[string]apiPlace{}, Effective: place, Summary: summary,
			}
			if mark, ok := marks[item.Name]; ok {
				item.Mark = markWord(mark)
			}
			if problem != "" {
				badPath := s.cat.RolePath(place, item.Name)
				why, state := s.unreadable(badPath, problem)
				item.Places[place] = apiPlace{
					Doc: r2Doc{Name: item.Name}, Path: s.cat.Display(badPath),
					Problems: apiCounts{Err: 1}, Problem: why, State: state,
				}
				item.Problems.Err++
			} else {
				path := s.cat.RolePath(place, doc.Name)
				text, _ := s.cat.Reader().Read(source.Doc{Path: path})
				item.Places[place] = apiPlace{
					Doc: *doc, Text: text, Hash: source.Hash(text),
					Path: s.cat.Display(path), Problems: r2DocCounts(text),
				}
				out.Hashes[place+"."+doc.Name] = source.Hash(text)
				item.Problems = item.Places[place].Problems
			}
			out.Items = append(out.Items, item)
		}
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
	summary.Chars = (prose.Document{Text: doc.Text}).Chars()
	return summary
}

// r2DocCounts is one document's own problems, counted from its text.
func r2DocCounts(text string) apiCounts {
	var out apiCounts
	if (prose.Document{Text: text}).Chars() > check.RoleCharLimit {
		out.Warn = 1
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
	target, bad := s.resourceSeen(domainplace.Role, place, name)
	if bad != nil {
		return nil, bad
	}
	file, failure := s.resources().ReadResource(ops.ExistingResource(target), ops.ResourceMain)
	if failure != nil || !file.Exists {
		return nil, refuse(404, t("web.error.not_found"))
	}
	path, text := file.Path, file.Text
	effective, hidden := s.effectiveOf(catalog.KindRole, name)
	return map[string]any{
		"doc":       r2DocOf(text, path),
		"text":      text,
		"hash":      source.Hash(text),
		"path":      s.cat.Display(path),
		"effective": effective,
		"hidden":    hidden,
	}, nil
}

// Check is POST /api/role/check: what is wrong with a panel's draft, or with the
// YAML a person is typing. The answer is the report as the boundary reads it.
func (k apiRole) Check(in apiIn) (any, *apiErr) {
	if len(in.Doc) == 0 {
		result, _ := k.s.resources().CheckResource(ops.ResourceInspection{Kind: domainplace.Role, Place: in.Place.At(k.s.cat.Base()), Name: in.Name, Text: in.Text})
		report := result.Document
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
	result, _ := k.s.resources().CheckResource(ops.ResourceInspection{Kind: domainplace.Role, Place: in.Place.At(k.s.cat.Base()), Name: doc.Name, Text: doc.Text, Original: original})
	return r2CheckAnswer(result.Document, false), nil
}

// r2CheckAnswer is one check report in the boundary's shape. fromText says the
// check read a whole text rather than a draft's fields, so every problem stands
// on its line.
func r2CheckAnswer(report check.DocReport, fromText bool) apiCheck {
	// Empty is an empty list, never null: a screen draws a list it can walk
	// without asking whether this kind of check carries one (n-3f6c).
	readers := report.Readers
	if readers == nil {
		readers = []check.RoleReader{}
	}
	return apiCheck{
		Problems:     r2Problems(report.Problems, fromText),
		Preview:      report.Markdown,
		Effect:       apiEffect{Diff: report.Diff},
		Readers:      readers,
		Limit:        report.Limit,
		Size:         report.Markdown.Chars + report.TemplateChars,
		TemplateSize: report.TemplateChars,
	}
}

// r2Problems is a report's problems in the boundary's shape (spec-boundary 5):
// the document's field each one belongs to is where a screen shows it, and a
// problem of a whole text, or of the document as a whole, stands on its line.
// field travels beside at so the screen stands a message under the box it names.
func r2Problems(problems []check.RoleProblem, _ bool) []apiProblem {
	out := make([]apiProblem, 0, len(problems))
	for _, problem := range problems {
		at := "line"
		if problem.Field == "name" {
			// The name stands in the panel's own field, not on a line of the
			// document: the screen draws the sentence under 名前 (d-841e).
			at = "name"
		}
		out = append(out, apiProblem{
			Sev: problem.Sev, At: at, Field: problem.Field, Text: problem.Text, Line: problem.Line,
			Definition: problem.Definition,
		})
	}
	return out
}

// Save is PUT /api/role/{name}: a panel's draft, or a file's text when the
// request carries no doc. The name in the path is the row's; the doc is the one
// that says what to write.
func (k apiRole) Save(_ string, in apiIn) (any, *apiErr) {
	if refusal := k.s.roleContextStillCurrent(in); refusal != nil {
		return nil, refusal
	}
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
		return nil, refuse(422, err.Error())
	}
	ref, bad := s.resourceRef(domainplace.Role, in.Place.String(), doc.Name)
	if bad != nil {
		return nil, bad
	}
	edit := ops.ResourceEdit{Text: doc.Text, Hash: in.Hash, MissingHash: true, RenameUsers: in.RenameUsers, Create: in.Original == ""}
	if in.Original != "" {
		old, bad := s.resourceSeen(domainplace.Role, in.Place.String(), in.Original)
		if bad != nil {
			return nil, bad
		}
		edit.Original = &old
	}

	var saved ops.ResourceSaved
	var failure *ops.ResourceFailure
	switch {
	case edit.Original != nil && edit.Original.Name() != ref.Name():
		saved, failure = s.resources().RenameResource(*edit.Original, ref, edit)
	case in.Base != nil:
		saved, failure = s.resources().CopyResource(ref, edit)
	default:
		saved, failure = s.resources().SaveResource(ref, edit)
	}
	if failure != nil {
		return nil, s.resourceFailure(failure, ruleDoc(s.cat.Display(failure.Path)), false)
	}
	return k.savedAnswer(in.Place.String(), doc.Name, r2DocOf(saved.Text, saved.Path), saved.Text), nil
}

// saveYAML writes one file's text as the person wrote it (the old
// /roles/yaml/save). It is refused only when a problem the saved document did
// not have has appeared (spec-roles X3).
func (k apiRole) saveYAML(in apiIn) (any, *apiErr) {
	s := k.s
	ref, bad := s.resourceRef(domainplace.Role, in.Place.String(), in.Name)
	if bad != nil {
		return nil, bad
	}
	saved, failure := s.resources().SaveResource(ref, ops.ResourceEdit{Text: in.Text, Hash: in.Hash})
	if failure != nil {
		return nil, s.resourceFailure(failure, nil, false)
	}
	return k.savedAnswer(in.Place.String(), in.Name, r2DocOf(saved.Text, saved.Path), saved.Text), nil
}

// savedAnswer is the answer of a write: the one document as GET
// /api/role/{name} reads it now, with the whole list under it (spec-boundary 5).
func (k apiRole) savedAnswer(place, name string, doc *r2Doc, text string) apiSaved {
	list := k.s.r2List()
	out := apiSaved{Doc: doc, Text: text, Hash: source.Hash(text), Path: k.s.cat.Display(k.s.cat.RolePath(place, name)), Items: list.Items, Hashes: list.Hashes}
	out.Effective, out.Hidden = k.s.effectiveOf(catalog.KindRole, name)
	return out
}

// Delete is DELETE /api/role/{name}?place=&hash=: take one role document out of
// its layer. Its answer is the list as it now reads.
func (k apiRole) Delete(name string, in apiIn) (any, *apiErr) {
	target, bad := k.s.resourceSeen(domainplace.Role, in.Place.String(), name)
	if bad != nil {
		return nil, bad
	}
	if failure := k.s.resources().DeleteResource(ops.ExistingResource(target), ops.ResourceDelete{Hash: in.Hash}); failure != nil {
		return nil, k.s.resourceFailure(failure, nil, true)
	}
	return k.List()
}
