// Package rules reads hergy's rule files: one YAML document per agreement
// (agreement.yaml) or role document (roles/<role>.yaml), and renders it to the
// Markdown a member's prompt carries.
//
// What a person keeps in the file and what an agent reads are different. The
// file carries the origin of each rule, a note for whoever maintains it, and
// the report or test that shows the rule was kept; the prompt carries only the
// text, its heading and, unless send_why says otherwise, its reason.
//
// The parser walks the YAML by hand rather than decoding into structs: it has
// to name the position of a break (sections[1].rules[2]), refuse a key that
// stands twice in one mapping, and keep the written order of reports.
package rules

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aq2bq/hergy/internal/i18n"
	"go.yaml.in/yaml/v3"
)

// RuleIDPattern is the shape a rule id has: an uppercase head with uppercase
// segments after it (C1, P-TURN-1, SPEC-AUD-5).
var RuleIDPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$`)

// Styles say how a section lays its rules out when it is sent.
const (
	StyleParagraphs = "paragraphs"
	StyleBullets    = "bullets"
	StyleNumbered   = "numbered"
	StyleIDs        = "ids"
)

// Kinds say where a rule came from.
const (
	KindIncident    = "incident"
	KindInstruction = "instruction"
	KindDerived     = "derived"
)

var (
	styles      = []string{StyleParagraphs, StyleBullets, StyleNumbered, StyleIDs}
	kinds       = []string{KindIncident, KindInstruction, KindDerived}
	dateLayout  = "2006-01-02"
	docKeys     = []string{"version", "title", "revision", "sections", "reports"}
	sectionKeys = []string{"heading", "level", "intro", "style", "rules"}
	ruleKeys    = []string{"id", "text", "why", "send_why", "note", "origin", "report", "tests"}
	originKeys  = []string{"kind", "date", "refs", "note"}
	reportKeys  = []string{"where", "fields"}
)

// Document is one rule file.
type Document struct {
	Version  int
	Title    string
	Revision *int // written for a person; never sent
	Sections []Section
	Reports  []Report
	Path     string // where it was read from, for errors and provenance
}

// Section is one heading with the rules under it. A section with no heading
// continues the one before it.
type Section struct {
	Heading string
	Level   int // 0 means the default level, 2
	Intro   string
	Style   string // empty means paragraphs
	Rules   []Rule
}

// Rule is one rule of a document.
type Rule struct {
	ID      string
	Text    string
	Why     string
	SendWhy *bool // nil means true
	Note    string
	Origin  []Origin
	Report  string // "report.field"
	Tests   []string
}

// Origin is where one rule came from.
type Origin struct {
	Kind string // incident, instruction or derived
	Date string // "" or YYYY-MM-DD
	Refs []string
	Note string
}

// Report is one output contract: a name, where it is written, and its fields.
type Report struct {
	Name   string
	Where  string
	Fields []ReportField
}

// ReportField is one field of a report.
type ReportField struct {
	Name        string
	Description string
}

// Rules is every rule of the document, in the written order.
func (d *Document) Rules() []Rule {
	var out []Rule
	for _, section := range d.Sections {
		out = append(out, section.Rules...)
	}
	return out
}

// Parse reads one rule file. path is where the text came from, and it is what
// an error names.
func Parse(text, path string) (*Document, error) {
	root, err := decode(text, path)
	if err != nil {
		return nil, err
	}
	top, err := openMapping(root, path, "the file", docKeys...)
	if err != nil {
		return nil, err
	}
	doc := &Document{Path: path}
	version, err := versionOf(top)
	if err != nil {
		return nil, err
	}
	doc.Version = version
	if doc.Title, err = top.str("title"); err != nil {
		return nil, err
	}
	if revision, ok, err := top.integer("revision"); err != nil {
		return nil, err
	} else if ok {
		doc.Revision = &revision
	}
	if sections, ok := top.atNode("sections"); !ok {
		return nil, i18n.Errorf("error.rules_sections_missing", path)
	} else if doc.Sections, err = parseSections(sections, path); err != nil {
		return nil, err
	}
	if reports, ok := top.atNode("reports"); ok {
		if doc.Reports, err = parseReports(reports, path); err != nil {
			return nil, err
		}
	}
	return doc, nil
}

// decode reads the YAML into a node tree so the parser can walk it.
func decode(text, path string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.NewDecoder(strings.NewReader(text)).Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, i18n.Errorf("error.file_empty", filepath.Base(path), path)
		}
		return nil, fmt.Errorf("%s: %s", path, strings.TrimSpace(err.Error()))
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &doc, nil
}

// versionOf holds the file to the one version this build reads.
func versionOf(n node) (int, error) {
	version, ok, err := n.integer("version")
	if err != nil || !ok {
		return 0, i18n.Errorf("error.version_missing", filepath.Base(n.path), n.path)
	}
	if version != 1 {
		return 0, i18n.Errorf("error.version_wrong", filepath.Base(n.path), version, n.path)
	}
	return version, nil
}

func parseSections(value *yaml.Node, path string) ([]Section, error) {
	if value.Tag == "!!null" || (value.Kind == yaml.SequenceNode && len(value.Content) == 0) {
		return nil, i18n.Errorf("error.rules_sections_empty", path)
	}
	if value.Kind != yaml.SequenceNode {
		return nil, i18n.Errorf("error.rules_not_sequence", path, "sections")
	}
	out := make([]Section, 0, len(value.Content))
	for i, item := range value.Content {
		section, err := parseSection(item, path, fmt.Sprintf("sections[%d]", i))
		if err != nil {
			return nil, err
		}
		out = append(out, section)
	}
	return out, nil
}

func parseSection(value *yaml.Node, path, label string) (Section, error) {
	n, err := openMapping(value, path, label, sectionKeys...)
	if err != nil {
		return Section{}, err
	}
	var s Section
	if s.Heading, err = n.str("heading"); err != nil {
		return Section{}, err
	}
	if err := sectionLevel(&s, n, path, label); err != nil {
		return Section{}, err
	}
	if s.Intro, err = n.str("intro"); err != nil {
		return Section{}, err
	}
	if s.Style, err = n.str("style"); err != nil {
		return Section{}, err
	}
	if s.Style != "" && !slices.Contains(styles, s.Style) {
		return Section{}, i18n.Errorf("error.rules_style_unknown", path, n.at("style"), s.Style)
	}
	if rules, ok := n.atNode("rules"); ok {
		if s.Rules, err = parseRules(rules, path, label+".rules"); err != nil {
			return Section{}, err
		}
	}
	return s, nil
}

// sectionLevel reads level, which a section without a heading must not write.
func sectionLevel(s *Section, n node, path, label string) error {
	level, ok, err := n.integer("level")
	if err != nil || !ok {
		return err
	}
	if s.Heading == "" {
		return i18n.Errorf("error.rules_level_without_heading", path, label)
	}
	if level < 2 || level > 4 {
		return i18n.Errorf("error.rules_level_range", path, n.at("level"), level)
	}
	s.Level = level
	return nil
}

func parseRules(value *yaml.Node, path, label string) ([]Rule, error) {
	if value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, i18n.Errorf("error.rules_not_sequence", path, label)
	}
	out := make([]Rule, 0, len(value.Content))
	for i, item := range value.Content {
		itemLabel := fmt.Sprintf("%s[%d]", label, i)
		rule, err := parseRule(item, path, itemLabel)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, nil
}

func parseRule(value *yaml.Node, path, label string) (Rule, error) {
	n, err := openMapping(value, path, label, ruleKeys...)
	if err != nil {
		return Rule{}, err
	}
	var r Rule
	if _, hasID := n.atNode("id"); !hasID {
		return Rule{}, i18n.Errorf("error.rules_rule_needs_id_text", path, label)
	}
	if _, hasText := n.atNode("text"); !hasText {
		return Rule{}, i18n.Errorf("error.rules_rule_needs_id_text", path, label)
	}
	id, err := n.str("id")
	if err != nil {
		return Rule{}, err
	}
	if id == "" {
		return Rule{}, i18n.Errorf("error.rules_rule_needs_id_text", path, label)
	}
	r.ID = id
	if !RuleIDPattern.MatchString(r.ID) {
		return Rule{}, i18n.Errorf("error.rules_id_pattern", path, n.at("id"), r.ID)
	}
	if r.Text, err = n.str("text"); err != nil {
		return Rule{}, err
	}
	if strings.TrimSpace(r.Text) == "" {
		return Rule{}, i18n.Errorf("error.rules_text_empty", path, label)
	}
	return fillRule(r, n, path, label)
}

// fillRule reads what a rule may carry besides id and text.
func fillRule(r Rule, n node, path, label string) (Rule, error) {
	var err error
	if r.Why, err = n.str("why"); err != nil {
		return Rule{}, err
	}
	if r.Note, err = n.str("note"); err != nil {
		return Rule{}, err
	}
	if send, ok, err := n.boolean("send_why"); err != nil {
		return Rule{}, err
	} else if ok {
		r.SendWhy = &send
	}
	if r.Report, err = n.str("report"); err != nil {
		return Rule{}, err
	}
	if tests, ok := n.atNode("tests"); ok {
		if r.Tests, err = listOf(tests, path, n.at("tests")); err != nil {
			return Rule{}, err
		}
	}
	if origin, ok := n.atNode("origin"); ok {
		if r.Origin, err = parseOrigins(origin, path, label); err != nil {
			return Rule{}, err
		}
	}
	return r, nil
}

func parseOrigins(value *yaml.Node, path, label string) ([]Origin, error) {
	if value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, i18n.Errorf("error.rules_not_sequence", path, label+".origin")
	}
	out := make([]Origin, 0, len(value.Content))
	for i, item := range value.Content {
		itemLabel := fmt.Sprintf("%s.origin[%d]", label, i)
		origin, err := parseOrigin(item, path, itemLabel)
		if err != nil {
			return nil, err
		}
		out = append(out, origin)
	}
	return out, nil
}

func parseOrigin(value *yaml.Node, path, label string) (Origin, error) {
	n, err := openMapping(value, path, label, originKeys...)
	if err != nil {
		return Origin{}, err
	}
	var o Origin
	if o.Kind, err = n.str("kind"); err != nil {
		return Origin{}, err
	}
	if !slices.Contains(kinds, o.Kind) {
		return Origin{}, i18n.Errorf("error.rules_origin_kind", path, label, o.Kind)
	}
	if o.Date, err = n.str("date"); err != nil {
		return Origin{}, err
	}
	if o.Date != "" && !isDate(o.Date) {
		return Origin{}, i18n.Errorf("error.rules_origin_date", path, n.at("date"), o.Date)
	}
	if o.Refs, err = n.list("refs"); err != nil {
		return Origin{}, err
	}
	if o.Note, err = n.str("note"); err != nil {
		return Origin{}, err
	}
	if o.Kind == KindDerived && len(o.Refs) == 0 {
		return Origin{}, i18n.Errorf("error.rules_derived_refs", path, label)
	}
	return o, nil
}

func parseReports(value *yaml.Node, path string) ([]Report, error) {
	if value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, i18n.Errorf("error.rules_not_mapping", path, "reports")
	}
	out := make([]Report, 0, len(value.Content)/2)
	seen := make(map[string]bool, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		name := value.Content[i].Value
		if seen[name] {
			return nil, i18n.Errorf("error.rules_duplicate_key", path, "reports", name)
		}
		seen[name] = true
		label := "reports." + name
		n, err := openMapping(value.Content[i+1], path, label, reportKeys...)
		if err != nil {
			return nil, err
		}
		report := Report{Name: name}
		if report.Where, err = n.str("where"); err != nil {
			return nil, err
		}
		if fields, ok := n.atNode("fields"); ok {
			if report.Fields, err = parseFields(fields, path, label+".fields"); err != nil {
				return nil, err
			}
		}
		out = append(out, report)
	}
	return out, nil
}

func parseFields(value *yaml.Node, path, label string) ([]ReportField, error) {
	if value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, i18n.Errorf("error.rules_not_mapping", path, label)
	}
	out := make([]ReportField, 0, len(value.Content)/2)
	seen := make(map[string]bool, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		name := value.Content[i].Value
		if seen[name] {
			return nil, i18n.Errorf("error.rules_duplicate_key", path, label, name)
		}
		seen[name] = true
		description, err := scalarString(value.Content[i+1], path, label+"."+name)
		if err != nil {
			return nil, err
		}
		out = append(out, ReportField{Name: name, Description: description})
	}
	return out, nil
}

func isDate(value string) bool {
	parsed, err := time.Parse(dateLayout, value)
	return err == nil && parsed.Format(dateLayout) == value
}

// node is one mapping being read: its fields, the file, and the position used
// in errors.
type node struct {
	fields []field
	path   string
	label  string
}

// field is one key and its value.
type field struct {
	key   string
	value *yaml.Node
}

// openMapping reads a mapping, refusing a repeated key and a key not allowed at
// this position.
func openMapping(value *yaml.Node, path, label string, allowed ...string) (node, error) {
	if value.Kind != yaml.MappingNode {
		return node{}, i18n.Errorf("error.rules_not_mapping", path, label)
	}
	n := node{path: path, label: label, fields: make([]field, 0, len(value.Content)/2)}
	seen := make(map[string]bool, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		if seen[key] {
			return node{}, i18n.Errorf("error.rules_duplicate_key", path, label, key)
		}
		seen[key] = true
		if !slices.Contains(allowed, key) {
			return node{}, i18n.Errorf("error.rules_unknown_key", path, label, key)
		}
		n.fields = append(n.fields, field{key: key, value: value.Content[i+1]})
	}
	return n, nil
}

func (n node) get(key string) (field, bool) {
	for _, f := range n.fields {
		if f.key == key {
			return f, true
		}
	}
	return field{}, false
}

// at is the position of one key, for an error message.
func (n node) at(key string) string { return n.label + "." + key }

// atNode is the value node of a key.
func (n node) atNode(key string) (*yaml.Node, bool) {
	if f, ok := n.get(key); ok {
		return f.value, true
	}
	return nil, false
}

// str is the text of a key, empty when the key is absent, null or empty.
func (n node) str(key string) (string, error) {
	f, ok := n.get(key)
	if !ok {
		return "", nil
	}
	return f.string(n.path, n.at(key))
}

func (n node) integer(key string) (int, bool, error) {
	f, ok := n.get(key)
	if !ok {
		return 0, false, nil
	}
	value, err := f.integer(n.path, n.at(key))
	return value, true, err
}

func (n node) boolean(key string) (bool, bool, error) {
	f, ok := n.get(key)
	if !ok {
		return false, false, nil
	}
	value, err := f.boolean(n.path, n.at(key))
	return value, true, err
}

func (n node) list(key string) ([]string, error) {
	f, ok := n.get(key)
	if !ok {
		return nil, nil
	}
	return listOf(f.value, n.path, n.at(key))
}

func (f field) string(path, label string) (string, error) {
	return scalarString(f.value, path, label)
}

func (f field) integer(path, label string) (int, error) {
	if f.value.Tag != "!!int" {
		return 0, i18n.Errorf("error.rules_not_integer", path, label)
	}
	var value int
	if err := f.value.Decode(&value); err != nil {
		return 0, i18n.Errorf("error.rules_not_integer", path, label)
	}
	return value, nil
}

func (f field) boolean(path, label string) (bool, error) {
	if f.value.Tag != "!!bool" {
		return false, i18n.Errorf("error.rules_not_boolean", path, label)
	}
	var value bool
	if err := f.value.Decode(&value); err != nil {
		return false, i18n.Errorf("error.rules_not_boolean", path, label)
	}
	return value, nil
}

// scalarString is a scalar as text. A date written without quotes comes back as
// a timestamp, and is still the text the file wrote.
func scalarString(value *yaml.Node, path, label string) (string, error) {
	if value.Tag == "!!null" {
		return "", nil
	}
	if value.Kind != yaml.ScalarNode || (value.Tag != "!!str" && value.Tag != "!!timestamp") {
		return "", i18n.Errorf("error.rules_not_string", path, label)
	}
	return value.Value, nil
}

// listOf is a list of strings, or one string standing for a one-item list.
func listOf(value *yaml.Node, path, label string) ([]string, error) {
	switch value.Kind {
	case yaml.ScalarNode:
		text, err := scalarString(value, path, label)
		if err != nil || strings.TrimSpace(text) == "" {
			return nil, err
		}
		return []string{text}, nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for i, item := range value.Content {
			text, err := scalarString(item, path, fmt.Sprintf("%s[%d]", label, i))
			if err != nil {
				return nil, err
			}
			out = append(out, text)
		}
		return out, nil
	default:
		return nil, i18n.Errorf("error.rules_not_sequence", path, label)
	}
}
