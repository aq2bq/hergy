package rulesview

import (
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/rules"
)

// The four sub-tabs of the rules screen: the rules themselves, the document as
// it reaches a member's prompt, what moved since the last launch, and the file
// as hergy writes it with the reader's checks (試作 rulesView の 4 つ).
const (
	TabRules   = "rules"
	TabPrompt  = "prompt"
	TabChanges = "changes"
	TabYAML    = "yaml"
)

// SubTab is one tab of the rules screen, as the band draws it.
type SubTab struct {
	Key   string
	Label string
	Href  string
	On    bool
}

// NormalizeTab reads the tab one URL names: the four the screen has, and the
// rules tab for anything else.
func NormalizeTab(tab string) string {
	switch tab {
	case TabPrompt, TabChanges, TabYAML:
		return tab
	}
	return TabRules
}

// NewTabs is the band: one tab per view, the open one marked. A layer with no
// document to show has no band. query travels with every tab, so a search box
// that is not empty is still there when the person comes back to the rules.
func NewTabs(doc Doc, on, query string) []SubTab {
	if doc.Document == nil {
		return nil
	}
	on = NormalizeTab(on)
	return []SubTab{
		{Key: TabRules, Label: i18n.T("web.rules.tab.rules"), Href: tabHref(doc, TabRules, query), On: on == TabRules},
		{Key: TabPrompt, Label: i18n.T("web.rules.tab.prompt"), Href: tabHref(doc, TabPrompt, query), On: on == TabPrompt},
		{Key: TabChanges, Label: i18n.T("web.rules.tab.changes"), Href: tabHref(doc, TabChanges, query), On: on == TabChanges},
		{Key: TabYAML, Label: i18n.T("web.rules.tab.yaml"), Href: tabHref(doc, TabYAML, query), On: on == TabYAML},
	}
}

// tabHref is the rules screen at one document, tab and search box. The rules
// tab is the screen's own URL, so it carries no tab value; the search box
// carries through so switching a tab does not clear it.
func tabHref(doc Doc, tab, query string) string {
	href := PathRules + "?layer=" + doc.Layer + "&doc=" + url.QueryEscape(doc.Path)
	if tab != "" && tab != TabRules {
		href += "&tab=" + tab
	}
	if query != "" {
		href += "&q=" + url.QueryEscape(query)
	}
	return href
}

// Prompt is the prompt tab: the document as hergy hands it to an agent. Raw
// shows the Markdown as it is, and Overlay draws the pieces the prompt leaves
// out under each rule (試作 renderTab).
type Prompt struct {
	Raw     bool
	Overlay bool
	// Chars is the length of the Markdown, in characters (the prototype counts
	// characters, not bytes).
	Chars int
	// Markdown is the document as hergy hands it over, shown as it is when Raw.
	Markdown string
	// Blocks is the document as a member reads it, drawn when not Raw.
	Blocks []PromptBlock
	// RawHref and OverlayHref flip the two switches.
	RawHref     string
	OverlayHref string
}

// PromptBlock is one piece of the document as the prompt tab draws it: a
// heading, the intro, one rule, a list of rules, or a report shape (試作 blocks).
type PromptBlock struct {
	Kind string // h, intro, p, list or report
	// A heading.
	Level int
	Hash  string
	Text  string
	// A list: the style it is laid out in, and its items.
	Style string
	Items []PromptRule
	// One rule, drawn as a paragraph.
	Rule PromptRule
	// A report shape.
	Name   string
	Where  string
	Fields []PromptField
}

// PromptRule is one rule as the prompt tab draws it: where it stands, its id,
// the text a member reads, where the panel opens it, and what the overlay adds.
type PromptRule struct {
	ID      string
	Section int
	Index   int
	Text    string
	// Href is where the right panel opens this rule, the same hx-get the rules
	// tab uses.
	Href string
	// ShowID says the list writes the id in front of the text (the ids style).
	ShowID bool
	// ShowUnsent says the overlay is on, so the pieces the prompt leaves out are
	// drawn under the rule.
	ShowUnsent bool
	Unsent     []Unsent
}

// PromptField is one field of a report shape.
type PromptField struct {
	Name        string
	Description string
}

// Unsent is one piece the prompt leaves out, shown under a rule when the
// overlay is on: the label it is shown under, and its text (試作 unsentHtml).
type Unsent struct {
	Label string
	Text  string
}

// promptOf reads one document as the prompt tab draws it: the Markdown hergy
// hands over, the same text as the blocks a member reads, and the links that
// flip the two switches.
func (s *Screen) promptOf(doc Doc, raw, overlay bool) Prompt {
	out := Prompt{Raw: raw, Overlay: overlay}
	if doc.Document != nil {
		out.Markdown = doc.Document.Markdown()
		out.Chars = utf8.RuneCountInString(out.Markdown)
		if !raw {
			out.Blocks = promptBlocks(doc.Document, panelPrefix(doc, doc.Path), overlay)
		}
	}
	out.RawHref = promptHref(doc, !raw, overlay)
	out.OverlayHref = promptHref(doc, raw, !overlay)
	return out
}

// promptHref is the prompt tab with the two switches set.
func promptHref(doc Doc, raw, overlay bool) string {
	q := url.Values{}
	q.Set("layer", doc.Layer)
	q.Set("doc", doc.Path)
	q.Set("tab", TabPrompt)
	if raw {
		q.Set("raw", "1")
	}
	if overlay && !raw {
		q.Set("overlay", "1")
	}
	return PathRules + "?" + q.Encode()
}

// promptBlocks is one document as the blocks a member reads: the title, the
// sections in order with their rules laid out the way each section says, and the
// report shapes at the end (試作 blocks). It is the same shape rules.Markdown
// writes, so the raw view and the drawn view say the same thing. prefix is the
// path each rule's panel opens at, and overlay says the pieces the prompt leaves
// out are drawn under each rule.
func promptBlocks(doc *rules.Document, prefix string, overlay bool) []PromptBlock {
	var out []PromptBlock
	if doc.Title != "" {
		out = append(out, headingBlock(1, doc.Title))
	}
	for i, section := range doc.Sections {
		if section.Heading != "" {
			out = append(out, headingBlock(levelOf(section.Level), section.Heading))
		}
		if intro := strings.TrimSpace(section.Intro); intro != "" {
			out = append(out, PromptBlock{Kind: "intro", Text: intro})
		}
		out = append(out, sectionBlocks(i, section, prefix, overlay)...)
	}
	if len(doc.Reports) > 0 {
		out = append(out, headingBlock(2, i18n.T("rules.reports_heading")))
		for _, report := range doc.Reports {
			out = append(out, reportBlock(report))
		}
	}
	return out
}

func headingBlock(level int, text string) PromptBlock {
	return PromptBlock{Kind: "h", Level: level, Hash: strings.Repeat("#", level), Text: text}
}

func reportBlock(report rules.Report) PromptBlock {
	fields := make([]PromptField, 0, len(report.Fields))
	for _, field := range report.Fields {
		fields = append(fields, PromptField{Name: field.Name, Description: field.Description})
	}
	return PromptBlock{Kind: "report", Name: report.Name, Where: report.Where, Fields: fields}
}

// sectionBlocks is one section's rules, laid out the way the section says: one
// rule after another, one list, or a list with each rule's id in front (試作
// blocks の styles).
func sectionBlocks(section int, s rules.Section, prefix string, overlay bool) []PromptBlock {
	if len(s.Rules) == 0 {
		return nil
	}
	style := s.Style
	if style == "" {
		style = rules.StyleParagraphs
	}
	if style == rules.StyleParagraphs {
		out := make([]PromptBlock, 0, len(s.Rules))
		for i, rule := range s.Rules {
			out = append(out, PromptBlock{Kind: "p", Rule: promptRule(section, i, rule, prefix, overlay, style)})
		}
		return out
	}
	items := make([]PromptRule, 0, len(s.Rules))
	for i, rule := range s.Rules {
		items = append(items, promptRule(section, i, rule, prefix, overlay, style))
	}
	return []PromptBlock{{Kind: "list", Style: style, Items: items}}
}

// promptRule is one rule as the prompt tab draws it: the text a member reads,
// which is rules.Rule.Sent, where the panel opens it, whether the list writes
// its id in front, and what the prompt leaves out when the overlay is on.
func promptRule(section, index int, rule rules.Rule, prefix string, overlay bool, style string) PromptRule {
	out := PromptRule{
		ID:      rule.ID,
		Section: section,
		Index:   index,
		Text:    rule.Sent(),
		Href:    prefix + "&section=" + strconv.Itoa(section) + "&i=" + strconv.Itoa(index),
		ShowID:  style == rules.StyleIDs,
	}
	if overlay {
		out.ShowUnsent = true
		out.Unsent = unsentOf(rule)
	}
	return out
}

// unsentOf is what a rule carries that the prompt leaves out: where it came
// from, a reason send_why keeps out, and the note (試作 unsentHtml). The report
// and the tests the prototype also lists are left out: the checks stand in the
// rules tab already.
func unsentOf(rule rules.Rule) []Unsent {
	var out []Unsent
	for _, origin := range rule.Origin {
		text := OriginLabel(origin)
		if origin.Note != "" {
			text += " · " + origin.Note
		}
		out = append(out, Unsent{Label: i18n.T("web.rules.prompt.unsent.origin"), Text: text})
	}
	if strings.TrimSpace(rule.Why) != "" && rule.SendWhy != nil && !*rule.SendWhy {
		out = append(out, Unsent{Label: i18n.T("web.rules.prompt.unsent.why"), Text: rule.Why})
	}
	if rule.Note != "" {
		out = append(out, Unsent{Label: i18n.T("web.rules.prompt.unsent.note"), Text: rule.Note})
	}
	return out
}

// DocChanges is what the launch record says moved in one document since the
// launch that last read it: the version it holds, and the rules that were
// added, dropped and changed. It is the same comparison the home screen draws
// (d-0a25), so the two cannot disagree.
type DocChanges struct {
	// Recorded is whether a launch of the layer read this document: without one
	// there is nothing to compare with.
	Recorded bool
	// Version is the document's own version now, 0 when the file writes none.
	Version int
	// Bumped is set when the version is ahead of the one the launch read: the
	// next launch hands the change over.
	Bumped  bool
	Added   []ChangeRule
	Removed []ChangeRule
	Changed []ChangeRule
	// Whole is set when the document moved but its rules could not be lined up.
	Whole bool
	// Missing is set when the launch read the document and its text is not in
	// the record, so the two cannot be compared.
	Missing bool
}

// Count is how many rules moved in the document.
func (d DocChanges) Count() int { return len(d.Added) + len(d.Removed) + len(d.Changed) }

// Changes reports whether anything moved in the document.
func (d DocChanges) Changes() bool { return d.Count() > 0 || d.Whole || d.Missing }

// CanBump says the version can be raised: a launch read the document, its
// content moved since, and the version has not been raised since that launch.
func (d DocChanges) CanBump() bool {
	return d.Recorded && !d.Bumped && !d.Missing && (d.Count() > 0 || d.Whole)
}

// Next is the version a bump writes.
func (d DocChanges) Next() int { return d.Version + 1 }

// ChangeRule is one rule that moved, as the changes tab draws it (試作
// changeHtml). The home screen draws the same rows.
type ChangeRule struct {
	ID      string
	Section string
	Fields  []string
	Diffs   []ChangePart
	// Href is where the rules screen opens the rule in its panel.
	Href string
}

// ChangePart is one part of a rule as it moved: what it read before and after.
type ChangePart struct {
	Before string
	After  string
}

// BumpAnswer is what raising one document's version answered with: the version
// it moved from and to, or why nothing was written (spec/10 の 7 節).
type BumpAnswer struct {
	Written bool
	From    int
	To      int
	// Why is why nothing was written: the record does not hold the document,
	// its content has not moved, or its version was already raised.
	Why string
}

// YAML is the YAML and checks tab: the file as it is on disk, and what the
// reader says about the whole document (試作 yamlTab).
type YAML struct {
	// Source is the file as it stands on disk, which is what hergy reads and
	// what the person may copy.
	Source string
	// Check is the break the reader names for the document, empty when it is
	// sound.
	Check string
	// OK says the reader found no problem.
	OK bool
}

// yamlOf reads the file the tab shows and asks the reader about the whole
// document: section is -1, so the checks are the ones hergy up runs over it.
func (s *Screen) yamlOf(layer string, doc Doc) YAML {
	source, err := s.port.ReadRule(doc.Path)
	if err != nil {
		return YAML{Check: err.Error()}
	}
	out := YAML{Source: source.Text}
	preview, err := s.port.Preview(layer, doc.Path, -1, source.Text)
	if err != nil {
		out.Check = err.Error()
		return out
	}
	out.Check = preview.Break
	out.OK = preview.Break == ""
	return out
}
