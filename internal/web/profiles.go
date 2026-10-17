package web

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/effect"
	"github.com/aq2bq/hergy/internal/profile"
	"github.com/aq2bq/hergy/internal/source"
)

// profiles is the profiles list (設計 3.7): every profile a name resolves to,
// its kind, who runs it, and the two things written from here — a new profile
// and the one a person opened.
func (s *Server) profiles(_ *http.Request) (view, error) {
	return s.renderProfiles(s.profilesData())
}

// profile is one profile's detail (設計 3.7): its kind, args and env, written
// from here, and who uses it.
func (s *Server) profile(r *http.Request) (view, error) {
	name := r.PathValue("name")
	value, ok := s.cat.Profile(name)
	if !ok {
		return s.profileMissing(name)
	}
	return s.renderProfile(s.profileData(name, value, nil))
}

// profilesData is the list, with the fields of one new profile.
func (s *Server) profilesData() profilesData {
	place := s.cat.NewPlace(catalog.NewProfile, "")
	kinds := s.kindChoices()
	data := profilesData{
		Crumb: []crumb{{Name: t("web.band.profiles")}},
		Token: s.token,
		Hash:  s.hashOf(place),
		Kinds: kinds,
		Href:  "/profiles",
	}
	if place != "" {
		// Where the new profile goes, so a person knows before pressing 足す
		// whether this writes the file every project shares (T2, K2).
		data.Place = s.cat.Display(place)
	}
	if len(kinds) > 0 {
		data.AddKind = kinds[0]
	}
	for _, entry := range s.cat.ProfileEntries() {
		row := profileRow{Name: entry.Name, Href: profilePath(entry.Name), Kind: entry.Kind}
		members, workers := s.cat.ProfileUsers(entry.Name)
		row.Users = append(refLines(members), workerRefLines(workers)...)
		if entry.UnusedPath != "" {
			row.Unused = s.cat.Display(entry.UnusedPath)
		}
		if entry.Outside {
			row.Path = s.cat.Display(dirOf(entry.Path))
		}
		data.Rows = append(data.Rows, row)
	}
	data.Empty = len(data.Rows) == 0
	return data
}

// profileData is the detail of one profile, as the screen shows and writes it:
// the values, the file they came from, and the sentences beside them. pairs is
// how the env rows stand on the page: the submitted rows when a write was
// refused (so what a person wrote stays, C4), and the profile's own otherwise.
func (s *Server) profileData(name string, value profile.Profile, pairs []envPair) profileData {
	if pairs == nil {
		pairs = envPairsOf(value.Env)
	}
	path := value.Path
	data := profileData{
		Name:      name,
		Href:      profilePath(name),
		Path:      s.cat.Display(path),
		Kind:      value.Kind,
		Kinds:     s.kindChoices(),
		ArgsText:  strings.Join(value.Args, "\n"),
		ArgsRows:  argsRows(value.Args),
		Users:     s.userLines(name),
		Token:     s.token,
		Hash:      s.hashOf(path),
		DraftKind: "profile:" + name + ":kind",
		DraftArgs: "profile:" + name + ":args",
		EnvRows:   s.envRows(name, pairs),
	}
	data.Deletable = len(data.Users) == 0
	if value.Kind == name {
		// The same warning hergy validate prints (d-a477): a name that is its
		// kind looks like choosing a kind, where I7 makes a profile the engine's
		// settings.
		data.Notice = append(data.Notice, tf("validate.profile_named_after_kind", name))
	}
	for _, entry := range s.cat.ProfileEntries() {
		if entry.Name == name && entry.UnusedPath != "" {
			data.Unused = s.cat.Display(entry.UnusedPath)
		}
	}
	return data
}

// profileMissing is the page a name no profiles.yaml holds: it says so once and
// answers 404 (R2・R3).
func (s *Server) profileMissing(name string) (view, error) {
	return s.renderProfile(profileData{
		Name:        name,
		Href:        profilePath(name),
		Missing:     true,
		MissingLine: tf("web.profile.missing", name),
		Status:      http.StatusNotFound,
	})
}

// profileRefused is the detail redrawn after a write that did not happen: the
// values a person wrote stay, and the reason (or reasons) stand beside them
// (設計 3.0 の「失敗の 1 行」, C4). A file that moved under the page gets the
// reload link with it.
func (s *Server) profileRefused(name string, shown profile.Profile, pairs []envPair, notice []string, reload bool) (view, error) {
	data := s.profileData(name, shown, pairs)
	data.Notice = append(notice, data.Notice...)
	data.NoticeReload = reload
	return s.renderProfile(data)
}

// renderProfile draws one profile's detail, with the trail above it.
func (s *Server) renderProfile(data profileData) (view, error) {
	body, err := s.show("profile.html", data)
	if err != nil {
		return view{}, err
	}
	return view{
		title:      tf("web.profile.title", data.Name),
		band:       bandProfiles,
		crumb:      []crumb{{Name: t("web.band.profiles"), Href: "/profiles"}, {Name: data.Name}},
		body:       body,
		status:     data.Status,
		replaceURL: data.ReplaceURL,
	}, nil
}

// renderProfiles draws the profiles list.
func (s *Server) renderProfiles(data profilesData) (view, error) {
	body, err := s.show("profiles.html", data)
	if err != nil {
		return view{}, err
	}
	return view{
		title:      t("web.profiles.title"),
		band:       bandProfiles,
		crumb:      data.Crumb,
		body:       body,
		replaceURL: data.ReplaceURL,
	}, nil
}

// profilesSaved redraws the list after a profile was written or taken away, with
// the saved line, the sentences that go with it, and what a running team has to
// take up (設計 3.0・3.7).
func (s *Server) profilesSaved(result fileSaved, notice ...string) (view, error) {
	data := s.profilesData()
	data.Saved = result.Line
	data.Notice = notice
	data.Handover = result.Handover
	data.ReplaceURL = "/profiles"
	return s.renderProfiles(data)
}

// profilesRefused redraws the list after a write that did not happen, with the
// reason in one line (設計 3.7).
func (s *Server) profilesRefused(notice ...string) (view, error) {
	data := s.profilesData()
	data.Notice = notice
	return s.renderProfiles(data)
}

// saveProfile changes one profile's kind, args and env (設計 3.7): it edits the
// profiles.yaml the profile was read from and hands it to saveFile, the one save
// road (設計 7.3), which checks the change against every definition that reads the
// file and writes only when the file is still the one the page read.
func (s *Server) saveProfile(r *http.Request) (view, error) {
	name := r.PathValue("name")
	value, ok := s.cat.Profile(name)
	if !ok {
		return s.profileMissing(name)
	}
	path := value.Path
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return view{}, err
	}
	edited, pairs, bad, ok := profileOfForm(name, path, r)
	if !ok {
		return s.profileRefused(name, edited, pairs, []string{bad}, false)
	}
	newText, err := profileEdit(text, s.cat.Display(path), name, value, edited)
	if err != nil {
		return s.profileRefused(name, edited, nil, []string{s.breakLine(path, err)}, false)
	}
	// A kind or an env name the file cannot read is refused with the file's own
	// line, the way hergy validate reports that file (設計 3.7 の「失敗の 1 行」).
	if _, err := profile.ParseProfiles(newText, path); err != nil {
		return s.profileRefused(name, edited, nil, []string{s.breakLine(path, err)}, false)
	}
	names := s.definitionNames()
	// What a hire would start with is read before the write, like the change
	// itself: after it, the catalog would read the new file on both sides
	// (設計 7.3).
	hires := s.hireLines(s.cat.With(path, newText), names)
	result, err := s.saveFile(fileWrite{
		Path: path, Hash: r.PostFormValue("hash"), Text: newText,
		Defs: names, Key: tf("web.saved.profile", name),
	})
	if err != nil {
		return view{}, err
	}
	if result.Stop != "" {
		return s.profileRefused(name, edited, nil, []string{result.Stop}, false)
	}
	if result.Clash {
		return s.profileRefused(name, edited, nil, []string{tf("web.saved.clash", s.cat.Display(path))}, true)
	}
	after, ok := s.cat.Profile(name)
	if !ok {
		return view{}, errors.New("web: the profile is gone after writing it")
	}
	data := s.profileData(name, after, nil)
	data.Saved = result.Line
	// Who is launched differently now (A2, 設計 3.0 の表): the engine change
	// reaches the members that run the profile, an env change reaches their
	// panes, and a profile only workers use reaches the next hire of one
	// (spec/41 T4). A change that reaches nobody says nothing here; the handover
	// line below says the running teams have nothing to take up.
	if len(result.Changes) > 0 {
		data.Preview = effect.Preview(result.Changes)
	}
	data.Preview = append(data.Preview, hires...)
	data.Handover = result.Handover
	data.ReplaceURL = profilePath(name)
	return s.renderProfile(data)
}

// definitionNames is every definition this place reads, by name. One profiles.yaml
// is the layer's, so a write into it is checked against all of them (d-7d73).
func (s *Server) definitionNames() []string {
	entries := s.entries()
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

// hireLines is what a profile change does to the workers that no member runs: the
// next hire of such a worker starts with the new kind, args or env (spec/41 T4).
// It is read through every definition, the way the change itself is.
func (s *Server) hireLines(with *catalog.Catalog, names []string) []string {
	var out []string
	for _, name := range names {
		before, err := s.cat.Read(name)
		if err != nil {
			continue
		}
		after, err := with.Read(name)
		if err != nil {
			continue
		}
		out = append(out, effect.HireLines(s.view(s.cat, before, s.cat.Reader()), s.view(with, after, with.Reader()))...)
	}
	return out
}

// addProfile adds one profile to the profiles.yaml new things go into (設計 3.7,
// K2). The name must not be one hergy already resolves, and the added profile is
// checked the way a change is: a file that would stop a definition is refused.
func (s *Server) addProfile(r *http.Request) (view, error) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	kind := r.PostFormValue("kind")
	// The name goes inside the file; the place is the layer's profiles.yaml,
	// which does not depend on it (K2, C13).
	place := s.cat.NewPlace(catalog.NewProfile, "")
	if place == "" {
		return view{}, errors.New("web: no place for a new profile")
	}
	if name == "" {
		// A name the file cannot hold is refused beside the field, not with the
		// file's own sentence (N4).
		return s.profilesRefused(t("web.profile.name_needed"))
	}
	if kind == "" {
		// The list offers an empty choice when nothing runs a kind (N1): an add
		// that leaves it empty has no kind to write.
		return s.profilesRefused(t("web.profile.kind_needed"))
	}
	if _, ok := s.cat.Profile(name); ok {
		return s.profilesRefused(tf("web.profiles.exists", name))
	}
	hash := source.Hash("")
	newText := ""
	text, err := s.cat.Reader().Read(source.Doc{Path: place})
	switch {
	case err == nil:
		// The file is there: the new profile is appended after the last one, and
		// every line around it stays as the person wrote it (spec/41 S1).
		if _, perr := profile.ParseProfiles(text, place); perr != nil {
			return s.profilesRefused(s.breakLine(place, perr))
		}
		edited, eerr := profile.EditYAML([]byte(text), s.cat.Display(place), profile.Add(name, kind))
		if eerr != nil {
			return s.profilesRefused(s.breakLine(place, eerr))
		}
		hash, newText = source.Hash(text), string(edited)
	case !errors.Is(err, fs.ErrNotExist):
		return view{}, err
	default:
		// The file is not there yet: hergy writes a new one in its own form.
		written, eerr := profile.EmitProfiles(map[string]profile.Profile{
			name: {Name: name, Kind: kind, Path: place},
		})
		if eerr != nil {
			return view{}, eerr
		}
		newText = written
	}
	// A name or a kind the file cannot read is refused with the file's own line.
	if _, err := profile.ParseProfiles(newText, place); err != nil {
		return s.profilesRefused(s.breakLine(place, err))
	}
	// The file may not be there yet (a layer that never held one); its folder is
	// made the way hergy new makes one.
	if err := os.MkdirAll(filepath.Dir(place), 0o755); err != nil {
		return view{}, err
	}
	result, err := s.saveFile(fileWrite{
		Path: place, Hash: hash, Text: newText,
		Defs: s.definitionNames(), Key: tf("web.saved.profile", name),
	})
	if err != nil {
		return view{}, err
	}
	if result.Stop != "" {
		return s.profilesRefused(result.Stop)
	}
	if result.Clash {
		return s.profilesRefused(tf("web.saved.clash", s.cat.Display(place)))
	}
	var notice []string
	if name == kind {
		// A name that is its kind gets validate's warning (d-a477): it is
		// written, and the screen says what the command line will say.
		notice = append(notice, tf("validate.profile_named_after_kind", name))
	}
	return s.profilesSaved(result, notice...)
}

// deleteProfile takes one profile out of its profiles.yaml, but only when
// nothing runs it (設計 3.7): a member or a worker that names it would stop
// reading, so the screen names who uses it instead.
func (s *Server) deleteProfile(r *http.Request) (view, error) {
	name := r.PathValue("name")
	value, ok := s.cat.Profile(name)
	if !ok {
		return s.profileMissing(name)
	}
	if users := s.userLines(name); len(users) > 0 {
		lines := make([]string, 0, len(users))
		for _, user := range users {
			lines = append(lines, tf("web.profile.in_use", user))
		}
		return s.profileRefused(name, value, nil, lines, false)
	}
	text, err := s.cat.Reader().Read(source.Doc{Path: value.Path})
	if err != nil {
		return view{}, err
	}
	edited, err := profile.EditYAML([]byte(text), s.cat.Display(value.Path), profile.Remove(name))
	if err != nil {
		return s.profileRefused(name, value, nil, []string{s.breakLine(value.Path, err)}, false)
	}
	result, err := s.saveFile(fileWrite{
		Path: value.Path, Hash: r.PostFormValue("hash"), Text: string(edited),
		Defs: s.definitionNames(), Key: tf("web.saved.profile", name), Removed: true,
	})
	if err != nil {
		return view{}, err
	}
	if result.Stop != "" {
		return s.profileRefused(name, value, nil, []string{result.Stop}, false)
	}
	if result.Clash {
		return s.profileRefused(name, value, nil, []string{tf("web.saved.clash", s.cat.Display(value.Path))}, true)
	}
	return s.profilesSaved(result)
}

// profileEdit is one profile written into the profiles.yaml it was read from:
// the places that moved, and nothing else (spec/41 S1). A value a person did not
// change is not written at all, so a save that changes nothing leaves the file
// byte for byte, and a save that changes one value leaves the lines of the
// others — their form, their quotes, their comments — exactly as they were.
func profileEdit(text, path, name string, before, after profile.Profile) (string, error) {
	out := text
	var ops []profile.Op
	if after.Kind != before.Kind {
		ops = append(ops, profile.SetKind(name, after.Kind))
	}
	if !slices.Equal(after.Args, before.Args) {
		ops = append(ops, profile.SetArgs(name, after.Args))
	}
	if !maps.Equal(after.Env, before.Env) {
		ops = append(ops, profile.SetEnv(name, after.Env))
	}
	for _, op := range ops {
		edited, err := profile.EditYAML([]byte(out), path, op)
		if err != nil {
			return "", err
		}
		out = string(edited)
	}
	return out, nil
}

// profileOfForm is the profile a person wrote into the form: the kind they
// chose, the args one to a line, and the env pairs. It comes back not-ok with
// the sentence to show when an env pair cannot be read (a value with no name, or
// two pairs sharing a name); the rows as they were written come back either way,
// so a refused write shows what the person typed (C4).
func profileOfForm(name, path string, r *http.Request) (profile.Profile, []envPair, string, bool) {
	pairs, ok := envPairsOfForm(r)
	shown := profile.Profile{
		Name: name,
		Kind: r.PostFormValue("kind"),
		Args: argsOfText(r.PostFormValue("args")),
		Path: path,
	}
	if !ok {
		return shown, pairs, t("web.profile.env_bad"), false
	}
	env := map[string]string{}
	for _, pair := range pairs {
		if _, seen := env[pair.Name]; seen {
			return shown, pairs, t("web.profile.env_bad"), false
		}
		env[pair.Name] = pair.Value
	}
	if len(env) == 0 {
		env = nil
	}
	shown.Env = env
	return shown, nil, "", true
}

// argsRows is how many lines the args box shows: the args it holds, and one
// more to write a new one into, never fewer than three.
func argsRows(args []string) int {
	return max(len(args)+1, 3)
}

// envPair is one environment variable as a person writes it.
type envPair struct {
	Name  string
	Value string
}

// envPairsOfForm reads the env rows a form sent, in the order they were written
// (the names and the values are two parallel fields). A row with neither a name
// nor a value is a blank row to add into and is dropped; a row with a value but
// no name is kept in the list and makes the second return false, so a screen can
// show what to fix beside the row the person wrote. The name is trimmed and the
// value is kept as typed, so a value with a line break or spaces survives
// (d-4ad6).
func envPairsOfForm(r *http.Request) ([]envPair, bool) {
	names, values := r.PostForm["envname"], r.PostForm["envvalue"]
	out := make([]envPair, 0, len(names))
	ok := true
	for i, name := range names {
		name = strings.TrimSpace(name)
		value := ""
		if i < len(values) {
			value = values[i]
		}
		if name == "" && value == "" {
			continue
		}
		if name == "" {
			ok = false
		}
		out = append(out, envPair{Name: name, Value: value})
	}
	return out, ok
}

// argsOfText reads args written one to a line: each line trimmed, a blank line
// dropped.
func argsOfText(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// envPairsOf is a profile's env by name, so the rows hold still.
func envPairsOf(env map[string]string) []envPair {
	out := make([]envPair, 0, len(env))
	for _, name := range sortedKeys(env) {
		out = append(out, envPair{Name: name, Value: env[name]})
	}
	return out
}

// envBlankRows is how many empty rows a profile's env shows, so a new variable
// can be written without a button that writes nothing.
const envBlankRows = 2

// envRows is a profile's env as the form shows it: the pairs it holds, and a few
// blank rows to add into. Each field has its own draft key, so the change kept
// in sessionStorage (C5) comes back to the row it was typed in.
func (s *Server) envRows(name string, pairs []envPair) []envRow {
	rows := make([]envRow, 0, len(pairs)+envBlankRows)
	for i := 0; i < len(pairs)+envBlankRows; i++ {
		row := envRow{
			NameDraft:  fmt.Sprintf("profile:%s:env:%d:name", name, i),
			ValueDraft: fmt.Sprintf("profile:%s:env:%d:value", name, i),
		}
		if i < len(pairs) {
			row.Name, row.Value = pairs[i].Name, pairs[i].Value
		}
		rows = append(rows, row)
	}
	return rows
}

// kindChoices is the kinds a screen offers (設計 3.7, spec/38 T4, spec/41 N1):
// the kinds the profiles this catalog resolves run, the ones run by most
// profiles first and, where the counts are equal, in herdr's own order; then the
// kinds nothing runs, in herdr's order. A kind herdr's list does not know is
// offered too, so opening a profile never changes it by itself (I7).
//
// The count is over every profile a name resolves to, the project's file and the
// global one together (K1): a default read from one file alone would offer a
// kind nothing uses.
//
// When nothing runs a kind, the list leads with an empty choice: the add form
// asks which kind rather than guessing herdr's first (N1).
func (s *Server) kindChoices() []string {
	used := s.usedKinds()
	kinds := profile.Kinds()
	rank := map[string]int{}
	for i, kind := range kinds {
		rank[kind] = i
	}
	names := make([]string, 0, len(used))
	for kind := range used {
		names = append(names, kind)
	}
	slices.SortFunc(names, func(a, b string) int {
		if d := cmp.Compare(used[b], used[a]); d != 0 {
			return d
		}
		if d := cmp.Compare(kindRank(rank, a), kindRank(rank, b)); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})
	out := make([]string, 0, len(kinds)+len(names)+1)
	if len(names) == 0 {
		// Nothing is in use, so no kind is a sensible default: the empty choice
		// stands first and an add that leaves it empty is refused.
		out = append(out, "")
	}
	out = append(out, names...)
	for _, kind := range kinds {
		if used[kind] == 0 {
			out = append(out, kind)
		}
	}
	return out
}

// kindRank is where a kind stands in herdr's own order; a kind herdr does not
// list stands after them all, so the two go after the kinds it knows.
func kindRank(rank map[string]int, kind string) int {
	if at, ok := rank[kind]; ok {
		return at
	}
	return len(rank)
}

// usedKinds is how many of the profiles this catalog resolves run each kind. A
// profiles.yaml that cannot be read contributes none: what cannot be read is not
// something to name a default from (N1).
func (s *Server) usedKinds() map[string]int {
	out := map[string]int{}
	profiles, _, err := s.cat.Profiles()
	if err != nil {
		return out
	}
	for _, value := range profiles {
		if value.Kind != "" {
			out[value.Kind]++
		}
	}
	return out
}

// userLines is who runs one profile, as lines: a member, or a worker by the
// member that hires it and the purpose (C8).
func (s *Server) userLines(name string) []string {
	members, workers := s.cat.ProfileUsers(name)
	return append(refLines(members), workerRefLines(workers)...)
}

// workerRefLines is worker refs as lines: "hergy-dev の lead が呼ぶ 実装".
func workerRefLines(refs []catalog.MemberRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, tf("web.ref.hires", ref.Definition, ref.Member, ref.Purpose))
	}
	return out
}

// profilesData is what the profiles list is given.
type profilesData struct {
	Crumb        []crumb
	Rows         []profileRow
	Empty        bool
	Token        string
	Hash         string
	Kinds        []string
	AddKind      string
	Place        string
	Saved        string
	Handover     handover
	Notice       []string
	NoticeReload bool
	Href         string
	ReplaceURL   string
}

// profileRow is one row of the profiles list.
type profileRow struct {
	Name   string
	Href   string
	Kind   string
	Users  []string
	Path   string
	Unused string
}

// profileData is what one profile's page is given: the values it writes, and the
// sentences beside them.
type profileData struct {
	Name         string
	Href         string
	Path         string
	Unused       string
	Missing      bool
	MissingLine  string
	Kind         string
	Kinds        []string
	ArgsText     string
	ArgsRows     int
	EnvRows      []envRow
	Users        []string
	Deletable    bool
	Notice       []string
	NoticeReload bool
	Saved        string
	Preview      []string
	Handover     handover
	Token        string
	Hash         string
	DraftKind    string
	DraftArgs    string
	Status       int
	ReplaceURL   string
}

// envRow is one environment variable in a profile's form: the name and the
// value, and the draft keys each field keeps its change under (C5).
type envRow struct {
	Name       string
	Value      string
	NameDraft  string
	ValueDraft string
}
