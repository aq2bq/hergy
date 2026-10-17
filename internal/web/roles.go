package web

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/aq2bq/hergy/internal/catalog"
)

// roles is the roles list (設計 3.5): every role a name resolves to, who plays
// it, and how many rules its document holds.
func (s *Server) roles(_ *http.Request) (view, error) {
	missing := s.roleSentences()
	data := rolesData{Crumb: []crumb{{Name: t("web.band.roles")}}}
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

// role is one role's document (設計 3.5): its rules, read only in this stage.
func (s *Server) role(r *http.Request) (view, error) {
	return s.rolePage(r.PathValue("role"), "")
}

// roleRule is one rule of a role's document, as a panel.
func (s *Server) roleRule(r *http.Request) (view, error) {
	return s.rolePage(r.PathValue("role"), r.PathValue("rule"))
}

// rolePage draws one role, with one rule open when open names it. A role no layer
// holds and no member plays is a 404 (N2); a role a member plays without a
// document keeps its page and shows what validate says.
func (s *Server) rolePage(name, open string) (view, error) {
	data := roleData{Name: name, Href: rolePath(name)}
	data.Users = refLines(s.cat.RoleUsers(name))
	doc, ok := s.cat.Role(name)
	if !ok {
		sentences := s.roleSentences()[name]
		if len(sentences) == 0 {
			data.MissingOne = tf("web.missing.role", name)
			return s.roleView(data, http.StatusNotFound)
		}
		data.Missing = sentences
		return s.roleView(data, 0)
	}
	parsed, err := s.ruleDoc(doc)
	if err != nil {
		data.Broken = s.breakLine(doc.Path, err)
		return s.roleView(data, 0)
	}
	data.Path = s.cat.Display(doc.Path)
	data.Sections = s.sectionsOf(parsed, func(id string) string { return roleRulePath(name, id) })
	if open != "" {
		card, found := cardOf(parsed, open)
		if !found {
			data.MissingOne = tf("web.missing.role_rule", name, open)
			return s.roleView(data, http.StatusNotFound)
		}
		data.Panel = panel{Title: tf("web.panel.rule", open), Key: "rule:" + open, Parent: rolePath(name), Crumb: open, Rule: &card}
		data.PanelOpen = true
	}
	return s.roleView(data, 0)
}

// roleView draws one role's page: the trail names the open rule when there is
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

// rolesData is what the roles list is given.
type rolesData struct {
	Crumb []crumb
	Rows  []roleRow
	Empty bool
}

// roleRow is one row of the roles list.
type roleRow struct {
	Name    string
	Href    string
	Users   []string
	Rules   int
	Missing []string
	Path    string
	Unused  string
}

// roleData is what one role's page is given.
type roleData struct {
	Name       string
	Href       string
	Users      []string
	Path       string
	Sections   []ruleSection
	Missing    []string
	MissingOne string
	Broken     string
	Crumb      []crumb
	Panel      panel
	PanelOpen  bool
}
