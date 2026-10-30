package definition

import (
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/aq2bq/hergy/internal/yamledit"
)

// YAMLLine is one line of a team.yaml as the topology tab's preview shows it
// (spec-teams B7, C10): the line's own text, and what the line stands for.
//
// At is "description" for the description's lines, "node:<member>" for the lines of one
// members[] entry, "worker:<index>" for the lines of one workers[] entry (the
// index is the entry's place in the sequence), and "turn:<a>-><b>" for one
// turns[] line. Every other line — the version, the keys themselves, a blank
// line, a comment — answers "".
type YAMLLine struct {
	Text string `json:"text"`
	At   string `json:"at"`
}

// YAMLLines reads one team.yaml's lines and says what each stands for. A text
// that does not read answers every line with an empty At, so a screen shows the
// text and marks nothing: the line positions come from the YAML tree, and a text
// with no tree has none (spec-teams B7).
func YAMLLines(text string) []YAMLLine {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		// A file's ending newline is not a line of its own.
		lines = lines[:len(lines)-1]
	}
	at := make([]string, len(lines))
	mark := func(from, to int, value string) {
		for i := from; i <= to; i++ {
			if i >= 1 && i <= len(at) && at[i-1] == "" {
				at[i-1] = value
			}
		}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err == nil && len(doc.Content) > 0 && doc.Content[0].Kind == yaml.MappingNode {
		root := doc.Content[0]
		if value := yamledit.MappingValue(root, "description"); value != nil {
			mark(value.Line, yamledit.NodeEndLine(value), "description")
		}
		for _, item := range sequenceOf(root, "members") {
			name := ""
			if value := yamledit.MappingValue(item, "name"); value != nil {
				name = value.Value
			}
			mark(item.Line, yamledit.NodeEndLine(item), "node:"+name)
		}
		for i, item := range sequenceOf(root, "workers") {
			mark(item.Line, yamledit.NodeEndLine(item), "worker:"+strconv.Itoa(i))
		}
		for _, item := range sequenceOf(root, "turns") {
			turn := strings.ReplaceAll(strings.TrimSpace(item.Value), " -> ", "->")
			mark(item.Line, yamledit.NodeEndLine(item), "turn:"+turn)
		}
	}
	out := make([]YAMLLine, len(lines))
	for i, line := range lines {
		out[i] = YAMLLine{Text: line, At: at[i]}
	}
	return out
}
