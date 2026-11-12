package web

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/ops"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/prose"
	"github.com/aq2bq/hergy/internal/source"
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
	UID            int    `json:"uid"`
	Name           string `json:"name,omitempty"`
	Project        *r2Doc `json:"project"`
	Global         *r2Doc `json:"global"`
	IsNew          bool   `json:"isNew,omitempty"`
	NoDoc          bool   `json:"noDoc,omitempty"`
	Markdown       string `json:"markdown,omitempty"`
	ProjectProblem string `json:"projectProblem,omitempty"`
	GlobalProblem  string `json:"globalProblem,omitempty"`
}

// r2Doc is one role document as the screen drafts and reads it. From is where a
// rule stood in the saved document: a draft sends it, the island does not.
type r2Doc struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// r2ReadingOf reads the catalog into the screen's own reading.
func (s *Server) r2ReadingOf() r2Reading {
	out := r2Reading{Project: filepath.Base(s.cat.Base().String())}
	// The few sentences the screen shows without asking: the same words the
	// server would say, so a screen builds no sentence of its own.
	out.Texts = map[string]string{
		"no_role_doc": t("error.no_role_doc"),
		"separator":   t("list.separator"),
	}
	// Where each place keeps its role documents, as a person reads a path.
	out.Places = map[string]string{
		catalog.PlaceProject: s.cat.Display(s.cat.LayerDir(domainplace.Selection(catalog.PlaceProject).Layer())) + "/roles/",
		catalog.PlaceGlobal:  s.cat.Display(s.cat.LayerDir(domainplace.Selection(catalog.PlaceGlobal).Layer())) + "/roles/",
	}
	uid := 0
	rows := map[string]*r2Row{}
	order := []string{}
	get := func(name string) *r2Row {
		if row, ok := rows[name]; ok {
			return row
		}
		row := &r2Row{}
		rows[name] = row
		order = append(order, name)
		return row
	}
	for _, listed := range ops.ListResourceRows(s.cat, catalog.KindRole, source.LayerLocal) {
		row := get(listed.Name)
		if ok, why := ops.ResourceRowReadable(s.cat, listed); !ok {
			// The row keeps its name and its reason: the list draws it and
			// counts it (r-ea17, d-17ab).
			row.Name = listed.Name
			row.ProjectProblem = why
			continue
		}
		switch {
		case listed.Legacy:
			row.Name = listed.Name
			row.Markdown = tf("error.role_markdown", listed.Name, s.cat.Display(listed.Path))
		case listed.NoDoc:
			row.Name = listed.Name
			row.NoDoc, row.IsNew = true, true
			row.Project = r2NewRole(listed.Name)
		default:
			row.Project = s.r2Read(listed.Path)
		}
	}
	for _, listed := range ops.ListResourceRows(s.cat, catalog.KindRole, source.LayerGlobal) {
		row := get(listed.Name)
		if ok, why := ops.ResourceRowReadable(s.cat, listed); !ok {
			row.Name = listed.Name
			row.GlobalProblem = why
			continue
		}
		row.Global = s.r2Read(listed.Path)
	}
	for _, name := range order {
		row := rows[name]
		if row.Project == nil && row.Global == nil && row.Markdown == "" && row.ProjectProblem == "" && row.GlobalProblem == "" {
			continue
		}
		uid++
		row.UID = uid
		out.Roles = append(out.Roles, *row)
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
	return r2Island{Role: s.r2ListOf(reading, []string{source.LayerLocal, source.LayerGlobal}), Team: s.t2List(), Texts: reading.Texts, Places: reading.Places,
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
