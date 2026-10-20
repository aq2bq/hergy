package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// apiProfile is the profile kind: the two profiles.yaml files behind the profile
// screen, as List / Get / Check / Save / Delete (spec-boundary 8). Every method
// lays out what the model's own reading returned; none of them decides a rule.
type apiProfile struct{ s *Server }

// List is GET /api/profile: the profiles tab as it now reads.
func (k apiProfile) List() (any, *apiErr) {
	return k.s.p2Data("profiles", url.Values{}), nil
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
		return map[string]any{"problems": p2ProblemsJSON(problems)}, nil
	}
	_, problems := profile.ReadAll(in.Text, display)
	out := make([]map[string]any, 0, len(problems))
	for _, problem := range p2Sorted(problems) {
		out = append(out, map[string]any{"sev": problem.Sev.Word(), "line": problem.Line, "text": problem.Text})
	}
	return map[string]any{"problems": out}, nil
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
	if in.Hash != hash {
		return nil, refuse(http.StatusConflict, tf("error.profiles_file_changed", display))
	}
	entries, _ := profile.ReadAll(text, display)
	edited, err := p2EditProfile(text, display, entries, draft.profileOf(), in.Original)
	if err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	if problem, bad := p2FirstError(edited, display); bad {
		return nil, refuse(http.StatusUnprocessableEntity, problem)
	}
	if err := source.WriteIfUnchanged(path, hash, edited); err != nil {
		return p2WriteError(err, display)
	}
	if in.Original != "" && in.Original != draft.Name && in.RenameUsers {
		s.p2RenameUsers(in.Place, in.Original, draft.Name)
	}
	return k.savedList(in.Open, draft.Name, in.Place), nil
}

// saveYAML writes one file's text as the person wrote it (the old
// /profiles/yaml/save).
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
		return p2WriteError(err, display)
	}
	project, shared, _ := s.p2Read()
	return map[string]any{
		"hash":  source.Hash(in.Text),
		"files": s.p2YamlData(p2Data{}, project, shared).Files,
	}, nil
}

// Delete is DELETE /api/profile/{name}?place=&hash=: take one profile out of one
// file.
func (k apiProfile) Delete(name, place, hash string) (any, *apiErr) {
	s := k.s
	path, display, ok := s.p2ProfilePath(place)
	if !ok {
		return nil, refuse(http.StatusBadRequest, "unknown file")
	}
	if err := s.p2Write(path); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, err.Error())
	}
	text, pathHash := s.p2Text(path)
	if hash != pathHash {
		return nil, refuse(http.StatusConflict, tf("error.profiles_file_changed", display))
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
		return p2WriteError(err, display)
	}
	return k.savedList(nil, "", ""), nil
}

// savedList is the list a save answers with: every row as it now reads, so a
// rename that splits one row into two answers with both in the order a reload
// shows them (audit N1). The names the page has open stay open, and the row that
// was written is open at its panel with its saved line.
func (k apiProfile) savedList(open []string, savedName, file string) p2Data {
	q := url.Values{}
	if len(open) > 0 {
		q.Set("open", strings.Join(open, ","))
	}
	data := k.s.p2Data("profiles", q)
	if savedName == "" {
		return data
	}
	for i := range data.Rows {
		if data.Rows[i].Name != savedName {
			continue
		}
		data.Rows[i].Open = true
		for j := range data.Rows[i].Panels {
			if data.Rows[i].Panels[j].File == file {
				data.Rows[i].Panels[j].SavedLine = true
			}
		}
	}
	return data
}
