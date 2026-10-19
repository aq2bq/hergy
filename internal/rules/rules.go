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
// an error names. It returns the document when the file holds no problem, and
// the file's first problem when it holds one: the whole reading is ReadAll, so
// a screen that shows every problem and the command line that stops at the
// first cannot say different sentences (C2).
func Parse(text, path string) (*Document, error) {
	doc, problems := ReadAll(text, path)
	if len(problems) > 0 {
		return nil, problems[0].err
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

// get is one key's field, and whether the mapping holds it.
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
