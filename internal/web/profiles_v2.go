package web

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
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

// p2Answer is one POST's answer: a status, a content type and a body.
type p2Answer struct {
	status  int
	ctype   string
	body    string
	headers map[string]string
}

// p2JSONAnswer is a JSON answer.
func p2JSONAnswer(status int, value any) p2Answer {
	raw, err := json.Marshal(value)
	if err != nil {
		return p2TextAnswer(http.StatusInternalServerError, err.Error())
	}
	return p2Answer{status: status, ctype: "application/json; charset=utf-8", body: string(raw)}
}

// p2TextAnswer is a refusal: the sentence a person reads, and a status.
func p2TextAnswer(status int, text string) p2Answer {
	return p2Answer{status: status, ctype: "application/json; charset=utf-8", body: p2JSON(map[string]string{"text": text})}
}

// p2HTMLAnswer is a fragment the page puts in place of what it sent.
func p2HTMLAnswer(html string, project, shared p2File, defs []p2Def) p2Answer {
	return p2Answer{
		status: http.StatusOK, ctype: "text/html; charset=utf-8", body: html,
		headers: map[string]string{
			"X-Hergy-Profiles":     fmt.Sprintf("%d", len(p2Names(project, shared))),
			"X-Hergy-Assign":       fmt.Sprintf("%d", p2People(defs)),
			"X-Hergy-Hash-Project": project.hash,
			"X-Hergy-Hash-Shared":  shared.hash,
		},
	}
}

// p2Post runs one write: the Origin of a request that sends one has to be this
// server, the body is capped, and the token of this run has to be on the form.
func (s *Server) p2Post(fn func(*http.Request) p2Answer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			http.Error(w, t("web.error.origin"), http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxWrite)
		if err := r.ParseForm(); err != nil {
			http.Error(w, t("web.error.token"), http.StatusForbidden)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.PostFormValue("token")), []byte(s.token)) != 1 {
			http.Error(w, t("web.error.token"), http.StatusForbidden)
			return
		}
		answer := fn(r)
		for key, value := range answer.headers {
			w.Header().Set(key, value)
		}
		w.Header().Set("Content-Type", answer.ctype)
		w.WriteHeader(answer.status)
		_, _ = w.Write([]byte(answer.body))
	}
}

// p2Page answers one tab of the profile screen with the whole page. It is not
// drawn inside the old frame: this screen carries its own stylesheet and loads
// no other, so its look is the one the mock agreed on.
func (s *Server) p2Page(w http.ResponseWriter, r *http.Request, tab string) {
	data := s.p2Data(r, tab)
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, "p2_page", data); err != nil {
		s.problem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// p2Data gathers one tab.
func (s *Server) p2Data(r *http.Request, tab string) p2Data {
	project, shared, defs := s.p2Read()
	data := p2Data{
		Lang: i18n.Language(), Project: filepath.Base(s.cat.Base()), Tab: tab, Token: s.token,
		ProfileCount: len(p2Names(project, shared)), AssignCount: p2People(defs),
		HashProject: project.hash, HashShared: shared.hash,
		PathProject: s.cat.Display(s.cat.ProjectProfileFile()),
		PathShared:  s.cat.Display(s.cat.SharedProfileFile()),
		OtherKinds:  p2JSON(otherKinds("")),
	}
	switch tab {
	case "assign":
		data = s.p2AssignData(data, project, shared, defs, r.URL.Query())
	case "yaml":
		data = s.p2YamlData(data, project, shared)
	default:
		data = s.p2ProfilesData(data, project, shared, defs, r.URL.Query())
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

// p2DraftOf reads the form's profile.
func p2DraftOf(r *http.Request) (p2Draft, error) {
	var draft p2Draft
	if err := json.Unmarshal([]byte(r.PostFormValue("profile")), &draft); err != nil {
		return p2Draft{}, i18n.Errorf("error.edit_yaml", "profile", err)
	}
	return draft, nil
}

// p2Check answers what is wrong with the profile a field is editing.
func (s *Server) p2Check(r *http.Request) p2Answer {
	file := r.PostFormValue("file")
	path, display, ok := s.p2ProfilePath(file)
	if !ok {
		return p2TextAnswer(http.StatusBadRequest, "unknown file")
	}
	draft, err := p2DraftOf(r)
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	text, _ := s.p2Text(path)
	entries, _ := profile.ReadAll(text, display)
	problems := profile.CheckDraft(draft.profileOf(), entries, r.PostFormValue("original"))
	return p2JSONAnswer(http.StatusOK, map[string]any{"problems": p2ProblemsJSON(problems)})
}

// p2ProblemsJSON is problems as the page reads them.
func p2ProblemsJSON(problems []profile.Problem) []map[string]any {
	out := make([]map[string]any, 0, len(problems))
	for _, problem := range problems {
		out = append(out, map[string]any{"sev": problem.Sev.Word(), "field": problem.Field, "text": problem.Text})
	}
	return out
}

// p2YAMLCheck answers what is wrong with the YAML a person is typing.
func (s *Server) p2YAMLCheck(r *http.Request) p2Answer {
	file := r.PostFormValue("file")
	_, display, ok := s.p2ProfilePath(file)
	if !ok {
		return p2TextAnswer(http.StatusBadRequest, "unknown file")
	}
	_, problems := profile.ReadAll(r.PostFormValue("text"), display)
	out := make([]map[string]any, 0, len(problems))
	for _, problem := range p2Sorted(problems) {
		out = append(out, map[string]any{"sev": problem.Sev.Word(), "line": problem.Line, "text": problem.Text})
	}
	return p2JSONAnswer(http.StatusOK, map[string]any{"problems": out})
}

// p2Save writes one panel of the profile tab.
func (s *Server) p2Save(r *http.Request) p2Answer {
	file := r.PostFormValue("file")
	path, display, ok := s.p2ProfilePath(file)
	if !ok {
		return p2TextAnswer(http.StatusBadRequest, "unknown file")
	}
	if err := s.p2Write(path); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	draft, err := p2DraftOf(r)
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	text, hash := s.p2Text(path)
	if r.PostFormValue("hash") != hash {
		return p2TextAnswer(http.StatusConflict, tf("error.profiles_file_changed", display))
	}
	original := r.PostFormValue("original")
	entries, _ := profile.ReadAll(text, display)
	edited, err := p2EditProfile(text, display, entries, draft.profileOf(), original)
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if problem, bad := p2FirstError(edited, display); bad {
		return p2TextAnswer(http.StatusUnprocessableEntity, problem)
	}
	if err := source.WriteIfUnchanged(path, hash, edited); err != nil {
		return p2WriteError(err, display)
	}
	if original != "" && original != draft.Name && r.PostFormValue("renameUsers") == "1" {
		s.p2RenameUsers(file, original, draft.Name)
	}
	return s.p2SavedRow(draft.Name, true, file)
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

// p2WriteError turns a write's failure into the answer a person reads.
func p2WriteError(err error, display string) p2Answer {
	if errors.Is(err, source.ErrChanged) {
		return p2TextAnswer(http.StatusConflict, tf("error.profiles_file_changed", display))
	}
	return p2TextAnswer(http.StatusInternalServerError, err.Error())
}

// p2SavedRow is the row a save answers with: the profile as it now reads, open
// at the panel that wrote it.
func (s *Server) p2SavedRow(name string, savedLine bool, file string) p2Answer {
	project, shared, defs := s.p2Read()
	chips := p2Chips(defs)
	row := p2RowOf(name, project, shared, chips, true)
	for i := range row.Panels {
		if row.Panels[i].File == file {
			row.Panels[i].SavedLine = savedLine
		}
	}
	html, err := s.p2Render("p2_row", row)
	if err != nil {
		return p2TextAnswer(http.StatusInternalServerError, err.Error())
	}
	return p2HTMLAnswer(html, project, shared, defs)
}

// p2Render executes one fragment template.
func (s *Server) p2Render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := s.tpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// p2Delete takes one profile out of one file. A profile that is in use and has
// no other file to stand in is refused, so a team is not left naming nothing.
func (s *Server) p2Delete(r *http.Request) p2Answer {
	file, name := r.PostFormValue("file"), r.PostFormValue("name")
	path, display, ok := s.p2ProfilePath(file)
	if !ok {
		return p2TextAnswer(http.StatusBadRequest, "unknown file")
	}
	if err := s.p2Write(path); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	text, hash := s.p2Text(path)
	if r.PostFormValue("hash") != hash {
		return p2TextAnswer(http.StatusConflict, tf("error.profiles_file_changed", display))
	}
	entries, _ := profile.ReadAll(text, display)
	if _, found := p2FindEntry(entries, name); !found {
		return p2TextAnswer(http.StatusUnprocessableEntity, tf("error.edit_no_profile", name))
	}
	otherPath := s.p2OtherPath(file)
	otherText, _ := s.p2Text(otherPath)
	otherEntries, _ := profile.ReadAll(otherText, display)
	_, otherHas := p2FindEntry(otherEntries, name)
	edited, err := profile.EditYAML([]byte(text), display, profile.Remove(name))
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if err := source.WriteIfUnchanged(path, hash, string(edited)); err != nil {
		return p2WriteError(err, display)
	}
	if otherHas {
		return s.p2SavedRow(name, false, "")
	}
	project, shared, defs := s.p2Read()
	return p2HTMLAnswer("", project, shared, defs)
}

// p2OtherPath is the other place a profile may stand.
func (s *Server) p2OtherPath(file string) string {
	if file == p2Shared {
		return s.cat.ProjectProfileFile()
	}
	return s.cat.SharedProfileFile()
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

// p2YAMLSave writes one file's text as the person wrote it.
func (s *Server) p2YAMLSave(r *http.Request) p2Answer {
	file := r.PostFormValue("file")
	path, display, ok := s.p2ProfilePath(file)
	if !ok {
		return p2TextAnswer(http.StatusBadRequest, "unknown file")
	}
	if err := s.p2Write(path); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	text := r.PostFormValue("text")
	if problem, bad := p2FirstError(text, display); bad {
		return p2TextAnswer(http.StatusUnprocessableEntity, problem)
	}
	if err := source.WriteIfUnchanged(path, r.PostFormValue("hash"), text); err != nil {
		return p2WriteError(err, display)
	}
	project, shared, defs := s.p2Read()
	html, err := s.p2Render("p2_yeds", s.p2YamlData(p2Data{}, project, shared).Files)
	if err != nil {
		return p2TextAnswer(http.StatusInternalServerError, err.Error())
	}
	return p2HTMLAnswer(html, project, shared, defs)
}

// p2AssignChange is one change the assign tab sends.
type p2AssignChange struct {
	Member  string `json:"member"`
	From    string `json:"from"`
	Purpose string `json:"purpose"`
	Profile string `json:"profile"`
}

// p2AssignSave writes one card's picks into its team.yaml.
func (s *Server) p2AssignSave(r *http.Request) p2Answer {
	name := r.PostFormValue("definition")
	def, ok := s.p2FindDef(name)
	if !ok {
		return p2TextAnswer(http.StatusNotFound, tf("web.error.not_found"))
	}
	var changes []p2AssignChange
	if err := json.Unmarshal([]byte(r.PostFormValue("changes")), &changes); err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	text, err := s.cat.Reader().Read(source.Doc{Path: def.path})
	if err != nil {
		return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
	}
	if r.PostFormValue("hash") != source.Hash(text) {
		return p2TextAnswer(http.StatusConflict, tf("error.profiles_file_changed", def.display))
	}
	edited := text
	for _, change := range changes {
		op := definition.MemberProfile(change.Member, change.Profile)
		if change.Member == "" {
			op = definition.WorkerProfile(change.From, change.Purpose, change.Profile)
		}
		next, err := definition.EditYAML([]byte(edited), def.display, op)
		if err != nil {
			return p2TextAnswer(http.StatusUnprocessableEntity, err.Error())
		}
		edited = string(next)
	}
	if problem, bad := s.p2MissingProfile(def, changes); bad {
		return p2TextAnswer(http.StatusUnprocessableEntity, problem)
	}
	if err := source.WriteIfUnchanged(def.path, source.Hash(text), edited); err != nil {
		return p2WriteError(err, def.display)
	}
	project, shared, defs := s.p2Read()
	card, err := s.p2Render("p2_team", p2SavedTeam(s, def.name, project, shared, defs))
	if err != nil {
		return p2TextAnswer(http.StatusInternalServerError, err.Error())
	}
	return p2HTMLAnswer(card, project, shared, defs)
}

// p2SavedTeam is one card after a save, with its saved line.
func p2SavedTeam(s *Server, name string, project, shared p2File, defs []p2Def) p2Team {
	for _, def := range defs {
		if def.name == name {
			team := s.p2Team(def, project, shared)
			team.Saved = true
			return team
		}
	}
	return p2Team{Name: name, Saved: true}
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
