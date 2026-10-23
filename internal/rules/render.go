package rules

import (
	"strings"
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
	// PartBlank is a blank line between two blocks.
	PartBlank = "blank"
	// PartExcluded is a rule the file keeps out of the prompt (in_prompt:
	// false), as a screen's preview draws it: it stands where the rule stands,
	// with no line number, and is in no prompt and in no line count
	// (spec-teams C1).
	PartExcluded = "excluded"
)

// MDLine is one physical line of the document as a member reads it: the line's
// text, the part it stands for, and where it is (Section is the section index or
// -1, Rule the rule index within that section or -1).
type MDLine struct {
	Text    string `json:"text"`
	Part    string `json:"part"`
	Section int    `json:"section"`
	Rule    int    `json:"rule"`
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

// PreviewLines is the document as a screen draws it: Lines(), with the rules the
// file keeps out of the prompt added back where they stand, each as one
// PartExcluded line ("<id> <first line of the body>"). Lines() and Markdown()
// are not changed: an excluded rule is in no prompt and in no line count
// (spec-teams C1, spec-roles A1).
func (d *Document) PreviewLines() []MDLine {
	var out []MDLine
	for i, block := range d.mdPreviewBlocks() {
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
	return out
}

// mdPreviewBlocks is mdBlocks with the rules the file keeps out of the prompt
// still in their place, so a screen can draw them (spec-teams C1).
func (d *Document) mdPreviewBlocks() [][]MDLine {
	var out [][]MDLine
	if d.Title != "" {
		out = append(out, []MDLine{{Text: "# " + d.Title, Part: PartTitle, Section: -1, Rule: -1}})
	}
	for i, section := range d.Sections {
		out = append(out, section.mdPreviewBlocks(i)...)
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

// mdPreviewBlocks is mdBlocks with the rules the file keeps out of the prompt
// still in their place: an excluded rule is one PartExcluded line, so a screen
// draws it where the rule stands (spec-teams C1).
func (s Section) mdPreviewBlocks(index int) [][]MDLine {
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
	var lines []MDLine
	for i, rule := range s.Rules {
		if !rule.Included() {
			lines = append(lines, excludedLine(rule, index, i))
			continue
		}
		lines = append(lines, mdRuleLines(item(rule), index, i, rule)...)
	}
	if blockText(lines) != "" {
		out = append(out, lines)
	}
	return out
}

// excludedLine is one rule the file keeps out of the prompt, as a screen's
// preview draws it: the rule's id and the first line of its body, so a person
// still sees what the rule says without it counting as a line a member reads
// (spec-teams C1, spec-roles A1).
func excludedLine(rule Rule, section, index int) MDLine {
	text := strings.TrimSpace(rule.Text)
	if at := strings.IndexByte(text, '\n'); at >= 0 {
		text = strings.TrimSpace(text[:at])
	}
	if rule.ID != "" {
		text = rule.ID + " " + text
	}
	return MDLine{Text: text, Part: PartExcluded, Section: section, Rule: index}
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
	body := strings.ReplaceAll(strings.TrimSpace(rule.Text), "\n", "\n   ")
	if rule.ID == "" {
		return "- " + body
	}
	return "- **" + rule.ID + "** " + body
}

// ruleLines is one rule's lines: the text split at its line breaks, each line
// marked as the rule's, with the reason a send_why: false rule holds beside its
// last line.
func mdRuleLines(text string, section, rule int, _ Rule) []MDLine {
	return textLines(text, PartRule, section, rule)
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
