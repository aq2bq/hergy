package rules

import (
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
)

// The editor changes a document where the screen already points: a section by
// its place in Sections, a rule by its place in that section. The screen draws
// a document and edits the same document, so the places agree; a place that is
// out of range is refused rather than writing somewhere else.
//
// The editor does not judge the content. The screen sends the edited document
// through Emit and Parse (and Check across the documents a member reads) and
// refuses to save what breaks, so the judgement lives in one place, the reader.

// AddRule puts a rule at the end of one section.
func (d *Document) AddRule(section int, rule Rule) error {
	if err := d.checkSection(section); err != nil {
		return err
	}
	d.Sections[section].Rules = append(d.Sections[section].Rules, rule)
	return nil
}

// SetRule replaces the rule at one place with the edited one. The place is
// where the rule stands now, so a rule may be given a new id.
func (d *Document) SetRule(section, rule int, next Rule) error {
	if err := d.checkRule(section, rule); err != nil {
		return err
	}
	d.Sections[section].Rules[rule] = next
	return nil
}

// DeleteRule takes the rule at one place out of its section.
func (d *Document) DeleteRule(section, rule int) error {
	if err := d.checkRule(section, rule); err != nil {
		return err
	}
	rules := d.Sections[section].Rules
	kept := make([]Rule, 0, len(rules)-1)
	kept = append(kept, rules[:rule]...)
	kept = append(kept, rules[rule+1:]...)
	d.Sections[section].Rules = kept
	return nil
}

// SectionMarkdown is one section as a member reads it: its heading, its intro
// and its rules, in the written order. The reports are left out because they
// close the whole document, not the section. The screen shows this while a
// person edits, so the effect of the edit is visible before it is saved.
func (d *Document) SectionMarkdown(section int) (string, error) {
	if err := d.checkSection(section); err != nil {
		return "", err
	}
	blocks := d.Sections[section].blocks()
	if len(blocks) == 0 {
		return "", nil
	}
	return strings.Join(blocks, "\n\n") + "\n", nil
}

func (d *Document) checkSection(section int) error {
	if section < 0 || section >= len(d.Sections) {
		return i18n.Errorf("error.rules_section_out_of_range", d.Path, section)
	}
	return nil
}

func (d *Document) checkRule(section, rule int) error {
	if err := d.checkSection(section); err != nil {
		return err
	}
	if rule < 0 || rule >= len(d.Sections[section].Rules) {
		return i18n.Errorf("error.rules_rule_out_of_range", d.Path, section, rule)
	}
	return nil
}
