package rules

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"go.yaml.in/yaml/v3"
)

// This file reads one rule file the way a screen needs it: not stopping at the
// first problem, but gathering every problem the file has and the document as
// far as it reads. The command line reads its first problem from here too, so a
// screen and hergy validate say the same sentence (spec-roles 6 C2).

// The two severities a Problem carries, as the screen and a JSON answer write
// them (the words the profiles screen uses too): an error stops the file from
// reading, a warning does not. A rule file's own shape gives errors and, for a
// style: line a file still writes, one warning; a warning is also added by the
// checks that span documents (CheckAll) and by the model-name check
// (ModelWords).
const (
	// SevError is a problem that stops the file from reading.
	SevError = "err"
	// SevWarning is a problem the file reads through.
	SevWarning = "warn"
)

// Problem is one thing wrong with a rule file. Field is where it stands, empty
// for the file as a whole: the keys title and revision, a section's
// sections[i].heading / .level / .intro / .style, a rule's id / text / why /
// send_why / in_prompt / report / tests, its origin.i / origin.i.date /
// origin.i.refs.j. Section and
// Rule are the rule's place, or -1 when the problem is not about one rule. Line
// is the YAML line the problem stands on, or 0 when it stands on none. The err
// is the value Parse returns for the file's first problem.
type Problem struct {
	Sev     string
	Path    string
	Section int
	Rule    int
	Field   string
	Line    int
	Text    string
	err     error
	// ruleID is the rule a problem that spans documents is about, when Section
	// and Rule alone do not say it: a name that stands twice, or a rule whose
	// report or derived ref is not there. A screen checking one document uses it
	// to place the problem on that document's rule (check.Role). It is not part
	// of the exported shape because a person never reads the id twice.
	ruleID string
}

// RuleID is the rule a problem that spans documents is about, or "" when the
// problem is about no single rule. Section and Rule name the rule where the
// problem was found (which may be another document than the one a screen is
// checking); a caller checking one document uses RuleID to place the problem on
// that document's own rule (check.Role).
func (p Problem) RuleID() string { return p.ruleID }

// newProblem is one problem, with the error Parse returns for it kept whole.
func newProblem(path string, section, rule int, field string, line int, err error) Problem {
	return Problem{
		Sev: SevError, Path: path, Section: section, Rule: rule,
		Field: field, Line: line, Text: err.Error(), err: err,
	}
}

// newWarning is one problem a file reads through: the sentence a person reads,
// and no error to stop the file with.
func newWarning(path string, section, rule int, field string, line int, err error) Problem {
	return Problem{
		Sev: SevWarning, Path: path, Section: section, Rule: rule,
		Field: field, Line: line, Text: err.Error(),
	}
}

// ReadAll reads one rule file into the document it holds and every problem it
// has, in the order the file writes them. A file that does not read at all
// (empty, broken YAML, a version hergy does not read) gives its problems and the
// document as far as it was read. The first error is the one Parse returns.
func ReadAll(text, path string) (*Document, []Problem) {
	root, err := decode(text, path)
	if err != nil {
		return nil, []Problem{newProblem(path, -1, -1, "", 0, err)}
	}
	if root.Kind != yaml.MappingNode {
		p := newProblem(path, -1, -1, "", root.Line, i18n.Errorf("error.rules_not_mapping", path, "the file"))
		return &Document{Path: path}, []Problem{p}
	}
	doc := &Document{Path: path}
	var problems []Problem

	top, more := collectMapping(root, path, "the file", -1, -1, append(append([]string{}, docKeys...), "reports")...)
	problems = append(problems, more...)

	switch version, ok, err := top.integer("version"); {
	case err != nil || !ok:
		problems = append(problems, newProblem(path, -1, -1, "", valueLine(top, "version"),
			i18n.Errorf("error.version_missing", filepath.Base(path), path)))
	case version != 1:
		problems = append(problems, newProblem(path, -1, -1, "", valueLine(top, "version"),
			i18n.Errorf("error.version_wrong", filepath.Base(path), version, path)))
	default:
		doc.Version = version
	}

	if title, err := top.str("title"); err != nil {
		problems = append(problems, newProblem(path, -1, -1, "title", valueLine(top, "title"), err))
	} else {
		doc.Title = title
	}

	if revision, ok, err := top.integer("revision"); err != nil {
		problems = append(problems, newProblem(path, -1, -1, "revision", valueLine(top, "revision"), err))
	} else if ok {
		doc.Revision = &revision
	}

	if sections, ok := top.atNode("sections"); !ok {
		problems = append(problems, newProblem(path, -1, -1, "", 0, i18n.Errorf("error.rules_sections_missing", path)))
	} else {
		doc.Sections, more = collectSections(sections, path)
		problems = append(problems, more...)
	}
	if reports, ok := top.atNode("reports"); ok {
		problems = append(problems, oldReportsProblems(reports, path)...)
	}

	return doc, problems
}

// collectMapping reads a mapping, filing a repeated key and a key not allowed at
// this position, and going on with the keys that are allowed.
func collectMapping(value *yaml.Node, path, label string, section, rule int, allowed ...string) (node, []Problem) {
	if value.Kind != yaml.MappingNode {
		return node{}, []Problem{newProblem(path, section, rule, "", value.Line,
			i18n.Errorf("error.rules_not_mapping", path, label))}
	}
	n := node{path: path, label: label, fields: make([]field, 0, len(value.Content)/2)}
	var problems []Problem
	seen := make(map[string]bool, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if seen[key] {
			problems = append(problems, newProblem(path, section, rule, "", value.Content[i].Line,
				i18n.Errorf("error.rules_duplicate_key", path, label, key)))
			continue
		}
		seen[key] = true
		if !slices.Contains(allowed, key) {
			problems = append(problems, newProblem(path, section, rule, "", value.Content[i].Line,
				i18n.Errorf("error.rules_unknown_key", path, label, key)))
			continue
		}
		n.fields = append(n.fields, field{key: key, value: value.Content[i+1]})
	}
	return n, problems
}

// collectSections reads the sections, filing every problem of every one.
func collectSections(value *yaml.Node, path string) ([]Section, []Problem) {
	if value.Tag == "!!null" || (value.Kind == yaml.SequenceNode && len(value.Content) == 0) {
		return nil, []Problem{newProblem(path, -1, -1, "", value.Line, i18n.Errorf("error.rules_sections_empty", path))}
	}
	if value.Kind != yaml.SequenceNode {
		return nil, []Problem{newProblem(path, -1, -1, "", value.Line, i18n.Errorf("error.rules_not_sequence", path, "sections"))}
	}
	out := make([]Section, 0, len(value.Content))
	var problems []Problem
	for i, item := range value.Content {
		section, more := collectSection(item, path, fmt.Sprintf("sections[%d]", i), i)
		out = append(out, section)
		problems = append(problems, more...)
	}
	return out, problems
}

// collectSection reads one section, filing every problem its keys and rules have.
func collectSection(value *yaml.Node, path, label string, index int) (Section, []Problem) {
	n, problems := collectMapping(value, path, label, index, -1, sectionKeys...)
	var s Section

	if heading, err := n.str("heading"); err != nil {
		problems = append(problems, newProblem(path, index, -1, sectionField(index, "heading"), valueLine(n, "heading"), err))
	} else {
		s.Heading = heading
	}

	if level, ok, err := n.integer("level"); err != nil {
		problems = append(problems, newProblem(path, index, -1, sectionField(index, "level"), valueLine(n, "level"), err))
	} else if ok {
		switch {
		case s.Heading == "":
			problems = append(problems, newProblem(path, index, -1, sectionField(index, "level"), valueLine(n, "level"),
				i18n.Errorf("error.rules_level_without_heading", path, label)))
		case level < 2 || level > 4:
			problems = append(problems, newProblem(path, index, -1, sectionField(index, "level"), valueLine(n, "level"),
				i18n.Errorf("error.rules_level_range", path, n.at("level"), level)))
		default:
			s.Level = level
		}
	}

	if intro, err := n.str("intro"); err != nil {
		problems = append(problems, newProblem(path, index, -1, sectionField(index, "intro"), valueLine(n, "intro"), err))
	} else {
		s.Intro = intro
	}

	if style, ok := n.atNode("style"); ok {
		// The layout is one way now, so a style: line is left alone and named
		// once: the file keeps reading, and the person is told to take the line
		// out (d-f4f6).
		problems = append(problems, newWarning(path, index, -1, sectionField(index, "style"), style.Line,
			i18n.Errorf("rules.style_gone", path, sectionName(s, index))))
	}

	if rules, ok := n.atNode("rules"); ok {
		var more []Problem
		s.Rules, more = collectRules(rules, path, label+".rules", index)
		problems = append(problems, more...)
	}
	return s, problems
}

// sectionField is where a section's own key stands: sections[i].key.
func sectionField(index int, key string) string {
	return fmt.Sprintf("sections[%d].%s", index, key)
}

// sectionName is how a sentence names a section: its heading, or, when the
// section has none and continues the one before it, the place the file writes it
// in.
func sectionName(s Section, index int) string {
	if s.Heading != "" {
		return s.Heading
	}
	return fmt.Sprintf("sections[%d]", index)
}

// collectRules reads the rules of one section, filing every problem of every one.
func collectRules(value *yaml.Node, path, label string, section int) ([]Rule, []Problem) {
	if value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, []Problem{newProblem(path, section, -1, fmt.Sprintf("sections[%d]", section), value.Line,
			i18n.Errorf("error.rules_not_sequence", path, label))}
	}
	out := make([]Rule, 0, len(value.Content))
	var problems []Problem
	for i, item := range value.Content {
		rule, more := collectRule(item, path, fmt.Sprintf("%s[%d]", label, i), section, i)
		out = append(out, rule)
		problems = append(problems, more...)
	}
	return out, problems
}

// collectRule reads one rule, filing every problem its keys have.
func collectRule(value *yaml.Node, path, label string, section, rule int) (Rule, []Problem) {
	n, problems := collectMapping(value, path, label, section, rule, append(append([]string{}, ruleKeys...), oldRuleKeys...)...)
	var r Rule

	_, hasText := n.atNode("text")
	if !hasText {
		problems = append(problems, newProblem(path, section, rule, "", value.Line,
			i18n.Errorf("error.rules_rule_needs_id_text", path, label)))
		return r, problems
	}

	// An id is hergy's now, so a file without one is a file hergy has not
	// written yet, not a broken one: the next write gives it one (d-4aab). An
	// id that is there, in whatever shape, is read as it stands.
	if _, ok := n.atNode("id"); ok {
		id, err := n.str("id")
		if err != nil {
			problems = append(problems, newProblem(path, section, rule, "id", valueLine(n, "id"), err))
			return r, problems
		}
		r.ID = id
	}
	for _, old := range oldRuleKeys {
		if oldKey, ok := n.atNode(old); ok {
			problems = append(problems, newProblem(path, section, rule, old, oldKey.Line,
				i18n.Errorf("error.rules_old_item", path, r.ID, old, oldAdvice(old))))
		}
	}

	text, err := n.str("text")
	if err != nil {
		problems = append(problems, newProblem(path, section, rule, "text", valueLine(n, "text"), err))
	} else if r.Text = text; strings.TrimSpace(r.Text) == "" {
		problems = append(problems, newProblem(path, section, rule, "text", valueLine(n, "text"),
			i18n.Errorf("error.rules_text_empty", path, label)))
	}

	return fillRuleAll(r, n, path, label, section, rule, problems)
}

// fillRuleAll reads what a rule may carry besides id and text, filing every
// problem, in the order the reader checks them.
func fillRuleAll(r Rule, n node, path, _ string, section, rule int, problems []Problem) (Rule, []Problem) {
	var err error
	if r.Note, err = n.str("note"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "note", valueLine(n, "note"), err))
	}
	if in, ok, err := n.boolean("in_prompt"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "in_prompt", valueLine(n, "in_prompt"), err))
	} else if ok {
		r.InPrompt = &in
	}
	if off, err := n.str("off_since"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "off_since", valueLine(n, "off_since"), err))
	} else if off != "" {
		switch {
		case !isDate(off):
			problems = append(problems, newProblem(path, section, rule, "off_since", valueLine(n, "off_since"),
				i18n.Errorf("error.rules_off_since_date", path, off)))
		case r.Included():
			// in_prompt says the rule is on; an off_since left behind is read as
			// absent and hergy drops it when it next writes the rule (d-9f30).
			r.OffSince = ""
		default:
			r.OffSince = off
		}
	}
	return r, problems
}

// oldAdvice says where an old rule key's content goes (r-81be).
func oldAdvice(key string) string {
	switch key {
	case "why", "send_why":
		return i18n.T("rules.old_why_advice")
	case "origin":
		return i18n.T("rules.old_origin_advice")
	default:
		return i18n.T("rules.old_check_advice")
	}
}

func oldReportsProblems(value *yaml.Node, path string) []Problem {
	if value.Kind != yaml.MappingNode {
		return []Problem{newProblem(path, -1, -1, "reports", value.Line,
			i18n.Errorf("error.rules_reports_gone", path, "reports"))}
	}
	if len(value.Content) == 0 {
		return []Problem{newProblem(path, -1, -1, "reports", value.Line,
			i18n.Errorf("error.rules_reports_gone", path, "reports"))}
	}
	var out []Problem
	for i := 0; i+1 < len(value.Content); i += 2 {
		name := value.Content[i].Value
		out = append(out, newProblem(path, -1, -1, "reports."+name, value.Content[i].Line,
			i18n.Errorf("error.rules_reports_gone", path, "reports."+name)))
	}
	return out
}

// valueLine is the line of one key's value, or 0 when the key is not there.
func valueLine(n node, key string) int {
	if f, ok := n.get(key); ok && f.value != nil {
		return f.value.Line
	}
	return 0
}
