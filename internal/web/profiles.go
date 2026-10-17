package web

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/catalog"
)

// profiles is the profiles list (設計 3.7): every profile a name resolves to,
// its kind, and who runs it.
func (s *Server) profiles(_ *http.Request) (view, error) {
	data := profilesData{Crumb: []crumb{{Name: t("web.band.profiles")}}}
	for _, entry := range s.cat.ProfileEntries() {
		row := profileRow{Name: entry.Name, Href: profilePath(entry.Name)}
		row.Kind = s.kindOf(entry.Name)
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
	return s.pageBody(view{
		title: t("web.profiles.title"),
		band:  bandProfiles,
		crumb: data.Crumb,
	}, "profiles.html", data)
}

// profile is one profile's detail (設計 3.7): its kind, args, env and who uses
// it, read only in this stage.
func (s *Server) profile(r *http.Request) (view, error) {
	name := r.PathValue("name")
	crumb := []crumb{{Name: t("web.band.profiles"), Href: "/profiles"}, {Name: name}}
	data := profileData{Name: name, Href: profilePath(name), Crumb: crumb}
	value, ok := s.cat.Profile(name)
	if !ok {
		data.Missing = true
		return s.pageBody(view{title: tf("web.profile.title", name), band: bandProfiles, crumb: crumb, status: http.StatusNotFound}, "profile.html", data)
	}
	data.Kind = value.Kind
	data.Path = s.cat.Display(value.Path)
	data.Args = value.Args
	data.Env = envLines(value.Env)
	members, workers := s.cat.ProfileUsers(name)
	data.Users = append(refLines(members), workerRefLines(workers)...)
	for _, entry := range s.cat.ProfileEntries() {
		if entry.Name == name && entry.UnusedPath != "" {
			data.Unused = s.cat.Display(entry.UnusedPath)
		}
	}
	return s.pageBody(view{title: tf("web.profile.title", name), band: bandProfiles, crumb: crumb}, "profile.html", data)
}

// workerRefLines is worker refs as lines: "hergy-dev の lead が呼ぶ 実装".
func workerRefLines(refs []catalog.MemberRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, tf("web.ref.hires", ref.Definition, ref.Member, ref.Purpose))
	}
	return out
}

// envLine is one environment variable, as a pair.
type envLine struct {
	Name  string
	Value string
}

// envLines is a profile's env, by name, so the list holds still.
func envLines(env map[string]string) []envLine {
	out := make([]envLine, 0, len(env))
	for _, name := range sortedKeys(env) {
		out = append(out, envLine{Name: name, Value: env[name]})
	}
	return out
}

// profilesData is what the profiles list is given.
type profilesData struct {
	Crumb []crumb
	Rows  []profileRow
	Empty bool
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

// profileData is what one profile's page is given.
type profileData struct {
	Name    string
	Href    string
	Kind    string
	Path    string
	Args    []string
	Env     []envLine
	Users   []string
	Unused  string
	Missing bool
	Crumb   []crumb
}
