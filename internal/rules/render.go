package rules

import (
	"strconv"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// defaultLevel is how deep a heading is when the section does not say.
const defaultLevel = 2

// Markdown is the document as a member reads it: the title, the sections in the
// written order, and the report shapes, with the revision, the note, the origin,
// the report and the tests left out. The wording of the report heading is in the
// locales.
func (d *Document) Markdown() string {
	var blocks []string
	if d.Title != "" {
		blocks = append(blocks, "# "+d.Title)
	}
	for _, section := range d.Sections {
		blocks = append(blocks, section.blocks()...)
	}
	if len(d.Reports) > 0 {
		blocks = append(blocks, d.reportsBlock())
	}
	if len(blocks) == 0 {
		return ""
	}
	return strings.Join(blocks, "\n\n") + "\n"
}

// blocks is one section as the blocks it is made of: the heading, the intro and
// the rules.
func (s Section) blocks() []string {
	var blocks []string
	if s.Heading != "" {
		blocks = append(blocks, strings.Repeat("#", s.level())+" "+s.Heading)
	}
	if intro := strings.TrimSpace(s.Intro); intro != "" {
		blocks = append(blocks, intro)
	}
	if body := s.rulesBlock(); body != "" {
		blocks = append(blocks, body)
	}
	return blocks
}

func (s Section) level() int {
	if s.Level == 0 {
		return defaultLevel
	}
	return s.Level
}

// rulesBlock is the rules of one section, laid out the way the section says.
func (s Section) rulesBlock() string {
	if len(s.Rules) == 0 {
		return ""
	}
	if s.Style == "" || s.Style == StyleParagraphs {
		blocks := make([]string, 0, len(s.Rules))
		for _, rule := range s.Rules {
			blocks = append(blocks, rule.Sent())
		}
		return strings.Join(blocks, "\n\n")
	}
	lines := make([]string, 0, len(s.Rules))
	for i, rule := range s.Rules {
		lines = append(lines, s.item(rule, i+1))
	}
	return strings.Join(lines, "\n")
}

// item is one rule as a list item, with a multi-line body kept inside it.
func (s Section) item(rule Rule, number int) string {
	body := strings.ReplaceAll(rule.Sent(), "\n", "\n   ")
	switch s.Style {
	case StyleBullets:
		return "- " + body
	case StyleNumbered:
		return strconv.Itoa(number) + ". " + body
	default:
		return "- **" + rule.ID + "** " + body
	}
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

// reportsBlock is the report shapes, at the end of the document.
func (d *Document) reportsBlock() string {
	blocks := make([]string, 0, len(d.Reports)+1)
	blocks = append(blocks, "## "+i18n.T("rules.reports_heading"))
	for _, report := range d.Reports {
		blocks = append(blocks, report.block())
	}
	return strings.Join(blocks, "\n\n")
}

// block is one report shape: its name, where it is written, and its fields.
func (r Report) block() string {
	fields := make([]string, 0, len(r.Fields))
	for _, field := range r.Fields {
		fields = append(fields, i18n.Tf("rules.report_field", field.Name, field.Description))
	}
	return i18n.Tf("rules.report_block", r.Name, r.Where, strings.Join(fields, "\n"))
}
