package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// This file is the profile screen's answers (r-c683): the three tabs, the JSON
// checks a field sends while it is being typed, and the writes of
// profiles.yaml and team.yaml. Every write carries the hash of the text the
// page read, so a file that moved under the page is refused, never written
// over.

// p2Page answers one tab of the profile screen with the whole page. It is not
// drawn inside the old frame: this screen carries its own stylesheet and loads
// no other, so its look is the one the mock agreed on.
func (s *Server) p2Page(w http.ResponseWriter, r *http.Request, tab string) {
	data := s.p2Data(tab, r.URL.Query())
	raw, err := json.Marshal(data)
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

// p2Data gathers one tab.
func (s *Server) p2Data(tab string, q url.Values) p2Data {
	project, shared, defs := s.p2Read()
	data := p2Data{
		Lang: i18n.Language(), Nav: s.navOf("profiles"), Tab: tab, Token: s.token,
		ProfileCount: len(p2Names(project, shared)), AssignCount: p2People(defs),
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
func (s *Server) p2ProfilePath(file string) (string, string, bool) {
	switch file {
	case p2Project:
		path := s.cat.ProjectProfileFile()
		return path, s.cat.Display(path), true
	case p2Shared:
		path := s.cat.SharedProfileFile()
		return path, s.cat.Display(path), true
	default:
		return "", "", false
	}
}

// p2Text is a file's text and hash as a write compares them. A file that is not
// there is empty: it is being made.
func (s *Server) p2Text(path string) (string, string) {
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return "", source.Hash("")
	}
	return text, source.Hash(text)
}

// p2Write stops a write that would make a layer that is not there: a project
// whose .hergy is gone has no place to put a profiles.yaml.
func (s *Server) p2Write(path string) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		return i18n.Errorf("error.profiles_no_layer", s.cat.Display(dir))
	}
	return nil
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

// p2ProblemsJSON is problems as the page reads them.
func p2ProblemsJSON(problems []profile.Problem) []map[string]any {
	out := make([]map[string]any, 0, len(problems))
	for _, problem := range problems {
		out = append(out, map[string]any{"sev": problem.Sev.Word(), "field": problem.Field, "text": problem.Text})
	}
	return out
}

// p2EditProfile is the ops one panel's save writes: the name, kind, args and env
// that changed, and every one of them for a profile being made.
func p2EditProfile(text, display string, entries []profile.Entry, draft profile.Draft, original string) (string, error) {
	if strings.TrimSpace(text) == "" {
		text = "version: 1\n"
	}
	ops, err := p2ProfileOps(entries, draft, original)
	if err != nil {
		return "", err
	}
	out := text
	for _, op := range ops {
		edited, err := profile.EditYAML([]byte(out), display, op)
		if err != nil {
			return "", err
		}
		out = string(edited)
	}
	return out, nil
}

// p2ProfileOps is the ops a panel's save means.
func p2ProfileOps(entries []profile.Entry, draft profile.Draft, original string) ([]profile.Op, error) {
	saved, found := p2FindEntry(entries, original)
	switch {
	case original == "":
		ops := []profile.Op{profile.Add(draft.Name, draft.Kind)}
		if len(draft.Args) > 0 {
			ops = append(ops, profile.SetArgs(draft.Name, draft.Args))
		}
		if len(draft.Env) > 0 {
			ops = append(ops, profile.SetEnvInOrder(draft.Name, draft.Env))
		}
		return ops, nil
	case !found:
		return nil, i18n.Errorf("error.edit_no_profile", original)
	}
	var ops []profile.Op
	name := original
	if draft.Name != original {
		ops = append(ops, profile.Rename(original, draft.Name))
		name = draft.Name
	}
	if draft.Kind != saved.Kind {
		ops = append(ops, profile.SetKind(name, draft.Kind))
	}
	if !slices.Equal(draft.Args, saved.Args) {
		ops = append(ops, profile.SetArgs(name, draft.Args))
	}
	if !p2SameEnv(draft.Env, saved.Env) {
		ops = append(ops, profile.SetEnvInOrder(name, draft.Env))
	}
	return ops, nil
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

// p2SameEnv reports whether the pairs and the file's env are the same values,
// whatever order the screen shows them in.
func p2SameEnv(pairs []profile.EnvPair, saved []profile.EnvPair) bool {
	if len(pairs) != len(saved) {
		return false
	}
	want := make(map[string]string, len(saved))
	for _, pair := range saved {
		want[pair.Key] = pair.Value
	}
	for _, pair := range pairs {
		if value, ok := want[pair.Key]; !ok || value != pair.Value {
			return false
		}
	}
	return true
}

// p2FirstError is the first problem that stops a text from reading.
func p2FirstError(text, display string) (string, bool) {
	_, problems := profile.ReadAll(text, display)
	for _, problem := range problems {
		if problem.Sev == profile.Err {
			return problem.Text, true
		}
	}
	return "", false
}

// p2WriteError turns a write's failure into the refusal a person reads.
func p2WriteError(err error, display string) (any, *apiErr) {
	if errors.Is(err, source.ErrChanged) {
		return nil, refuse(http.StatusConflict, tf("error.profiles_file_changed", display))
	}
	return nil, refuse(http.StatusInternalServerError, err.Error())
}

// p2RenameUsers writes the new name into every definition that reads the file
// whose profile was renamed, so the people that used it keep a profile that is
// there.
func (s *Server) p2RenameUsers(file, old, next string) {
	project, _, defs := s.p2Read()
	for _, def := range defs {
		if !p2ReadsFile(def, file, project, old) {
			continue
		}
		text, err := s.cat.Reader().Read(source.Doc{Path: def.path})
		if err != nil {
			continue
		}
		edited := p2RenameInTeam(text, def, old, next, s.cat.Display(def.path))
		if edited != text {
			_ = source.WriteBeside(def.path, edited)
		}
	}
}

// p2ReadsFile reports whether a definition names the profile of the file being
// written: a project definition reads the project's file, and a definition that
// the project's own file hides reads only the shared file.
func p2ReadsFile(def p2Def, file string, project p2File, name string) bool {
	if file == p2Project {
		return def.local
	}
	if !def.local {
		return true
	}
	_, shadowed := p2Find(project, name)
	return !shadowed
}

// p2RenameInTeam writes one name over another in the members and workers that
// name it, leaving every other line as it was.
func p2RenameInTeam(text string, def p2Def, old, next, display string) string {
	out := text
	for _, member := range def.members {
		if member.Profile != old {
			continue
		}
		if edited, err := definition.EditYAML([]byte(out), display, definition.MemberProfile(member.Name, next)); err == nil {
			out = string(edited)
		}
	}
	for _, worker := range def.workers {
		if worker.Profile != old {
			continue
		}
		if edited, err := definition.EditYAML([]byte(out), display, definition.WorkerProfile(worker.From, worker.Purpose, next)); err == nil {
			out = string(edited)
		}
	}
	return out
}

// p2AssignChange is one change the assign tab sends.
type p2AssignChange struct {
	Member  string `json:"member"`
	From    string `json:"from"`
	Purpose string `json:"purpose"`
	Profile string `json:"profile"`
}

// p2FindDef is one definition, freshly read.
func (s *Server) p2FindDef(name string) (p2Def, bool) {
	for _, def := range s.p2Defs() {
		if def.name == name {
			return def, true
		}
	}
	return p2Def{}, false
}

// p2MissingProfile is the sentence a save is refused with when a picked profile
// is not one the definition reads.
func (s *Server) p2MissingProfile(def p2Def, changes []p2AssignChange) (string, bool) {
	project, shared, _ := s.p2Read()
	_, kinds := p2Resolved(def, project, shared)
	members := map[string]int{}
	for i, member := range def.members {
		members[member.Name] = i
	}
	workers := map[string]int{}
	for i, worker := range def.workers {
		workers[worker.From+"\x00"+worker.Purpose] = i
	}
	for _, change := range changes {
		if _, ok := kinds[change.Profile]; ok {
			continue
		}
		if change.Member != "" {
			return tf("error.no_profile", fmt.Sprintf("members[%d]", members[change.Member]), change.Profile), true
		}
		return tf("error.workers_no_profile", fmt.Sprintf("workers[%d]", workers[change.From+"\x00"+change.Purpose]), change.Profile), true
	}
	return "", false
}

// oldProfile moves a profile's old page link to the row it names, opened: the
// old screen had a page per profile, and this one opens the row in the list.
func (s *Server) oldProfile(w http.ResponseWriter, r *http.Request) {
	target := "/profiles?open=" + url.QueryEscape(r.PathValue("name"))
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}
