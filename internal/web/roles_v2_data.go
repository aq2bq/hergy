package web

import (
	"path/filepath"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/rules"
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
// offers to make one (W8); a name whose document is still Markdown carries the
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
	Name     string      `json:"name"`
	Title    string      `json:"title"`
	Revision *int        `json:"revision"`
	Sections []r2Section `json:"sections"`
	Hash     string      `json:"hash,omitempty"`
	Text     string      `json:"text,omitempty"`
}

type r2Section struct {
	From    *int     `json:"from,omitempty"`
	Heading string   `json:"heading"`
	Level   int      `json:"level"`
	Intro   string   `json:"intro"`
	Rules   []r2Rule `json:"rules"`
}

type r2Rule struct {
	From     *r2From    `json:"from,omitempty"`
	ID       string     `json:"id"`
	Text     string     `json:"text"`
	Why      string     `json:"why"`
	SendWhy  bool       `json:"send_why"`
	InPrompt bool       `json:"in_prompt"`
	Note     string     `json:"note"`
	Report   string     `json:"report"`
	Tests    []string   `json:"tests"`
	Origin   []r2Origin `json:"origin"`
}

type r2From struct {
	Section int `json:"section"`
	Rule    int `json:"rule"`
}

type r2Origin struct {
	Kind string   `json:"kind"`
	Date string   `json:"date"`
	Refs []string `json:"refs"`
	Note string   `json:"note"`
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
			// A member names this role and no layer holds a .yaml for it. A
			// document still left as Markdown is named as such, and hergy does
			// not write over it: the team-building skill moves it over.
			if md, ok := s.cat.RoleMarkdown(entry.Name); ok {
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
}

// r2PageIsland builds the role page's island from the same reading the answers
// are built from.
func (s *Server) r2PageIsland() r2Island {
	reading := s.r2ReadingOf()
	return r2Island{Role: s.r2ListOf(reading), Team: s.t2List(), Texts: reading.Texts, Places: reading.Places}
}

// r2Read is one role document as the island carries it: the fields a person
// edits, and the text the YAML tab writes. A document that does not read keeps
// the fields the parser got as far as, and its own text for the YAML tab.
func (s *Server) r2Read(path string) *r2Doc {
	text, err := s.cat.Reader().Read(source.Doc{Path: path})
	if err != nil {
		return nil
	}
	doc, _ := rules.ReadAll(text, s.cat.Display(path))
	if doc == nil {
		doc = &rules.Document{Path: path}
	}
	return r2DocOf(doc, source.Hash(text), text)
}

// r2DocOf is a parsed document in the screen's JSON shape.
func r2DocOf(doc *rules.Document, hash, text string) *r2Doc {
	out := &r2Doc{Name: roleNameOf(doc.Path), Title: doc.Title, Revision: doc.Revision, Hash: hash, Text: text}
	for _, section := range doc.Sections {
		sec := r2Section{Heading: section.Heading, Level: section.Level, Intro: section.Intro}
		for i := range section.Rules {
			rule := &section.Rules[i]
			send := rule.SendWhy == nil || *rule.SendWhy
			row := r2Rule{ID: rule.ID, Text: rule.Text, Why: rule.Why, SendWhy: send, InPrompt: rule.Included(), Note: rule.Note, Report: rule.Report, Tests: rule.Tests}
			for _, origin := range rule.Origin {
				row.Origin = append(row.Origin, r2Origin{Kind: origin.Kind, Date: origin.Date, Refs: origin.Refs, Note: origin.Note})
			}
			sec.Rules = append(sec.Rules, row)
		}
		out.Sections = append(out.Sections, sec)
	}
	return out
}

// r2NewRole is the content of a new role a screen opens (spec-roles 1.2.4):
// revision 1, one section 使命, and one empty rule. The name is the caller's.
func r2NewRole(name string) *r2Doc {
	revision := 1
	return &r2Doc{
		Name: name, Revision: &revision,
		Sections: []r2Section{{
			Heading: "使命",
			Rules:   []r2Rule{{SendWhy: true, InPrompt: true}},
		}},
	}
}

// roleNameOf is the role a document's file names: roles/<name>.yaml.
func roleNameOf(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

// document is the draft in the rules package's words, the document a check or a
// rewrite reads.
func (d r2Doc) document() *rules.Document {
	out := &rules.Document{Title: d.Title, Revision: d.Revision}
	for _, section := range d.Sections {
		sec := rules.Section{Heading: section.Heading, Level: section.Level, Intro: section.Intro}
		for i := range section.Rules {
			rule := &section.Rules[i]
			// send_why true is the format's default and the file leaves it
			// unsaid, so a draft that carries the default travels as no value
			// at all: a rule a person did not touch comes back byte for byte.
			var send *bool
			if !rule.SendWhy {
				no := false
				send = &no
			}
			// in_prompt true is the default the same way, so a rule that is in
			// the prompt comes back with the key unsaid. The screen offers no
			// switch for the key yet (d-f187; the switch is a later request).
			var inPrompt *bool
			if !rule.InPrompt {
				no := false
				inPrompt = &no
			}
			parsed := rules.Rule{ID: rule.ID, Text: rule.Text, Why: rule.Why, SendWhy: send, InPrompt: inPrompt, Note: rule.Note, Report: rule.Report, Tests: rule.Tests}
			for _, origin := range rule.Origin {
				parsed.Origin = append(parsed.Origin, rules.Origin{Kind: origin.Kind, Date: origin.Date, Refs: origin.Refs, Note: origin.Note})
			}
			sec.Rules = append(sec.Rules, parsed)
		}
		out.Sections = append(out.Sections, sec)
	}
	return out
}

// froms is where each draft rule stood in the saved document, in the order the
// draft writes its rules (the place Rewrite reads).
func (d r2Doc) froms() []rules.Place {
	var out []rules.Place
	for _, section := range d.Sections {
		for _, rule := range section.Rules {
			if rule.From == nil {
				out = append(out, rules.Place{Section: -1, Rule: -1})
				continue
			}
			out = append(out, rules.Place{Section: rule.From.Section, Rule: rule.From.Rule})
		}
	}
	return out
}

// sectionFroms is where each draft section stood in the saved document, in the
// order the draft writes its sections (the place Rewrite reads, C7). Nil when no
// section says: the write then reads the places from the rules' from.
func (d r2Doc) sectionFroms() []int {
	out := make([]int, len(d.Sections))
	found := false
	for i, section := range d.Sections {
		out[i] = -1
		if section.From != nil {
			out[i] = *section.From
			found = true
		}
	}
	if !found {
		return nil
	}
	return out
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
