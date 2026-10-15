package rules

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Emit writes a document back as YAML in the one format hergy writes (d-cbac,
// spec/10 section 6): the keys in the order of spec/01 section 3, a value equal
// to a default and an empty value left out, a value of more than one line
// written as a literal block (|-, or | when the value keeps a last line break),
// and any other value on one line with the quotes yaml.v3 asks for.
//
// The format is decided once so a file hergy writes is the same file it reads
// back: Emit is idempotent, and the Markdown a member reads is unchanged. Two
// things a file may hold are not carried over, and both are in the CHANGELOG:
// a comment (F7) and a value spelled out that equals a default (send_why: true).
// The first save of a file thus rewrites the whole file and shows one large
// diff in git.
//
// The order of the keys comes from the same lists the parser reads a file with,
// so a key cannot move in one place and stay in the other.
func Emit(d *Document) (string, error) {
	values := map[string]*yaml.Node{
		"version":  emitInt(d.emitVersion()),
		"sections": emitSections(d.Sections),
	}
	if d.Title != "" {
		values["title"] = emitText(d.Title)
	}
	if d.Revision != nil {
		values["revision"] = emitInt(*d.Revision)
	}
	if len(d.Reports) > 0 {
		values["reports"] = emitReports(d.Reports)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(emitMapping(docKeys, values)); err != nil {
		return "", fmt.Errorf("emit %s: %w", d.Path, err)
	}
	if err := enc.Close(); err != nil {
		return "", fmt.Errorf("emit %s: %w", d.Path, err)
	}
	return buf.String(), nil
}

// emitVersion is the format version to write. A document built by an editor
// rather than read from a file has none, and this build writes version 1 only.
func (d *Document) emitVersion() int {
	if d.Version == 0 {
		return 1
	}
	return d.Version
}

func emitSections(sections []Section) *yaml.Node {
	items := make([]*yaml.Node, 0, len(sections))
	for _, section := range sections {
		values := map[string]*yaml.Node{}
		if section.Heading != "" {
			values["heading"] = emitText(section.Heading)
		}
		if section.Level != 0 {
			values["level"] = emitInt(section.Level)
		}
		if section.Intro != "" {
			values["intro"] = emitText(section.Intro)
		}
		if section.Style != "" {
			values["style"] = emitText(section.Style)
		}
		if len(section.Rules) > 0 {
			values["rules"] = emitRules(section.Rules)
		}
		items = append(items, emitMapping(sectionKeys, values))
	}
	return emitSequence(items)
}

func emitRules(rules []Rule) *yaml.Node {
	items := make([]*yaml.Node, 0, len(rules))
	for _, rule := range rules {
		values := map[string]*yaml.Node{
			"id":   emitText(rule.ID),
			"text": emitText(rule.Text),
		}
		if rule.Why != "" {
			values["why"] = emitText(rule.Why)
		}
		// The default is true, and a default is not written.
		if rule.SendWhy != nil && !*rule.SendWhy {
			values["send_why"] = emitFalse()
		}
		if rule.Note != "" {
			values["note"] = emitText(rule.Note)
		}
		if len(rule.Origin) > 0 {
			values["origin"] = emitOrigins(rule.Origin)
		}
		if rule.Report != "" {
			values["report"] = emitText(rule.Report)
		}
		if len(rule.Tests) > 0 {
			values["tests"] = emitList(rule.Tests)
		}
		items = append(items, emitMapping(ruleKeys, values))
	}
	return emitSequence(items)
}

func emitOrigins(origins []Origin) *yaml.Node {
	items := make([]*yaml.Node, 0, len(origins))
	for _, origin := range origins {
		values := map[string]*yaml.Node{"kind": emitText(origin.Kind)}
		if origin.Date != "" {
			values["date"] = emitText(origin.Date)
		}
		if len(origin.Refs) > 0 {
			values["refs"] = emitList(origin.Refs)
		}
		if origin.Note != "" {
			values["note"] = emitText(origin.Note)
		}
		items = append(items, emitMapping(originKeys, values))
	}
	return emitSequence(items)
}

// emitReports is the reports of a document as one mapping, each report under
// its name, in the written order.
func emitReports(reports []Report) *yaml.Node {
	names := make([]string, 0, len(reports))
	values := map[string]*yaml.Node{}
	for _, report := range reports {
		names = append(names, report.Name)
		body := map[string]*yaml.Node{}
		if report.Where != "" {
			body["where"] = emitText(report.Where)
		}
		if len(report.Fields) > 0 {
			body["fields"] = emitFields(report.Fields)
		}
		values[report.Name] = emitMapping(reportKeys, body)
	}
	return emitMapping(names, values)
}

// emitFields is the fields of one report as one mapping, each field under its
// name, in the written order.
func emitFields(fields []ReportField) *yaml.Node {
	names := make([]string, 0, len(fields))
	values := map[string]*yaml.Node{}
	for _, field := range fields {
		names = append(names, field.Name)
		values[field.Name] = emitText(field.Description)
	}
	return emitMapping(names, values)
}

// emitMapping is a mapping whose keys are written in the order given. A key
// with no value is left out, and a key given twice is left out the second time,
// so a mapping that a reader would refuse for a repeated key is not written.
func emitMapping(keys []string, values map[string]*yaml.Node) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil || seen[key] {
			continue
		}
		seen[key] = true
		n.Content = append(n.Content, emitText(key), value)
	}
	return n
}

func emitSequence(items []*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Content: items}
}

// emitText is one text value. A value with a line break is a literal block; any
// other value is one line and yaml.v3 adds quotes when the plain form would
// read as something else (a number, a date, a comment). The string tag is set
// so a text that looks like a number still comes back as text.
func emitText(text string) *yaml.Node {
	style := yaml.Style(0)
	if strings.Contains(text, "\n") {
		style = yaml.LiteralStyle
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: text, Style: style}
}

func emitList(values []string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(values))
	for _, value := range values {
		items = append(items, emitText(value))
	}
	return emitSequence(items)
}

func emitInt(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)}
}

// emitFalse is the only boolean the format writes: send_why: false, the one
// value of that key that is not the default.
func emitFalse() *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
}
