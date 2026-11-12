package web

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/catalog"
	domainplace "github.com/aq2bq/hergy/internal/domain/place"
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

// listLayers is List over the layers given: a headquarters group is one layer's
// rows, decided by the same owner the band asks (r-ea17).
func (k apiProfile) listLayers(layers []string) apiList {
	return k.s.p2ListLayers(layers)
}

// p2List is the profile list answer: one row per place, taken from the owner
// (ShownRows) so the rows are the same set and order the band counts (r-ea17).
// A name both files hold is two rows, and a file that cannot be read is the one
// row the owner returns for it.
func (s *Server) p2List() apiList {
	return s.p2ListLayers([]string{source.LayerLocal, source.LayerGlobal})
}

// p2ListLayers is the list over the layers given.
func (s *Server) p2ListLayers(layers []string) apiList {
	project, shared, defs := s.p2Read()
	chips := p2Chips(defs)
	all := []string{source.LayerLocal, source.LayerGlobal}
	out := apiList{Hashes: map[string]string{p2Project: project.hash, p2Shared: shared.hash}}
	for _, layer := range all {
		selected := domainplace.Selection(layer).API()
		if !slices.Contains(layers, layer) {
			continue
		}
		marks := map[string]catalog.ListedRow{}
		for _, listed := range ops.ListResourceRows(s.cat, catalog.KindProfile, layer) {
			marks[listed.Name] = listed
		}
		for _, listed := range ops.ListResourceRows(s.cat, catalog.KindProfile, layer) {
			if listed.Problem != "" || listed.Path == "" {
				why, state := s.unreadable(listed.Path, listed.Problem)
				out.Items = append(out.Items, apiItem{
					Name: listed.Name, Places: map[string]apiPlace{}, Effective: selected,
					Problems: apiCounts{Err: 1}, Summary: apiProfileSummary{},
				})
				item := &out.Items[len(out.Items)-1]
				item.Places[selected] = apiPlace{
					Doc: p2Draft{Name: listed.Name}, Path: s.cat.Display(listed.Path),
					Problems: apiCounts{Err: 1}, Problem: why, State: state,
				}
				continue
			}
			item := p2ItemOf(p2RowOf(listed.Name, project, shared, chips, false), project, shared)
			place, ok := item.Places[selected]
			if !ok {
				continue
			}
			item.Places = map[string]apiPlace{selected: place}
			item.Hidden = nil
			if mark, ok := marks[item.Name]; ok {
				item.Mark = markWord(mark)
			}
			item.Effective = selected
			item.Problems = place.Problems
			out.Items = append(out.Items, item)
		}
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
		it.Effective = panel.File
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
	if !file.ok && file.problem == "" {
		text = "version: 1\n"
	}
	_, problems := profile.ReadAll(text, s.cat.Display(file.path))
	if file.problem != "" {
		problems = append(problems, profile.Problem{Sev: profile.Err, Line: 1, Text: file.problem})
	}
	out := apiFile{Place: file.file, Label: file.label, Path: s.cat.Display(file.path), Text: text, Hash: file.hash, Problem: file.problem}
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
	display, ok := k.s.p2ProfilePath(place)
	if !ok {
		return nil, refuse(http.StatusNotFound, t("web.error.not_found"))
	}
	target, bad := k.s.resourceSeen(domainplace.Profile, place, name)
	if bad != nil {
		return nil, bad
	}
	file, failure := k.s.resources().ReadResource(ops.ExistingResource(target), ops.ResourceMain)
	if failure != nil {
		return nil, refuse(404, t("web.error.not_found"))
	}
	text, hash := file.Text, file.Hash
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
	if _, ok := k.s.p2ProfilePath(in.Place.String()); !ok {
		return nil, refuse(400, "unknown file")
	}
	query := ops.ResourceInspection{Kind: domainplace.Profile, Place: in.Place.At(k.s.cat.Base()), Text: in.Text, Original: in.Original}
	if len(in.Doc) > 0 {
		var draft p2Draft
		if err := json.Unmarshal(in.Doc, &draft); err != nil {
			return nil, refuse(422, i18n.Errorf("error.edit_yaml", "profile", err).Error())
		}
		value := draft.profileOf()
		query.Profile = &value
	}
	checked, failure := k.s.resources().CheckResource(query)
	if failure != nil {
		return nil, k.s.resourceFailure(failure, nil, false)
	}
	if query.Profile != nil {
		return apiCheck{Problems: p2DraftProblems(checked.Profiles)}, nil
	}
	var out []apiProblem
	for _, p := range p2Sorted(checked.Profiles) {
		out = append(out, apiProblem{Sev: p.Sev.Word(), At: "line", Text: p.Text, Line: p.Line})
	}
	if out == nil {
		out = []apiProblem{}
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
	if _, ok := s.p2ProfilePath(in.Place.String()); !ok {
		return nil, refuse(400, "unknown file")
	}
	var draft p2Draft
	if err := json.Unmarshal(in.Doc, &draft); err != nil {
		return nil, refuse(422, i18n.Errorf("error.edit_yaml", "profile", err).Error())
	}
	ref, bad := s.resourceRef(domainplace.Profile, in.Place.String(), draft.Name)
	if bad != nil {
		return nil, bad
	}
	value := draft.profileOf()
	edit := ops.ResourceEdit{Profile: &value, Hash: in.Hash, RenameUsers: in.RenameUsers}
	if in.Original != "" {
		old, bad := s.resourceSeen(domainplace.Profile, in.Place.String(), in.Original)
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
	_ = saved
	if failure != nil {
		return nil, s.resourceFailure(failure, profileDoc(draft.Name, s.cat.Display(failure.Path)), false)
	}
	return k.savedAnswer(in.Place.String(), draft.Name), nil
}

// saveYAML writes one file's text as the person wrote it (the old
// /profiles/yaml/save). Its answer is the file as the YAML tab reads it now,
// with the whole list under it.
func (k apiProfile) saveYAML(in apiIn) (any, *apiErr) {
	s := k.s
	if _, ok := s.p2ProfilePath(in.Place.String()); !ok {
		return nil, refuse(400, "unknown file")
	}
	saved, failure := s.resources().SaveProfilesFile(in.Place.At(s.cat.Base()), in.Hash, in.Text)
	if failure != nil {
		return nil, s.resourceFailure(failure, nil, false)
	}
	list := s.p2List()
	return apiSaved{Doc: p2FileDocOf(saved.Text, s.cat.Display(saved.Path)), Text: saved.Text, Hash: saved.Hash, Path: s.cat.Display(saved.Path), Items: list.Items, Hashes: list.Hashes, Files: list.Files}, nil
}

// Delete is DELETE /api/profile/{name}?place=&hash=: take one profile out of one
// file. Its answer is the list as it now reads.
func (k apiProfile) Delete(name string, in apiIn) (any, *apiErr) {
	s := k.s
	if _, ok := s.p2ProfilePath(in.Place.String()); !ok {
		return nil, refuse(400, "unknown file")
	}
	target, bad := s.resourceSeen(domainplace.Profile, in.Place.String(), name)
	if bad != nil {
		return nil, bad
	}
	if failure := s.resources().DeleteResource(ops.ExistingResource(target), ops.ResourceDelete{Hash: in.Hash}); failure != nil {
		return nil, s.resourceFailure(failure, nil, true)
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
