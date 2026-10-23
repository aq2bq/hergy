package rules

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// This file answers where a problem's field stands in the YAML, so the YAML tab
// can mark the line and put the cursor on it (spec-roles 6 C6). check.Role knows
// a problem's field (the screen's own vocabulary: text, id, origin.i.refs.j) but
// not the file's shape, so the shape is read here and the field is looked up
// against it.

// Lines is where each place a problem can name stands in one rule file, keyed by
// a field of the screen's vocabulary. A place that is not in the file answers 0,
// which the caller reads as the file as a whole.
type Lines struct {
	lines map[string]int
}

// LinesOf reads one rule file's lines, or answers nothing when the file does not
// read. A file that does not read has no line to point at: its own problems
// carry the line they were read with.
func LinesOf(text, path string) Lines {
	out := Lines{lines: map[string]int{}}
	root, err := decode(text, path)
	if err != nil || root.Kind != yaml.MappingNode {
		return out
	}
	top := mappingOf(root)
	out.put("d:title", at(top, "title"))
	out.put("d:revision", at(top, "revision"))
	sections := top["sections"]
	if sections == nil || sections.Kind != yaml.SequenceNode {
		return out
	}
	for si, item := range sections.Content {
		section := mappingOf(item)
		for _, key := range []string{"heading", "level", "intro", "style"} {
			out.put(fmt.Sprintf("s:%d:%s", si, key), at(section, key))
		}
		rules := section["rules"]
		if rules == nil || rules.Kind != yaml.SequenceNode {
			continue
		}
		for ri, rule := range rules.Content {
			out.fillRule(si, ri, rule)
		}
	}
	return out
}

// At is the line of one problem's field: a rule's field when the problem is
// about a rule, a section's own key when it is about a section, and the file's
// title or revision when it is about neither. A field that is not in the file
// answers 0.
func (l Lines) At(section, rule int, field string) int {
	switch {
	case section < 0 && rule < 0:
		return l.lines["d:"+field]
	case rule < 0:
		key := field
		if i := strings.LastIndex(key, "."); i >= 0 {
			key = key[i+1:]
		}
		return l.lines[fmt.Sprintf("s:%d:%s", section, key)]
	default:
		return l.lines[fmt.Sprintf("r:%d:%d:%s", section, rule, field)]
	}
}

// fillRule records one rule's own fields.
func (l Lines) fillRule(section, rule int, item *yaml.Node) {
	rm := mappingOf(item)
	put := func(field string, line int) {
		l.put(fmt.Sprintf("r:%d:%d:%s", section, rule, field), line)
	}
	for _, key := range ruleKeys {
		put(key, at(rm, key))
	}

}

// put records a line, leaving out the places that are not in the file.
func (l Lines) put(key string, line int) {
	if line > 0 {
		l.lines[key] = line
	}
}

// mappingOf is one mapping's keys and their values, the first of a repeated key
// winning, the way the reader reads it.
func mappingOf(value *yaml.Node) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	if value == nil || value.Kind != yaml.MappingNode {
		return out
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if _, ok := out[value.Content[i].Value]; !ok {
			out[value.Content[i].Value] = value.Content[i+1]
		}
	}
	return out
}

// at is the line of one key's value, or 0 when the key is not there.
func at(m map[string]*yaml.Node, key string) int {
	if v, ok := m[key]; ok && v != nil {
		return v.Line
	}
	return 0
}
