package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/aq2bq/hergy/internal/catalog"
)

// roles is the roles list (設計 3.5): every role a name resolves to, who plays
// it, and how many rules its document holds. A role a member names and no layer
// holds a document for keeps its row and offers to make one (W8).
func (s *Server) roles(_ *http.Request) (view, error) {
	missing := s.roleSentences()
	data := rolesData{
		Crumb: []crumb{{Name: t("web.band.roles")}},
		Token: s.token,
	}
	for _, entry := range s.cat.Roles() {
		row := roleRow{Name: entry.Name, Href: rolePath(entry.Name)}
		row.Users = refLines(s.cat.RoleUsers(entry.Name))
		if entry.UnusedPath != "" {
			row.Unused = s.cat.Display(entry.UnusedPath)
		}
		if entry.HasDocument {
			row.Rules = s.ruleCount(entry.Path)
			if entry.Outside {
				row.Path = s.cat.Display(dirOf(entry.Path))
			}
		} else {
			row.Missing = missing[entry.Name]
			row.Make = s.makeForm(catalog.NewRole, entry.Name, "/roles/"+url.PathEscape(entry.Name))
		}
		data.Rows = append(data.Rows, row)
	}
	data.Empty = len(data.Rows) == 0
	return s.pageBody(view{
		title: t("web.roles.title"),
		band:  bandRoles,
		crumb: data.Crumb,
	}, "roles.html", data)
}

// role is one role's document (設計 3.5).
func (s *Server) role(r *http.Request) (view, error) {
	return s.rulesPage(s.rolePlace(r.PathValue("role")), nil, saved{}, "", false)
}

// roleRule is one rule of a role's document, open.
func (s *Server) roleRule(r *http.Request) (view, error) {
	return s.openRule(r, s.rolePlace(r.PathValue("role")))
}

// roleNew is the card a role's document is adding a rule with.
func (s *Server) roleNew(r *http.Request) (view, error) {
	return s.addRule(r, s.rolePlace(r.PathValue("role")))
}

// saveRoleRule writes one rule of a role's document.
func (s *Server) saveRoleRule(r *http.Request) (view, error) {
	return s.saveRule(r, s.rolePlace(r.PathValue("role")), false)
}

// addRoleRule adds a rule to the end of one section of a role's document.
func (s *Server) addRoleRule(r *http.Request) (view, error) {
	return s.saveRule(r, s.rolePlace(r.PathValue("role")), true)
}

// writeRole is the write of the document itself (its own page): the second press
// of 消す takes a rule out, and 作る writes the document where it is not there yet
// (W8). A place with no way to make one answers the internal error page, which a
// run never reaches: the app always tells the catalog where its project is.
func (s *Server) writeRole(r *http.Request) (view, error) {
	role := r.PathValue("role")
	if r.PostFormValue("make") != "" {
		place := s.madeRolePlace(role)
		if place.Doc.Path == "" {
			return view{}, errors.New("web: the catalog does not say where a new document goes")
		}
		return s.writeDocument(r, place)
	}
	return s.writeDocument(r, s.rolePlace(role))
}

// rolePage draws one role's document: its sections of cards, the members that
// play it, the card that is open, and, after a write, the saved line (3.5). A
// role no member plays and no layer holds is a 404 (N2); a role a member plays
// without a document keeps its page and says what validate says (3.5).
func (s *Server) rolePage(place rulesPlace, open *ruleOpen, line saved, notice string, reload bool) (view, error) {
	name := place.Role
	data := roleData{
		Name:         name,
		Href:         rolePath(name),
		Saved:        line.Line,
		Handover:     line.Handover,
		Notice:       notice,
		NoticeReload: reload,
	}
	data.Users = refLines(s.cat.RoleUsers(name))
	doc, ok := s.cat.Role(name)
	if !ok {
		sentences := s.roleSentences()[name]
		data.Make = s.makeForm(catalog.NewRole, name, rolePath(name))
		if len(sentences) == 0 {
			data.MissingOne = tf("web.missing.role", name)
			return s.roleView(data, http.StatusNotFound)
		}
		data.Missing = sentences
		return s.roleView(data, 0)
	}
	place.Doc = doc
	parsed, err := s.ruleDoc(doc)
	if err != nil {
		data.Broken = s.breakLine(doc.Path, err)
		return s.roleView(data, 0)
	}
	data.Path = s.cat.Display(doc.Path)
	warnings := s.ruleTexts(place)
	data.Sections = s.sectionsOf(parsed, func(id string) string { return roleRulePath(name, id) }, place.addURL, warnings)
	if open != nil {
		data.Panel = s.ruleCard(place, *open, notice, reload, line.Line != "", warnings)
		data.PanelOpen = true
		data.Notice = ""
		if data.Panel.Missing {
			return s.roleView(data, http.StatusNotFound)
		}
	}
	return s.roleView(data, 0)
}

// roleView draws one role's page: the trail names the open card when there is
// one (N3), and the status is 404 when the URL named something that is not there
// (N2).
func (s *Server) roleView(data roleData, status int) (view, error) {
	trail := []crumb{{Name: t("web.band.roles"), Href: "/roles"}, {Name: data.Name, Href: rolePath(data.Name)}}
	if data.PanelOpen && data.Panel.Crumb != "" {
		trail = append(trail, crumb{Name: data.Panel.Crumb})
	} else {
		trail[len(trail)-1].Href = ""
	}
	data.Crumb = trail
	return s.pageBody(view{title: tf("web.role.title", data.Name), band: bandRoles, crumb: trail, status: status}, "role.html", data)
}

// roleSentences is the sentence validate writes for every role whose document a
// definition names but a layer does not hold, by role name (設計 3.5).
func (s *Server) roleSentences() map[string][]string {
	out := map[string][]string{}
	for _, entry := range s.entries() {
		if entry.Err != nil {
			continue
		}
		def, err := s.readDefinition(entry.Name)
		if err != nil {
			continue
		}
		findings := s.findings(entry.Name)
		for i := range def.Graph.Members {
			role := def.Graph.Members[i].Role
			field := fmt.Sprintf("members[%d].role", i)
			for _, finding := range findings {
				if finding.Field == field {
					out[role] = append(out[role], finding.Text)
				}
			}
		}
	}
	return out
}

// refLines is member refs as lines a person reads: "hergy-dev の lead".
func refLines(refs []catalog.MemberRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, refLine(ref))
	}
	return out
}

// refLine is one member ref as a line: the definition, the member and, for a
// worker, the purpose it is hired for.
func refLine(ref catalog.MemberRef) string {
	if ref.Purpose != "" {
		return tf("web.ref.worker", ref.Definition, ref.Member, ref.Purpose)
	}
	return tf("web.ref.member", ref.Definition, ref.Member)
}

// dirOf is the settings directory a file stands in.
func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return path
}

// sortedKeys is a map's keys, sorted, for a list that holds still.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// makeForm is the one button that makes a document a place does not have yet
// (W8): where the write goes, the token of this run, and the hash of nothing.
func (s *Server) makeForm(kind, name, action string) *makeForm {
	place := s.cat.NewPlace(kind, name)
	if place == "" {
		return nil
	}
	return &makeForm{Href: action, Token: s.token, Hash: s.hashOf(place), Path: s.cat.Display(place)}
}

// makeForm is what the 作る button sends: where the write goes, the token of this
// run, the hash of nothing, and the place the document is made, so the person
// reads where it goes before pressing (spec/42 T1).
type makeForm struct {
	Href  string
	Token string
	Hash  string
	Path  string
}

// rolesData is what the roles list is given.
type rolesData struct {
	Crumb []crumb
	Rows  []roleRow
	Empty bool
	Token string
}

// roleRow is one row of the roles list.
type roleRow struct {
	Name    string
	Href    string
	Users   []string
	Rules   int
	Missing []string
	Make    *makeForm
	Path    string
	Unused  string
}

// roleData is what one role's page is given.
type roleData struct {
	Name         string
	Href         string
	Users        []string
	Path         string
	Sections     []ruleSection
	Missing      []string
	MissingOne   string
	Broken       string
	Make         *makeForm
	Notice       string
	NoticeReload bool
	Saved        string
	Handover     handover
	Crumb        []crumb
	Panel        panel
	PanelOpen    bool
}
