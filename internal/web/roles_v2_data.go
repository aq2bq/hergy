package web

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/prose"
	"github.com/aq2bq/hergy/internal/source"
	"go.yaml.in/yaml/v3"
)

// This file gathers what the role screen's data island carries (r-778c): the
// rows (each name and the one or two documents behind it), and the teams with
// the people that play the names. The screen's own checks come from
// check.Role, one POST at a time, so nothing here decides what is wrong.

// r2Reading is the role screen's reading of the catalog: one row per role name.
// It is what the list answer and the page's island are built from
// (spec-boundary 5).
type r2Reading struct {
	Project string            `json:"project"`
	Roles   []r2Row           `json:"roles"`
	Texts   map[string]string `json:"texts"`
	Places  map[string]string `json:"places"`
}

// r2Row is one role name and the documents behind it: the project's, the
// global one, or both. A name no layer holds a document for keeps its row and
// offers to make one (W8); a name whose document is still legacy YAML carries the
// sentence that says so instead of a panel (設計 1.2.10).
type r2Row struct {
	UID      int    `json:"uid"`
	Name     string `json:"name,omitempty"`
	Project  *r2Doc `json:"project"`
	Global   *r2Doc `json:"global"`
	IsNew    bool   `json:"isNew,omitempty"`
	NoDoc    bool   `json:"noDoc,omitempty"`
	Markdown string `json:"markdown,omitempty"`
}

// r2Doc is one role document as the screen drafts and reads it. From is where a
// rule stood in the saved document: a draft sends it, the island does not.
type r2Doc struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type r2Member struct {
	Name  string `json:"name"`
	Saved string `json:"saved"`
	Draft string `json:"draft"`
}

// r2ReadingOf reads the catalog into the screen's own reading.
func (s *Server) r2ReadingOf() r2Reading {
	out := r2Reading{Project: filepath.Base(s.cat.Base())}
	// The few sentences the screen shows without asking: the same words the
	// server would say, so a screen builds no sentence of its own.
	out.Texts = map[string]string{
		"no_role_doc": t("error.no_role_doc"),
		"separator":   t("list.separator"),
	}
	// Where each place keeps its role documents, as a person reads a path.
	out.Places = map[string]string{
		catalog.PlaceProject: s.cat.Display(s.cat.LayerDir(catalog.Layer(catalog.PlaceProject))) + "/roles/",
		catalog.PlaceGlobal:  s.cat.Display(s.cat.LayerDir(catalog.Layer(catalog.PlaceGlobal))) + "/roles/",
	}
	uid := 0
	for _, entry := range s.cat.Roles() {
		row := r2Row{}
		switch {
		case entry.HasDocument && entry.Outside:
			row.Global = s.r2Read(entry.Path)
		case entry.HasDocument:
			row.Project = s.r2Read(entry.Path)
			if entry.UnusedPath != "" {
				row.Global = s.r2Read(entry.UnusedPath)
			}
		default:
			// A member names this role and no layer holds a .md for it. A
			// legacy YAML document is named as such, and hergy does not write
			// over it: hergy migrate converts it explicitly.
			if md, ok := s.cat.LegacyRole(entry.Name); ok {
				row.Name = entry.Name
				row.Markdown = tf("error.role_markdown", entry.Name, s.cat.Display(md.Path))
				break
			}
			// Otherwise the row keeps its name with no place chosen and opens
			// a new role's content (設計 1.2.10). The panel's place switch
			// starts on the project, where the document would go (W8).
			row.Name = entry.Name
			row.NoDoc = true
			row.IsNew = true
			row.Project = r2NewRole(entry.Name)
		}
		if row.Project == nil && row.Global == nil && row.Markdown == "" {
			continue
		}
		uid++
		row.UID = uid
		out.Roles = append(out.Roles, row)
	}
	return out
}

// r2Island is the JSON the role page carries (spec-boundary 5): the role list in
// the boundary's shape, the team definitions the 割り当て tab reads, and the few
// words and places only the server can make.
type r2Island struct {
	Role   apiList           `json:"role"`
	Team   apiList           `json:"team"`
	Texts  map[string]string `json:"texts"`
	Places map[string]string `json:"places"`
	// HQ is the headquarters lists (d-73fc): the projects' groups of roles and of
	// the definitions that read them.
	HQ *hqScreenIsland `json:"hq,omitempty"`
}

// r2PageIsland builds the role page's island from the same reading the answers
// are built from.
func (s *Server) r2PageIsland() r2Island {
	reading := s.r2ReadingOf()
	return r2Island{Role: s.r2ListOf(reading), Team: s.t2List(), Texts: reading.Texts, Places: reading.Places,
		HQ: s.hqScreenIsland(apiRoleKind, apiTeamKind)}
}

// r2Read is one role document as the island carries it: the fields a person
// edits, and the text the YAML tab writes. A document that does not read keeps
// the fields the parser got as far as, and its own text for the YAML tab.
func (s *Server) r2Read(path string) *r2Doc {
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return nil
	}
	return r2DocOf(text, path)
}

func r2DocOf(text, path string) *r2Doc {
	name := roleNameOf(path)
	return &r2Doc{Name: name, Title: (prose.Document{Text: text}).Title(name), Text: text}
}

func r2NewRole(name string) *r2Doc { return r2DocOf("# "+name+"\n\n## 責務\n", name+".md") }

// roleNameOf is the role a document's file names: roles/<name>.yaml.
func roleNameOf(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

// r2MembersOf is the people one team definition names, with the role each one
// plays. A file that does not read names none.
func r2MembersOf(text string) []r2Member {
	var raw struct {
		Members []struct {
			Name string `yaml:"name"`
			Role string `yaml:"role"`
		} `yaml:"members"`
	}
	if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
		return nil
	}
	out := make([]r2Member, 0, len(raw.Members))
	for _, member := range raw.Members {
		out = append(out, r2Member{Name: member.Name, Saved: member.Role, Draft: member.Role})
	}
	return out
}
