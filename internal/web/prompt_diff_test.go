package web

import (
	"strings"
	"testing"
)

func TestPromptDiffRetainsBothBodies(t *testing.T) {
	for _, pair := range [][2]string{{"", ""}, {"a\n", "a"}, {"\r\n \t\n", "\n \t\n"}, {"a\nb\nc\n", "b\nd\nc\n"}, {"x", "a\nx\nb"}, {"x", "y"}, {"same\nsame\n", "same\n"}} {
		lines := PromptDiff(pair[0], pair[1])
		var old, next strings.Builder
		a, b := 0, 0
		for _, line := range lines {
			if line.Kind != "add" {
				a++
				old.WriteString(line.Text)
				if line.Old != a {
					t.Fatal(line)
				}
			}
			if line.Kind != "remove" {
				b++
				next.WriteString(line.Text)
				if line.New != b {
					t.Fatal(line)
				}
			}
		}
		if old.String() != pair[0] || next.String() != pair[1] {
			t.Fatalf("cannot reconstruct %q: %+v", pair, lines)
		}
	}
}
func TestPromptDiffTurnChange(t *testing.T) {
	lines := PromptDiff("名簿\n## ターン\nlead -> builder\n", "名簿\n## ターン\nlead -> human\n")
	if len(lines) != 4 || lines[2].Kind != "remove" || lines[3].Kind != "add" {
		t.Fatal(lines)
	}
}
