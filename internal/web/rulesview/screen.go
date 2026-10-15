package rulesview

import (
	"html/template"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// Screen is the rules screen: the Port it reads and writes through, and how it
// draws (spec/10 の 6 節). The HTTP routes that call it live beside it, and the
// app hands it the Port and the WebUI's template functions.
type Screen struct {
	port   Port
	render *Renderer
}

// New builds the screen for one run. funcs are the WebUI's template functions
// (web.Funcs).
func New(port Port, funcs template.FuncMap) (*Screen, error) {
	render, err := NewRenderer(funcs)
	if err != nil {
		return nil, err
	}
	return &Screen{port: port, render: render}, nil
}

// RulesBody is the rules screen: the layer's rule files, the one asked for (the
// first when none is named), and the filter.
func (s *Screen) RulesBody(layer, path, filter string) (string, error) {
	docs := s.docs(layer)
	shown := pick(layer, docs, path)
	data := NewRules(shown, filter)
	data.Groups = NewIndex(shown.Path, docs)
	return s.render.Rules(data)
}

// PanelBody is the edit panel, opened from a rule's place or for a new rule at
// the end of a section. values are the query: sec, i, and new or close.
func (s *Screen) PanelBody(layer, path string, values url.Values) (string, error) {
	if values.Get("close") != "" {
		return s.render.Close()
	}
	docs := s.docs(layer)
	shown := pick(layer, docs, path)
	section := atoi(values.Get("sec"))
	var edit EditRule
	if values.Get("new") != "" {
		edit = EditRule{Section: section, Index: -1, Send: true, ID: suggestID(shown.Document, section)}
	} else {
		rule, err := ruleAt(shown.Document, shown.Path, section, atoi(values.Get("i")))
		if err != nil {
			return "", err
		}
		edit = NewEditRule(section, atoi(values.Get("i")), rule)
	}
	return s.render.Panel(s.panelData(layer, path, values, edit, "", false))
}

// PanelFormBody is the edit panel again, redrawn from the submitted form: a
// person adding or removing an origin, or opening or closing the delete ask.
// Nothing is written here; it only moves the form along.
func (s *Screen) PanelFormBody(layer, path string, values url.Values) (string, error) {
	edit := formOf(values)
	switch {
	case values.Get("add_origin") != "":
		edit.Origins = append(edit.Origins, OriginRow{})
	case values.Get("remove_origin") != "":
		if i := atoi(values.Get("remove_origin")); i >= 0 && i < len(edit.Origins) {
			edit.Origins = append(edit.Origins[:i:i], edit.Origins[i+1:]...)
		}
	}
	asking := values.Get("delete_ask") != ""
	return s.render.Panel(s.panelData(layer, path, values, edit, "", asking))
}

// PreviewBody is the preview of the rule being edited: the section as a member
// reads it, and the break the reader names, when there is one.
func (s *Screen) PreviewBody(layer, path string, values url.Values) (string, error) {
	docs := s.docs(layer)
	shown := pick(layer, docs, path)
	data := Panel{Layer: layer, Doc: shown}
	if shown.Document != nil {
		data.Preview = s.preview(layer, shown, formOf(values))
		data.Broken = data.Preview.Break != ""
	}
	return s.render.Preview(data)
}

// SaveBody writes one rule back: the rule at its place, or a new one at the end
// of its section. When the write is refused (the file moved under the screen,
// or the path is not one of the layer's rule files), the panel comes back with
// the reason on it and nothing is written.
func (s *Screen) SaveBody(layer, path string, values url.Values) (string, error) {
	docs := s.docs(layer)
	shown := pick(layer, docs, path)
	if shown.Document == nil {
		return "", i18n.Errorf("web.rules.error.no_document", path)
	}
	edit := formOf(values)
	next, err := withEdit(shown.Document, edit)
	if err != nil {
		return "", err
	}
	return s.write(layer, shown, next, values, edit)
}

// DeleteBody takes one rule out and writes the file back.
func (s *Screen) DeleteBody(layer, path string, values url.Values) (string, error) {
	docs := s.docs(layer)
	shown := pick(layer, docs, path)
	if shown.Document == nil {
		return "", i18n.Errorf("web.rules.error.no_document", path)
	}
	edit := formOf(values)
	next := clone(shown.Document)
	if err := next.DeleteRule(edit.Section, edit.Index); err != nil {
		return "", err
	}
	return s.write(layer, shown, next, values, edit)
}

// ReportBody makes a report shape (or adds a field to one that is there) in the
// document, points the rule at it, and writes the file back. This is how a rule
// with no check gets one where the person already is (findings F2).
func (s *Screen) ReportBody(layer, path string, values url.Values) (string, error) {
	docs := s.docs(layer)
	shown := pick(layer, docs, path)
	if shown.Document == nil {
		return "", i18n.Errorf("web.rules.error.no_document", path)
	}
	name := strings.TrimSpace(values.Get("report_name"))
	field := strings.TrimSpace(values.Get("field_name"))
	if name == "" || field == "" {
		return "", i18n.Errorf("web.rules.error.report_needs_name_field")
	}
	edit := formOf(values)
	edit.Report = name + "." + field
	next, err := withEdit(shown.Document, edit)
	if err != nil {
		return "", err
	}
	if err := addReportField(next, name, field, values.Get("report_where"), values.Get("field_description")); err != nil {
		return "", err
	}
	return s.write(layer, shown, next, values, edit)
}

// addReportField adds a field to a report shape, making the shape when the
// document does not hold it yet.
func addReportField(doc *rules.Document, name, field, where, description string) error {
	for i := range doc.Reports {
		if doc.Reports[i].Name == name {
			return doc.AddReportField(i, rules.ReportField{Name: field, Description: description})
		}
	}
	return doc.AddReport(rules.Report{
		Name:   name,
		Where:  where,
		Fields: []rules.ReportField{{Name: field, Description: description}},
	})
}

// write emits the edited document and hands it to the Port. A refused write
// comes back as the panel with the reason, not as an error page: the person is
// in the middle of editing, and the screen can say what to do about it.
func (s *Screen) write(layer string, shown Doc, next *rules.Document, values url.Values, edit EditRule) (string, error) {
	text, err := rules.Emit(next)
	if err != nil {
		return s.refused(layer, shown, values, edit, err.Error())
	}
	if err := s.port.WriteRule(shown.Path, text, shown.Hash); err != nil {
		return s.refused(layer, shown, values, edit, err.Error())
	}
	return s.RulesBody(layer, shown.Path, values.Get("filter"))
}

// refused draws the panel again with the reason on it.
func (s *Screen) refused(layer string, shown Doc, values url.Values, edit EditRule, reason string) (string, error) {
	return s.render.Panel(s.panelData(layer, shown.Path, values, edit, reason, false))
}

// panelData is one edit panel: the rule as the form holds it, the reports it may
// point at, and the preview.
func (s *Screen) panelData(layer, path string, values url.Values, edit EditRule, notice string, asking bool) Panel {
	docs := s.docs(layer)
	shown := pick(layer, docs, path)
	data := Panel{
		Layer:        layer,
		Doc:          shown,
		Edit:         edit,
		New:          edit.Index < 0,
		Global:       layer == LayerGlobal,
		Notice:       notice,
		AskingDelete: asking,
		Token:        values.Get("token"),
	}
	data.Reports = ReportOptions(s.scope(layer, docs, shown))
	data.Preview = s.preview(layer, shown, edit)
	data.Broken = data.Preview.Break != ""
	return data
}

// preview is the edited rule as a member would read it. The whole edited
// document is written out and handed to the Port, so the overlay and the checks
// are the code hergy up runs (spec/10 の 3 節).
func (s *Screen) preview(layer string, shown Doc, edit EditRule) Preview {
	if shown.Document == nil {
		return Preview{}
	}
	next, err := withEdit(shown.Document, edit)
	if err != nil {
		return Preview{Break: err.Error()}
	}
	text, err := rules.Emit(next)
	if err != nil {
		return Preview{Break: err.Error()}
	}
	preview, err := s.port.Preview(layer, shown.Path, edit.Section, text)
	if err != nil {
		return Preview{Break: err.Error()}
	}
	return preview
}

// docs reads the rule files one layer's team definitions read, once each, and
// parses them. A file that cannot be read or parsed keeps its place with the
// reason, so one broken file does not hide the rest (spec/10 の 5 節).
func (s *Screen) docs(layer string) []Doc {
	byPath := map[string]*Doc{}
	var order []string
	for _, team := range s.port.RuleTeams(layer) {
		named := map[string]bool{}
		for _, member := range team.Members {
			for _, file := range member.Files {
				doc, ok := byPath[file.Path]
				if !ok {
					doc = &Doc{Path: file.Path, Layer: layer, Group: GroupDoc}
					if file.Role {
						doc.Group = GroupRole
					}
					byPath[file.Path] = doc
					order = append(order, file.Path)
				}
				if !named[file.Path] {
					named[file.Path] = true
					doc.UsedBy = append(doc.UsedBy, team.Name)
				}
			}
		}
	}
	sort.Strings(order)
	out := make([]Doc, 0, len(order))
	for _, path := range order {
		doc := byPath[path]
		source, err := s.port.ReadRule(path)
		if err != nil {
			doc.Broken = err.Error()
			out = append(out, *doc)
			continue
		}
		document, err := rules.Parse(source.Text, path)
		if err != nil {
			doc.Broken = err.Error()
			out = append(out, *doc)
			continue
		}
		doc.Document = document
		doc.Hash = source.Hash
		out = append(out, *doc)
	}
	return out
}

// scope is the documents the members that read shown also read: the range a
// report field may be pointed at (d-0c48).
func (s *Screen) scope(layer string, docs []Doc, shown Doc) []Doc {
	wanted := map[string]bool{}
	for _, team := range s.port.RuleTeams(layer) {
		for _, member := range team.Members {
			reads := false
			for _, file := range member.Files {
				if file.Path == shown.Path {
					reads = true
				}
			}
			if !reads {
				continue
			}
			for _, file := range member.Files {
				wanted[file.Path] = true
			}
		}
	}
	var out []Doc
	for _, doc := range docs {
		if wanted[doc.Path] {
			out = append(out, doc)
		}
	}
	return out
}

// pick is the document asked for, or the first one when none is named.
func pick(layer string, docs []Doc, path string) Doc {
	for _, doc := range docs {
		if doc.Path == path {
			return doc
		}
	}
	if len(docs) > 0 {
		return docs[0]
	}
	return Doc{Layer: layer}
}

// ruleAt is one rule of a document, named by its place.
func ruleAt(doc *rules.Document, path string, section, index int) (rules.Rule, error) {
	if doc == nil || section < 0 || section >= len(doc.Sections) {
		return rules.Rule{}, i18n.Errorf("web.rules.error.no_rule", path, section, index)
	}
	list := doc.Sections[section].Rules
	if index < 0 || index >= len(list) {
		return rules.Rule{}, i18n.Errorf("web.rules.error.no_rule", path, section, index)
	}
	return list[index], nil
}

// withEdit is the document with one rule written into it: replaced at its
// place, or added at the end of its section when the place is -1.
func withEdit(doc *rules.Document, edit EditRule) (*rules.Document, error) {
	next := clone(doc)
	if edit.Index < 0 {
		return next, next.AddRule(edit.Section, edit.Rule())
	}
	return next, next.SetRule(edit.Section, edit.Index, edit.Rule())
}

// clone is a document the edit functions may change without the one that was
// read changing with it.
func clone(doc *rules.Document) *rules.Document {
	out := *doc
	out.Sections = make([]rules.Section, len(doc.Sections))
	for i, section := range doc.Sections {
		section.Rules = append([]rules.Rule(nil), section.Rules...)
		out.Sections[i] = section
	}
	return &out
}

// formOf reads the edit panel's form. The origin fields come in parallel lists,
// one entry per row, which is what a form with repeated names sends.
func formOf(values url.Values) EditRule {
	edit := EditRule{
		Section: atoi(values.Get("section")),
		Index:   atoiDefault(values.Get("i"), -1),
		ID:      values.Get("id"),
		Text:    values.Get("text"),
		Why:     values.Get("why"),
		Send:    values.Get("send") != "",
		Note:    values.Get("note"),
		Report:  values.Get("report"),
		Tests:   values.Get("tests"),
	}
	kinds := values["origin_kind"]
	dates := values["origin_date"]
	refs := values["origin_refs"]
	notes := values["origin_note"]
	for i, kind := range kinds {
		edit.Origins = append(edit.Origins, OriginRow{
			Kind: kind,
			Date: at(dates, i),
			Refs: at(refs, i),
			Note: at(notes, i),
		})
	}
	return edit
}

// suggestID is the id the screen offers for a new rule in one section: a free id
// near the ones that are already there, which the person may change.
func suggestID(doc *rules.Document, section int) string {
	if doc == nil {
		return "NEW"
	}
	base := "NEW"
	if last := lastID(doc, section); last != "" {
		base = bump(last)
	}
	return doc.SuggestRuleID(base)
}

// lastID is the id of the last rule of one section, or of the whole document
// when the section holds none.
func lastID(doc *rules.Document, section int) string {
	if section >= 0 && section < len(doc.Sections) {
		list := doc.Sections[section].Rules
		if len(list) > 0 {
			return list[len(list)-1].ID
		}
	}
	all := doc.Rules()
	if len(all) > 0 {
		return all[len(all)-1].ID
	}
	return ""
}

var trailingNumber = regexp.MustCompile(`^(.*?)([0-9]+)$`)

// bump raises the number an id ends with (C6 -> C7); an id that ends with no
// number gets -2, which the id pattern allows.
func bump(id string) string {
	if m := trailingNumber.FindStringSubmatch(id); m != nil {
		if n, err := strconv.Atoi(m[2]); err == nil {
			return m[1] + strconv.Itoa(n+1)
		}
	}
	return id + "-2"
}

func at(values []string, i int) string {
	if i < len(values) {
		return values[i]
	}
	return ""
}

func atoi(value string) int { return atoiDefault(value, 0) }

func atoiDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
