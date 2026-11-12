package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"

	"github.com/aq2bq/hergy/internal/ops"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the profile screen's answers (r-c683): the three tabs, the JSON
// checks a field sends while it is being typed, and the writes of
// profiles.yaml and team.yaml. Every write carries the hash of the text the
// page read, so a file that moved under the page is refused, never written
// over.

// p2Island is the JSON the profile pages carry (spec-boundary 5): the profile
// list in the boundary's shape, and, on the 割り当て tab, the assignment cards
// (the team kind stays in the shape that tab reads).
type p2Island struct {
	Profile apiList   `json:"profile"`
	Assign  *p2Assign `json:"assign,omitempty"`
	// HQ is the headquarters lists (d-73fc): the projects' groups of profiles.
	HQ *hqScreenIsland `json:"hq,omitempty"`
}

// p2Page answers one tab of the profile screen with the whole page. It is not
// drawn inside the old frame: this screen carries its own stylesheet and loads
// no other, so its look is the one the mock agreed on.
func (s *Server) p2Page(w http.ResponseWriter, r *http.Request, tab string) {
	scoped, refusal := s.withProjectInput(projectQuery(r))
	if refusal != nil {
		writeJSON(w, refusal.status, refusal)
		return
	}
	s = scoped
	data := s.p2Data(tab, r.URL.Query())
	island := p2Island{Profile: s.p2List(), HQ: s.hqScreenIsland(apiProfileKind)}
	if tab == "assign" {
		island.Assign = data.Assign
	}
	raw, err := json.Marshal(island)
	if err != nil {
		s.problem(w, r, err)
		return
	}
	data.Data = template.JS(raw) //nolint:gosec // raw is json.Marshal output, which escapes <, > and &, so it cannot close the script element that carries it
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, "p2_page", data); err != nil {
		s.problem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// p2RowCount is how many rows the profile list draws: the owner's rows for both
// layers, one row per place (r-ea17). A name both layers hold is two rows, so
// the list's own すべて count is the same number as the rows (its audit).
func (s *Server) p2RowCount() int {
	return len(ops.ListResourceRows(s.cat, catalog.KindProfile, source.LayerLocal)) +
		len(ops.ListResourceRows(s.cat, catalog.KindProfile, source.LayerGlobal))
}

// p2Data gathers one tab.
func (s *Server) p2Data(tab string, q url.Values) p2Data {
	project, shared, defs := s.p2Read()
	data := p2Data{
		Lang: i18n.Language(), Nav: s.navOf("profiles"), Tab: tab, Token: s.token,
		ProfileCount: s.p2RowCount(), AssignCount: p2People(defs),
		HashProject: project.hash, HashShared: shared.hash,
		PathProject: s.cat.Display(s.cat.ProjectProfileFile()),
		PathShared:  s.cat.Display(s.cat.SharedProfileFile()),
		OtherKinds:  p2JSON(otherKinds("")),
	}
	switch tab {
	case "assign":
		data = s.p2AssignData(data, project, shared, defs, q)
	case "yaml":
		data = s.p2YamlData(data, project, shared)
	default:
		data = s.p2ProfilesData(data, project, shared, defs, q)
	}
	return data
}

// p2ProfilePath is one place's profiles.yaml and how a person reads it.
func (s *Server) p2ProfilePath(file string) (string, bool) {
	switch file {
	case p2Project:
		path := s.cat.ProjectProfileFile()
		return s.cat.Display(path), true
	case p2Shared:
		path := s.cat.SharedProfileFile()
		return s.cat.Display(path), true
	default:
		return "", false
	}
}

// p2Draft is the form's profile.
type p2Draft struct {
	Name string      `json:"name"`
	Kind string      `json:"kind"`
	Args []string    `json:"args"`
	Env  []p2EnvJSON `json:"env"`
}

// profileOf is the draft in the profile package's words.
func (d p2Draft) profileOf() profile.Draft {
	env := make([]profile.EnvPair, 0, len(d.Env))
	for _, pair := range d.Env {
		env = append(env, profile.EnvPair{Key: pair.Key, Value: pair.Value})
	}
	return profile.Draft{Name: d.Name, Kind: d.Kind, Args: d.Args, Env: env}
}

// p2FileDoc is one profiles.yaml as a save answers it: the profiles the file
// holds, in the order the file writes them. A save writes a file, so its answer
// carries the file's document, not one profile of it (n-3f6c).
type p2FileDoc struct {
	Profiles []p2Draft `json:"profiles"`
}

// p2FileDocOf reads a saved file into the document a save answers with. A file
// that does not read has no profile to name, and the answer still carries the
// document with none in it.
func p2FileDocOf(text, display string) p2FileDoc {
	entries, _ := profile.ReadAll(text, display)
	out := p2FileDoc{Profiles: make([]p2Draft, 0, len(entries))}
	for _, entry := range entries {
		env := make([]p2EnvJSON, 0, len(entry.Env))
		for _, pair := range entry.Env {
			env = append(env, p2EnvJSON{Key: pair.Key, Value: pair.Value})
		}
		out.Profiles = append(out.Profiles, p2Draft{Name: entry.Name, Kind: entry.Kind, Args: entry.Args, Env: env})
	}
	return out
}

// p2FindEntry is one name's entry in a file.
func p2FindEntry(entries []profile.Entry, name string) (profile.Entry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return profile.Entry{}, false
}

// oldProfile moves a profile's old page link to the row it names, opened: the
// old screen had a page per profile, and this one opens the row in the list.
func (s *Server) oldProfile(w http.ResponseWriter, r *http.Request) {
	target := "/profiles?open=" + url.QueryEscape(r.PathValue("name"))
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}
