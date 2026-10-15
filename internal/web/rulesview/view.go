package rulesview

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// The two layers, as the URLs carry them. They repeat internal/web's values so
// this package can be read and tested on its own.
const (
	LayerLocal  = "local"
	LayerGlobal = "global"
)

// The two groups a rule file falls into: a document every member reads, or the
// role file of one member. The list shows them apart.
const (
	GroupDoc  = "doc"
	GroupRole = "role"
)

// The filters the rules list offers (spec/10 の 6 節).
const (
	FilterAll      = "all"
	FilterNoOrigin = "origin"
	FilterNoReason = "reason"
	FilterNoCheck  = "check"
)

// Doc is one rule file the screen shows: where it is, which layer it came from,
// the parsed document, the hash of what was read, and the team definitions that
// read it. When the file cannot be read or parsed, Broken carries the reason
// (the same words the reader names) and Document is nil.
type Doc struct {
	Path     string
	Group    string
	Layer    string
	Document *rules.Document
	Hash     string
	Broken   string
	UsedBy   []string
}

// Label is what the file is called on the screen: its title, or its file name.
func (d Doc) Label() string {
	if d.Document != nil && d.Document.Title != "" {
		return d.Document.Title
	}
	return filepath.Base(d.Path)
}

// Tally is what the file carries, counted the way hergy validate counts it.
func (d Doc) Tally() rules.Tally {
	var tally rules.Tally
	if d.Document != nil {
		tally.Add(d.Document)
	}
	return tally
}

// Revision is the document's version, and 0 when the file writes none.
func (d Doc) Revision() int {
	if d.Document != nil && d.Document.Revision != nil {
		return *d.Document.Revision
	}
	return 0
}

// DocGroup is one group of the file list.
type DocGroup struct {
	Label string
	Rows  []DocRow
}

// DocRow is one file in the list.
type DocRow struct {
	Path     string
	Label    string
	Total    int
	Origin   int
	Href     string // the rules screen showing this file
	Selected bool
	Broken   bool
}

// NewIndex lists the rule files of one layer, with the shown one marked. The
// files whose document could not be read are not here.
func NewIndex(shown string, docs []Doc) []DocGroup {
	var out []DocGroup
	groups := []struct{ key, label string }{
		{GroupDoc, i18n.T("web.rules.group.doc")},
		{GroupRole, i18n.T("web.rules.group.role")},
	}
	for _, group := range groups {
		var rows []DocRow
		for _, doc := range docs {
			if doc.Group != group.key {
				continue
			}
			tally := doc.Tally()
			rows = append(rows, DocRow{
				Path:     doc.Path,
				Label:    doc.Label(),
				Total:    tally.Total,
				Origin:   tally.WithOrigin,
				Href:     docHref(doc, FilterAll),
				Selected: doc.Path == shown,
				Broken:   doc.Broken != "",
			})
		}
		if len(rows) > 0 {
			out = append(out, DocGroup{Label: group.label, Rows: rows})
		}
	}
	return out
}

// Counts are the numbers the screen shows above the rules, counted the way
// rules.Tally counts them: a reason counts whether or not it is sent, and a
// note is neither a reason nor an origin.
type Counts struct {
	Rules    int
	Origin   int
	Reason   int
	Check    int
	NoOrigin int
	NoReason int
	NoCheck  int
}

// FilterChip is one of the three filters, with how many rules it would keep.
type FilterChip struct {
	Key   string
	Label string
	Count int
	On    bool
	Href  string // the same screen with this filter on
}

// FilterChips is the filter row, with one filter on.
func FilterChips(c Counts, on string) []FilterChip {
	return []FilterChip{
		{Key: FilterAll, Label: i18n.T("web.rules.filter.all"), Count: c.Rules, On: on == FilterAll},
		{Key: FilterNoOrigin, Label: i18n.T("web.rules.filter.no_origin"), Count: c.NoOrigin, On: on == FilterNoOrigin},
		{Key: FilterNoReason, Label: i18n.T("web.rules.filter.no_reason"), Count: c.NoReason, On: on == FilterNoReason},
		{Key: FilterNoCheck, Label: i18n.T("web.rules.filter.no_check"), Count: c.NoCheck, On: on == FilterNoCheck},
	}
}

// Tag is one label under a rule in the list.
type Tag struct {
	Kind string // origin, check or note
	Text string
}

// Rule is one rule as the list shows it. Section and Index are where it stands,
// which is what the screen points with when it opens the rule.
type Rule struct {
	Section int
	Index   int
	ID      string
	Text    string
	Why     string
	Send    bool
	Tags    []Tag
}

// Section is one section of a document, with the rules the filter keeps. A
// section the filter empties is left out of the list altogether.
type Section struct {
	Index       int
	Heading     string
	Hash        string
	Intro       string
	StyleLabel  string
	HeadingOnly bool
	Rules       []Rule
}

// Rules is what the rules screen draws: the files to choose from, the one being
// shown, its sections with the filter applied, and the numbers on top.
type Rules struct {
	Layer    string
	Groups   []DocGroup
	Doc      Doc
	Filter   string
	Filters  []FilterChip
	Sections []Section
	Counts   Counts
	// PanelPrefix is the panel's path with the layer and the document on it;
	// a link adds section and i to it.
	PanelPrefix string
	Global      bool
	// Broken is why the shown file cannot be read, empty when it reads. Empty
	// says the layer has no rule file at all; NoMatch says the filter kept none
	// of this document's rules.
	Broken  string
	Empty   bool
	NoMatch bool
}

// NewRules fills the rules screen for one document and filter. filter may be
// empty or unknown, which means all.
func NewRules(doc Doc, filter string) Rules {
	if !knownFilter(filter) {
		filter = FilterAll
	}
	data := Rules{
		Layer:       doc.Layer,
		Doc:         doc,
		Filter:      filter,
		Global:      doc.Layer == LayerGlobal,
		Broken:      doc.Broken,
		PanelPrefix: PathPanel + "?layer=" + doc.Layer + "&doc=" + url.QueryEscape(doc.Path),
	}
	if doc.Document == nil {
		data.Empty = true
		return data
	}
	data.Counts = CountRules(doc.Document)
	data.Filters = FilterChips(data.Counts, filter)
	for i := range data.Filters {
		data.Filters[i].Href = docHref(doc, data.Filters[i].Key)
	}
	kept := 0
	for i, section := range doc.Document.Sections {
		view := NewSection(i, section, filter)
		kept += len(view.Rules)
		if filter != FilterAll && len(view.Rules) == 0 {
			continue
		}
		data.Sections = append(data.Sections, view)
	}
	data.NoMatch = kept == 0
	return data
}

// CountRules counts one document the way hergy validate counts the files.
func CountRules(doc *rules.Document) Counts {
	var c Counts
	for _, rule := range doc.Rules() {
		c.Rules++
		if HasOrigin(rule) {
			c.Origin++
		} else {
			c.NoOrigin++
		}
		if rule.Why != "" {
			c.Reason++
		} else {
			c.NoReason++
		}
		if HasCheck(rule) {
			c.Check++
		} else {
			c.NoCheck++
		}
	}
	return c
}

// NewSection is one section of a document, with the filter applied.
func NewSection(index int, section rules.Section, filter string) Section {
	view := Section{
		Index:   index,
		Heading: section.Heading,
		Intro:   section.Intro,
	}
	if section.Heading != "" {
		view.Hash = strings.Repeat("#", levelOf(section.Level))
	}
	view.StyleLabel = styleLabel(section.Style)
	view.HeadingOnly = section.Heading != "" && len(section.Rules) == 0 && strings.TrimSpace(section.Intro) == ""
	for i, rule := range section.Rules {
		if !Keeps(rule, filter) {
			continue
		}
		view.Rules = append(view.Rules, NewRule(index, i, rule))
	}
	return view
}

// NewRule is one rule as the list shows it.
func NewRule(section, index int, rule rules.Rule) Rule {
	return Rule{
		Section: section,
		Index:   index,
		ID:      rule.ID,
		Text:    rule.Text,
		Why:     rule.Why,
		Send:    rule.SendWhy == nil || *rule.SendWhy,
		Tags:    tagsOf(rule),
	}
}

// Keeps says whether one filter keeps one rule. An unknown filter keeps all.
func Keeps(rule rules.Rule, filter string) bool {
	switch filter {
	case FilterNoOrigin:
		return !HasOrigin(rule)
	case FilterNoReason:
		return rule.Why == ""
	case FilterNoCheck:
		return !HasCheck(rule)
	}
	return true
}

// HasOrigin says whether a rule says where it came from.
func HasOrigin(rule rules.Rule) bool { return len(rule.Origin) > 0 }

// HasCheck says whether a rule names a report field or a test.
func HasCheck(rule rules.Rule) bool { return rule.Report != "" || len(rule.Tests) > 0 }

// OriginLabel is one origin as the screen writes it: the kind, then, for an
// incident or an instruction, the date and the references; for a derived rule,
// an arrow and the rules it came from.
func OriginLabel(origin rules.Origin) string {
	kind := i18n.T("web.rules.origin.kind." + origin.Kind)
	if origin.Kind == rules.KindDerived {
		return kind + " ← " + strings.Join(origin.Refs, " ")
	}
	parts := []string{kind, origin.Date}
	return strings.Join(nonEmpty(append(parts, origin.Refs...)), " ")
}

func tagsOf(rule rules.Rule) []Tag {
	var out []Tag
	for _, origin := range rule.Origin {
		out = append(out, Tag{Kind: "origin", Text: OriginLabel(origin)})
	}
	if rule.Report != "" {
		out = append(out, Tag{Kind: "check", Text: i18n.Tf("web.rules.rule.report", rule.Report)})
	}
	if len(rule.Tests) > 0 {
		out = append(out, Tag{Kind: "check", Text: i18n.Tf("web.rules.rule.tests", strings.Join(rule.Tests, ", "))})
	}
	if rule.Note != "" {
		out = append(out, Tag{Kind: "note", Text: i18n.T("web.rules.rule.note")})
	}
	return out
}

func knownFilter(filter string) bool {
	switch filter {
	case FilterAll, FilterNoOrigin, FilterNoReason, FilterNoCheck:
		return true
	}
	return false
}

func levelOf(level int) int {
	if level == 0 {
		return 2
	}
	return level
}

func styleLabel(style string) string {
	if style == "" {
		style = rules.StyleParagraphs
	}
	return i18n.T("web.rules.style." + style)
}

func nonEmpty(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// docHref is the rules screen for one document and filter. The path is a query
// value, so it is escaped rather than written into the URL as it stands.
func docHref(doc Doc, filter string) string {
	href := PathRules + "?layer=" + doc.Layer + "&doc=" + url.QueryEscape(doc.Path)
	if filter != "" && filter != FilterAll {
		href += "&filter=" + filter
	}
	return href
}
