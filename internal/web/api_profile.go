package web

import (
	"encoding/json"
	"net/http"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// apiProfile is the profile kind: the two profiles.yaml files behind the profile
// screen, as List / Get / Check / Save / Delete (spec-boundary 8). Every method
// lays out what the model's own reading returned; none of them decides a rule.
type apiProfile struct{ s *Server }

// apiProfileSummary is the profile list's columns: the kind a profile runs and
// the words it runs with, and the people that name it (spec-boundary 5).
type apiProfileSummary struct {
	Kind      string       `json:"kind"`
	KindClass string       `json:"kindClass"`
	Args      string       `json:"args"`
	Env       []string     `json:"env"`
	More      int          `json:"more"`
	Users     int          `json:"users"`
	Roles     string       `json:"roles"`
	Chips     []p2UserChip `json:"chips"`
}

// List is GET /api/profile: both files' profiles as items, the files' hashes,
// and the files themselves for the YAML tab.
func (k apiProfile) List() (any, *apiErr) {
	return k.s.p2List(), nil
}

// p2List is the profile list answer: one item per name, the two files' hashes,
// and the two files' own readings.
func (s *Server) p2List() apiList {
	project, shared, defs := s.p2Read()
	chips := p2Chips(defs)
	out := apiList{Hashes: map[string]string{p2Project: project.hash, p2Shared: shared.hash}}
	for _, name := range p2Names(project, shared) {
		out.Items = append(out.Items, p2ItemOf(p2RowOf(name, project, shared, chips, false), project, shared))
	}
	for _, file := range []p2File{project, shared} {
		out.Files = append(out.Files, s.p2FileOf(file))
	}
	return out
}

// p2ItemOf is one row as the list carries it: each panel becomes the item's
// reading of one place, and the row's columns its summary.
func p2ItemOf(row p2Row, project, shared p2File) apiItem {
	texts := map[string]string{p2Project: project.text, p2Shared: shared.text}
	it := apiItem{
		Name:     row.Name,
		Places:   map[string]apiPlace{},
		Problems: apiCounts{Err: row.Err, Warn: row.Warn},
		Summary: apiProfileSummary{
			Kind: row.Kind, KindClass: row.KindClass, Args: row.Args, Env: row.Env,
			More: row.More, Users: row.Users, Roles: row.Roles, Chips: row.Chips,
		},
	}
	for _, panel := range row.Panels {
		if panel.Shadowed {
			it.Hidden = append(it.Hidden, panel.File)
		} else {
			it.Effective = panel.File
		}
		it.Places[panel.File] = apiPlace{
			Doc:           p2Draft{Name: panel.Name, Kind: panel.Kind, Args: panel.Args, Env: p2EnvPairs(panel.Env)},
			Text:          texts[panel.File],
			Hash:          panel.Hash,
			Path:          panel.Path,
			RenameTargets: panel.RenameUsers,
			Problems:      apiCounts{Err: panel.Err, Warn: panel.Warn},
		}
	}
	return it
}

// p2FileOf is one profiles.yaml as the YAML tab reads it: the whole text and the
// problems of that text. A file that is not there reads as the empty file a save
// would make.
func (s *Server) p2FileOf(file p2File) apiFile {
	text := file.text
	if !file.ok {
		text = "version: 1\n"
	}
	_, problems := profile.ReadAll(text, s.cat.Display(file.path))
	out := apiFile{Place: file.file, Label: file.label, Path: s.cat.Display(file.path), Text: text, Hash: file.hash}
	for _, problem := range p2Sorted(problems) {
		out.Problems = append(out.Problems, apiProblem{Sev: problem.Sev.Word(), At: "line", Text: problem.Text, Line: problem.Line})
	}
	return out
}

// apiProfileGet is one profile in one place, the answer of GET
// /api/profile/{name}.
type apiProfileGet struct {
	Doc       p2Draft  `json:"doc"`
	Text      string   `json:"text"`
	Hash      string   `json:"hash"`
	Path      string   `json:"path"`
	Effective string   `json:"effective"`
	Hidden    []string `json:"hidden"`
}

// Get is GET /api/profile/{name}?place=: one file's document for one name.
func (k apiProfile) Get(name, place string) (any, *apiErr) {
	path, display, ok := k.s.p2ProfilePath(place)
	if !ok {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	text, hash := k.s.p2Text(path)
	entries, _ := profile.ReadAll(text, display)
	entry, found := p2FindEntry(entries, name)
	if !found {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	effective, hidden := k.s.p2Effective(name, place)
	return apiProfileGet{
		Doc:       p2DocOf(entry),
		Text:      text,
		Hash:      hash,
		Path:      display,
		Effective: effective,
		Hidden:    hidden,
	}, nil
}

// p2Effective is which place is in effect for one name and which of them is
// hidden, the same words a row's pills carry.
func (s *Server) p2Effective(name, place string) (string, []string) {
	project, shared, _ := s.p2Read()
	_, inProject := p2Find(project, name)
	_, inShared := p2Find(shared, name)
	switch {
	case inProject && inShared:
		return p2Project, []string{p2Shared}
	case inProject:
		return p2Project, nil
	case inShared:
		return p2Shared, nil
	}
	return place, nil
}

// p2DocOf is one file entry as the screen's draft.
func p2DocOf(entry profile.Entry) p2Draft {
	return p2Draft{Name: entry.Name, Kind: entry.Kind, Args: entry.Args, Env: p2EnvPairs(entry.Env)}
}

// Check is POST /api/profile/check: what is wrong with a panel's draft or with a
// file's YAML text.
func (k apiProfile) Check(in apiIn) (any, *apiErr) {
	_, display, ok := k.s.p2ProfilePath(in.Place)
	if !ok {
		return nil, refuse(http.StatusBadRequest, "unknown file")
	}
	if len(in.Doc) > 0 {
		var draft p2Draft
		if err := json.Unmarshal(in.Doc, &draft); err != nil {
			return nil, refuse(http.StatusUnprocessableEntity, i18n.Errorf("error.edit_yaml", "profile", err).Error())
		}
		text, _ := k.s.p2Text(k.pathOf(in.Place))
		entries, _ := profile.ReadAll(text, display)
		problems := profile.CheckDraft(draft.profileOf(), entries, in.Original)
		return apiCheck{Problems: p2DraftProblems(problems)}, nil
	}
	_, problems := profile.ReadAll(in.Text, display)
	out := make([]apiProblem, 0, len(problems))
	for _, problem := range p2Sorted(problems) {
		out = append(out, apiProblem{Sev: problem.Sev.Word(), At: "line", Text: problem.Text, Line: problem.Line})
	}
	return apiCheck{Problems: out}, nil
}

// p2DraftProblems is a draft's problems in the boundary's shape: the field each
// one belongs to is the place it is shown.
func p2DraftProblems(problems []profile.Problem) []apiProblem {
	out := make([]apiProblem, 0, len(problems))
	for _, problem := range problems {
		at := problem.Field
		if at == "" {
			at = "line"
		}
		out = append(out, apiProblem{Sev: problem.Sev.Word(), At: at, Field: problem.Field, Text: problem.Text, Line: problem.Line})
	}
	return out
}

// pathOf is one place's profiles.yaml.
func (k apiProfile) pathOf(place string) string {
	path, _, _ := k.s.p2ProfilePath(place)
	return path
}

// Save is PUT /api/profile/{name}: a panel's draft, or a file's text when the
// request carries one. The name in the path is the row's; the doc is the one
// that says what to write, so it is also the one that names the file's place.
func (k apiProfile) Save(_ string, in apiIn) (any, *apiErr) {
	if len(in.Doc) == 0 {
		return k.saveYAML(in)
	}
	return k.savePanel(in)
}

// savePanel writes one panel of the profile tab (the old /profiles/save).
func (k apiProfile) savePanel(in apiIn) (any, *apiErr) {
	s := k.s
	path, display, ok := s.p2ProfilePath(in.Place)
	if !ok {
		return nil, refuse(http.StatusBadRequest, "unknown file")
	}
	if err := s.p2Write(path); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	var draft p2Draft
	if err := json.Unmarshal(in.Doc, &draft); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, i18n.Errorf("error.edit_yaml", "profile", err).Error())
	}
	text, hash := s.p2Text(path)
	entries, _ := profile.ReadAll(text, display)
	edited, err := p2EditProfile(text, display, entries, draft.profileOf(), in.Original)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	// The file moved under the screen: the save is refused, and the answer
	// carries the file as it is now and the text the panel would write over it
	// (r-3c77, d-a34a).
	if in.Hash != hash {
		return nil, s.refuseChanged(display, path, profileDoc(draft.Name, display), false)
	}
	if problem, bad := p2FirstError(edited, display); bad {
		return nil, refuse(http.StatusUnprocessableEntity, problem)
	}
	if err := source.WriteIfUnchanged(path, hash, edited); err != nil {
		return k.s.p2WriteError(err, display, path, profileDoc(draft.Name, display))
	}
	if in.Original != "" && in.Original != draft.Name && in.RenameUsers {
		s.p2RenameUsers(in.Place, in.Original, draft.Name)
	}
	return k.savedAnswer(in.Place, draft.Name), nil
}

// saveYAML writes one file's text as the person wrote it (the old
// /profiles/yaml/save). Its answer is the file as the YAML tab reads it now,
// with the whole list under it.
func (k apiProfile) saveYAML(in apiIn) (any, *apiErr) {
	s := k.s
	path, display, ok := s.p2ProfilePath(in.Place)
	if !ok {
		return nil, refuse(http.StatusBadRequest, "unknown file")
	}
	if err := s.p2Write(path); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	if problem, bad := p2FirstError(in.Text, display); bad {
		return nil, refuse(http.StatusUnprocessableEntity, problem)
	}
	if err := source.WriteIfUnchanged(path, in.Hash, in.Text); err != nil {
		return k.s.p2WriteError(err, display, path, nil)
	}
	list := k.s.p2List()
	return apiSaved{
		Doc:  p2FileDocOf(in.Text, display),
		Text: in.Text, Hash: source.Hash(in.Text), Path: display,
		Items: list.Items, Hashes: list.Hashes, Files: list.Files,
	}, nil
}

// Delete is DELETE /api/profile/{name}?place=&hash=: take one profile out of one
// file. Its answer is the list as it now reads.
func (k apiProfile) Delete(name string, in apiIn) (any, *apiErr) {
	s := k.s
	place, hash := in.Place, in.Hash
	path, display, ok := s.p2ProfilePath(place)
	if !ok {
		return nil, refuse(http.StatusBadRequest, "unknown file")
	}
	if err := s.p2Write(path); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	text, pathHash := s.p2Text(path)
	if hash != pathHash {
		return nil, s.refuseChanged(display, path, nil, true)
	}
	entries, _ := profile.ReadAll(text, display)
	if _, found := p2FindEntry(entries, name); !found {
		return nil, refuse(http.StatusUnprocessableEntity, tf("error.edit_no_profile", name))
	}
	edited, err := profile.EditYAML([]byte(text), display, profile.Remove(name))
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	if err := source.WriteIfUnchanged(path, pathHash, string(edited)); err != nil {
		return k.s.p2WriteError(err, display, path, nil)
	}
	return k.listAnswer(), nil
}

// listAnswer is the whole list, the answer a delete and a YAML save carry
// because the counts move with them (spec-boundary 5).
func (k apiProfile) listAnswer() apiList {
	return k.s.p2List()
}

// savedAnswer is the answer of a panel's save: the one profile as GET
// /api/profile/{name} reads it now, with the whole list under it and the written
// panel marked as just saved, so the row says 保存しました.
func (k apiProfile) savedAnswer(place, name string) apiSaved {
	list := k.s.p2List()
	out := apiSaved{Items: list.Items, Hashes: list.Hashes, Files: list.Files}
	if one, refusal := k.Get(name, place); refusal == nil {
		if got, ok := one.(apiProfileGet); ok {
			out.Doc = p2FileDocOf(got.Text, got.Path)
			out.Text, out.Hash, out.Path = got.Text, got.Hash, got.Path
			out.Effective, out.Hidden = got.Effective, got.Hidden
		}
	}
	for i := range out.Items {
		if out.Items[i].Name != name {
			continue
		}
		if panel, ok := out.Items[i].Places[place]; ok {
			panel.SavedLine = true
			out.Items[i].Places[place] = panel
		}
	}
	return out
}
