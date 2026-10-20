package rules

import (
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// defaultLevel is how deep a heading is when the section does not say.
const defaultLevel = 2

// Part says what one line of the Markdown a member reads stands for. The screen
// draws the preview from these, so the parts are the web's words too.
const (
	// PartTitle is the "# title" line.
	PartTitle = "title"
	// PartHeading is a section's "## heading" line.
	PartHeading = "heading"
	// PartIntro is a line of a section's intro.
	PartIntro = "intro"
	// PartRule is a line of a rule's body.
	PartRule = "rule"
	// PartReports is a line of the reports block at the end.
	PartReports = "reports"
	// PartBlank is a blank line between two blocks.
	PartBlank = "blank"
)

// MDLine is one physical line of the document as a member reads it: the line's
// text, the part it stands for, and where it is (Section is the section index or
// -1, Rule the rule index within that section or -1). Held is the reason of a
// rule whose send_why is false: it is shown beside the rule's last line, struck
// through, and is not part of Text, so a line count and a character count are
// the ones a member is handed (spec/38 M2).
type MDLine struct {
	Text    string `json:"text"`
	Part    string `json:"part"`
	Section int    `json:"section"`
	Rule    int    `json:"rule"`
	Held    string `json:"held"`
}

// Lines is the document as the lines of the Markdown a member reads: one entry
// per physical line, with a blank line between blocks, exactly the layout
// Markdown writes. Markdown is built from Lines, so the preview a screen draws
// and the prompt a member is handed cannot drift apart.
func (d *Document) Lines() []MDLine {
	blocks := d.mdBlocks()
	var out []MDLine
	for i, block := range blocks {
		if i > 0 {
			out = append(out, MDLine{Part: PartBlank, Section: -1, Rule: -1})
		}
		out = append(out, block...)
	}
	return out
}

// Markdown is the document as a member reads it: the title, the sections in the
// written order, and the report shapes, with the revision, the note, the origin,
// the report and the tests left out. The wording of the report heading is in the
// locales.
func (d *Document) Markdown() string {
	blocks := d.mdBlocks()
	if len(blocks) == 0 {
		return ""
	}
	texts := make([]string, len(blocks))
	for i, block := range blocks {
		texts[i] = blockText(block)
	}
	return strings.Join(texts, "\n\n") + "\n"
}

// mdBlocks is the document as the blocks it is made of: the title, each
// section's heading, intro and rules, and the reports block. A block is one run
// of lines joined with Markdown's blank line between blocks.
func (d *Document) mdBlocks() [][]MDLine {
	var out [][]MDLine
	if d.Title != "" {
		out = append(out, []MDLine{{Text: "# " + d.Title, Part: PartTitle, Section: -1, Rule: -1}})
	}
	for i, section := range d.Sections {
		out = append(out, section.mdBlocks(i)...)
	}
	if len(d.Reports) > 0 {
		out = append(out, d.reportBlocks()...)
	}
	return out
}

// blocks is one section as the blocks it is made of: the heading, the intro and
// the rules. It is what SectionMarkdown joins.
func (s Section) blocks() []string {
	blocks := s.mdBlocks(0)
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		out = append(out, blockText(block))
	}
	return out
}

// mdBlocks is one section as its blocks: the heading, the intro and the rules
// as one list.
func (s Section) mdBlocks(index int) [][]MDLine {
	var out [][]MDLine
	if s.Heading != "" {
		out = append(out, []MDLine{{
			Text: strings.Repeat("#", s.level()) + " " + s.Heading,
			Part: PartHeading, Section: index, Rule: -1,
		}})
	}
	if intro := strings.TrimSpace(s.Intro); intro != "" {
		out = append(out, textLines(intro, PartIntro, index, -1))
	}
	for _, block := range s.ruleBlocks(index) {
		// A rules block that holds nothing but empty lines is left out, the way
		// Markdown has always left it out.
		if blockText(block) != "" {
			out = append(out, block)
		}
	}
	return out
}

func (s Section) level() int {
	if s.Level == 0 {
		return defaultLevel
	}
	return s.Level
}

// ruleBlocks is the rules of one section, as the one block of a list: a rule
// with an id is "- **<id>** <body>", a rule without one is "- <body>", and the
// lines of a multi-line body stay inside their item. A rule the file keeps out
// of the prompt (in_prompt: false) has no line here; its place in the section is
// kept, so a line that does stand here points at the rule it is (C6).
func (s Section) ruleBlocks(index int) [][]MDLine {
	var lines []MDLine
	for i, rule := range s.Rules {
		if !rule.Included() {
			continue
		}
		lines = append(lines, mdRuleLines(item(rule), index, i, rule)...)
	}
	if len(lines) == 0 {
		return nil
	}
	return [][]MDLine{lines}
}

// item is one rule as a list item, with a multi-line body kept inside it. A rule
// carries its id into the prompt as the item's head.
func item(rule Rule) string {
	body := strings.ReplaceAll(rule.Sent(), "\n", "\n   ")
	if rule.ID == "" {
		return "- " + body
	}
	return "- **" + rule.ID + "** " + body
}

// Sent is the rule as a member reads it: its text, and its reason after one
// space, unless send_why says not to.
func (r Rule) Sent() string {
	text := strings.TrimSpace(r.Text)
	if r.Why == "" || (r.SendWhy != nil && !*r.SendWhy) {
		return text
	}
	return text + " " + strings.TrimSpace(r.Why)
}

// heldReason is the reason a rule carries beside its last line rather than in
// its text: the trimmed why of a rule whose send_why is false. Empty otherwise.
func (r Rule) heldReason() string {
	if r.SendWhy == nil || *r.SendWhy || strings.TrimSpace(r.Why) == "" {
		return ""
	}
	return strings.TrimSpace(r.Why)
}

// ruleLines is one rule's lines: the text split at its line breaks, each line
// marked as the rule's, with the reason a send_why: false rule holds beside its
// last line.
func mdRuleLines(text string, section, rule int, r Rule) []MDLine {
	lines := textLines(text, PartRule, section, rule)
	if held := r.heldReason(); held != "" && len(lines) > 0 {
		lines[len(lines)-1].Held = held
	}
	return lines
}

// textLines is one text as lines of one part and place.
func textLines(text string, part string, section, rule int) []MDLine {
	parts := strings.Split(text, "\n")
	out := make([]MDLine, 0, len(parts))
	for _, line := range parts {
		out = append(out, MDLine{Text: line, Part: part, Section: section, Rule: rule})
	}
	return out
}

// blockText is one block as the text Markdown writes for it.
func blockText(block []MDLine) string {
	lines := make([]string, len(block))
	for i, line := range block {
		lines[i] = line.Text
	}
	return strings.Join(lines, "\n")
}

// reportBlocks is the report shapes, at the end of the document: the heading,
// then one block per report.
func (d *Document) reportBlocks() [][]MDLine {
	out := make([][]MDLine, 0, len(d.Reports)+1)
	out = append(out, []MDLine{{Text: "## " + i18n.T("rules.reports_heading"), Part: PartReports, Section: -1, Rule: -1}})
	for _, report := range d.Reports {
		out = append(out, textLines(report.block(), PartReports, -1, -1))
	}
	return out
}

// block is one report shape: its name, where it is written, and its fields.
func (r Report) block() string {
	fields := make([]string, 0, len(r.Fields))
	for _, field := range r.Fields {
		fields = append(fields, i18n.Tf("rules.report_field", field.Name, field.Description))
	}
	return i18n.Tf("rules.report_block", r.Name, r.Where, strings.Join(fields, "\n"))
}
