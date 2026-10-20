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
// report / tests, its origin.i / origin.i.date / origin.i.refs.j. Section and
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

	top, more := collectMapping(root, path, "the file", -1, -1, docKeys...)
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
		doc.Reports, more = collectReports(reports, path)
		problems = append(problems, more...)
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
	n, problems := collectMapping(value, path, label, section, rule, ruleKeys...)
	var r Rule

	_, hasID := n.atNode("id")
	_, hasText := n.atNode("text")
	if !hasID || !hasText {
		problems = append(problems, newProblem(path, section, rule, "", value.Line,
			i18n.Errorf("error.rules_rule_needs_id_text", path, label)))
		return r, problems
	}

	id, err := n.str("id")
	switch {
	case err != nil:
		problems = append(problems, newProblem(path, section, rule, "id", valueLine(n, "id"), err))
		return r, problems
	case id == "":
		problems = append(problems, newProblem(path, section, rule, "", valueLine(n, "id"),
			i18n.Errorf("error.rules_rule_needs_id_text", path, label)))
		return r, problems
	}
	r.ID = id
	if !RuleIDPattern.MatchString(r.ID) {
		problems = append(problems, newProblem(path, section, rule, "id", valueLine(n, "id"),
			i18n.Errorf("error.rules_id_pattern", path, n.at("id"), r.ID)))
	}

	if r.Text, err = n.str("text"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "text", valueLine(n, "text"), err))
	} else if strings.TrimSpace(r.Text) == "" {
		problems = append(problems, newProblem(path, section, rule, "text", valueLine(n, "text"),
			i18n.Errorf("error.rules_text_empty", path, label)))
	}

	return fillRuleAll(r, n, path, label, section, rule, problems)
}

// fillRuleAll reads what a rule may carry besides id and text, filing every
// problem, in the order the reader checks them.
func fillRuleAll(r Rule, n node, path, label string, section, rule int, problems []Problem) (Rule, []Problem) {
	var err error
	if r.Why, err = n.str("why"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "why", valueLine(n, "why"), err))
	}
	if r.Note, err = n.str("note"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "note", valueLine(n, "note"), err))
	}
	if send, ok, err := n.boolean("send_why"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "send_why", valueLine(n, "send_why"), err))
	} else if ok {
		r.SendWhy = &send
	}
	if r.Report, err = n.str("report"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "report", valueLine(n, "report"), err))
	}
	if tests, ok := n.atNode("tests"); ok {
		var more []Problem
		r.Tests, more = collectList(tests, path, n.at("tests"), section, rule, "tests")
		problems = append(problems, more...)
	}
	if origin, ok := n.atNode("origin"); ok {
		var more []Problem
		r.Origin, more = collectOrigins(origin, path, label, section, rule)
		problems = append(problems, more...)
	}
	return r, problems
}

// collectList is a list of strings, or one string standing for a one-item list,
// filing a problem for every item that is not a string.
func collectList(value *yaml.Node, path, label string, section, rule int, field string) ([]string, []Problem) {
	switch value.Kind {
	case yaml.ScalarNode:
		text, err := scalarString(value, path, label)
		if err != nil {
			return nil, []Problem{newProblem(path, section, rule, field, value.Line, err)}
		}
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []string{text}, nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		var problems []Problem
		for i, item := range value.Content {
			text, err := scalarString(item, path, fmt.Sprintf("%s[%d]", label, i))
			if err != nil {
				problems = append(problems, newProblem(path, section, rule, field, item.Line, err))
				continue
			}
			out = append(out, text)
		}
		return out, problems
	default:
		return nil, []Problem{newProblem(path, section, rule, field, value.Line, i18n.Errorf("error.rules_not_sequence", path, label))}
	}
}

// collectOrigins reads a rule's origins, filing every problem of every one.
func collectOrigins(value *yaml.Node, path, label string, section, rule int) ([]Origin, []Problem) {
	if value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, []Problem{newProblem(path, section, rule, "", value.Line,
			i18n.Errorf("error.rules_not_sequence", path, label+".origin"))}
	}
	out := make([]Origin, 0, len(value.Content))
	var problems []Problem
	for i, item := range value.Content {
		origin, more := collectOrigin(item, path, fmt.Sprintf("%s.origin[%d]", label, i), section, rule, i)
		out = append(out, origin)
		problems = append(problems, more...)
	}
	return out, problems
}

// collectOrigin reads one origin, filing every problem its keys have.
func collectOrigin(value *yaml.Node, path, label string, section, rule, index int) (Origin, []Problem) {
	field := fmt.Sprintf("origin.%d", index)
	n, problems := collectMapping(value, path, label, section, rule, originKeys...)
	var o Origin

	if kind, err := n.str("kind"); err != nil {
		problems = append(problems, newProblem(path, section, rule, field, valueLine(n, "kind"), err))
	} else {
		o.Kind = kind
		if !slices.Contains(kinds, o.Kind) {
			problems = append(problems, newProblem(path, section, rule, field, valueLine(n, "kind"),
				i18n.Errorf("error.rules_origin_kind", path, label, o.Kind)))
		}
	}
	if date, err := n.str("date"); err != nil {
		problems = append(problems, newProblem(path, section, rule, field+".date", valueLine(n, "date"), err))
	} else {
		o.Date = date
		if o.Date != "" && !isDate(o.Date) {
			problems = append(problems, newProblem(path, section, rule, field+".date", valueLine(n, "date"),
				i18n.Errorf("error.rules_origin_date", path, n.at("date"), o.Date)))
		}
	}
	if refs, ok := n.atNode("refs"); ok {
		var more []Problem
		o.Refs, more = collectRefs(refs, path, n.at("refs"), section, rule, index)
		problems = append(problems, more...)
	}
	if note, err := n.str("note"); err != nil {
		problems = append(problems, newProblem(path, section, rule, "note", valueLine(n, "note"), err))
	} else {
		o.Note = note
	}
	if o.Kind == KindDerived && len(o.Refs) == 0 {
		problems = append(problems, newProblem(path, section, rule, field, value.Line,
			i18n.Errorf("error.rules_derived_refs", path, label)))
	}
	return o, problems
}

// collectRefs is a derived rule's refs: one name per item, filed with the two
// problems a name can have (not a string, not a name). The field names the
// origin and, for an item, the ref, so a screen marks that ref's chip.
func collectRefs(value *yaml.Node, path, label string, section, rule, index int) ([]string, []Problem) {
	field := fmt.Sprintf("origin.%d.refs", index)
	switch value.Kind {
	case yaml.ScalarNode:
		text, err := scalarString(value, path, label)
		if err != nil {
			return nil, []Problem{newProblem(path, section, rule, field, value.Line, err)}
		}
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []string{text}, nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		var problems []Problem
		for j, item := range value.Content {
			text, err := scalarString(item, path, fmt.Sprintf("%s[%d]", label, j))
			if err != nil {
				problems = append(problems, newProblem(path, section, rule,
					fmt.Sprintf("%s.%d", field, j), item.Line, err))
				continue
			}
			out = append(out, text)
		}
		return out, problems
	default:
		return nil, []Problem{newProblem(path, section, rule, field, value.Line, i18n.Errorf("error.rules_not_sequence", path, label))}
	}
}

// collectReports reads the reports block, filing every problem of every report.
func collectReports(value *yaml.Node, path string) ([]Report, []Problem) {
	if value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, []Problem{newProblem(path, -1, -1, "", value.Line, i18n.Errorf("error.rules_not_mapping", path, "reports"))}
	}
	out := make([]Report, 0, len(value.Content)/2)
	var problems []Problem
	seen := make(map[string]bool, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		name := value.Content[i].Value
		if seen[name] {
			problems = append(problems, newProblem(path, -1, -1, "", value.Content[i].Line,
				i18n.Errorf("error.rules_duplicate_key", path, "reports", name)))
			continue
		}
		seen[name] = true
		label := "reports." + name
		n, more := collectMapping(value.Content[i+1], path, label, -1, -1, reportKeys...)
		problems = append(problems, more...)
		report := Report{Name: name}
		if where, err := n.str("where"); err != nil {
			problems = append(problems, newProblem(path, -1, -1, "reports."+name+".where", valueLine(n, "where"), err))
		} else {
			report.Where = where
		}
		if fields, ok := n.atNode("fields"); ok {
			var fieldProblems []Problem
			report.Fields, fieldProblems = collectFields(fields, path, label+".fields")
			problems = append(problems, fieldProblems...)
		}
		out = append(out, report)
	}
	return out, problems
}

// collectFields reads one report's fields, filing every problem of every one.
func collectFields(value *yaml.Node, path, label string) ([]ReportField, []Problem) {
	if value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, []Problem{newProblem(path, -1, -1, "", value.Line, i18n.Errorf("error.rules_not_mapping", path, label))}
	}
	out := make([]ReportField, 0, len(value.Content)/2)
	var problems []Problem
	seen := make(map[string]bool, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		name := value.Content[i].Value
		if seen[name] {
			problems = append(problems, newProblem(path, -1, -1, "", value.Content[i].Line,
				i18n.Errorf("error.rules_duplicate_key", path, label, name)))
			continue
		}
		seen[name] = true
		description, err := scalarString(value.Content[i+1], path, label+"."+name)
		if err != nil {
			problems = append(problems, newProblem(path, -1, -1, "", value.Content[i+1].Line, err))
			continue
		}
		out = append(out, ReportField{Name: name, Description: description})
	}
	return out, problems
}

// valueLine is the line of one key's value, or 0 when the key is not there.
func valueLine(n node, key string) int {
	if f, ok := n.get(key); ok && f.value != nil {
		return f.value.Line
	}
	return 0
}
