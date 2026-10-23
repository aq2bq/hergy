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
// a comment (F7) and a value spelled out that equals a default (send_why: true,
// in_prompt: true).
// A style: line of a section is not written either: the layout of a section is
// one way now (d-f4f6). The first save of a file thus rewrites the whole file
// and shows one large diff in git.
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
		if !rule.Included() {
			values["in_prompt"] = emitFalse()
			if rule.OffSince != "" {
				values["off_since"] = emitText(rule.OffSince)
			}
		}
		if rule.Note != "" {
			values["note"] = emitText(rule.Note)
		}
		items = append(items, emitMapping(ruleKeys, values))
	}
	return emitSequence(items)
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

func emitInt(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)}
}

// emitFalse is the only boolean the format writes: send_why: false and
// in_prompt: false, the one value of each of those keys that is not the default.
func emitFalse() *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
}
