package definition

import "testing"

func TestYAMLLinesSaysWhatEachLineStandsFor(t *testing.T) {
	text := `version: 1
# mission は必須。
mission: このチームの使命を 1 行で
members:
  - name: lead
    role: lead
    profile: claude-default
  - name: builder
    role: builder
    profile: claude-default
workers:
  - from: lead
    purpose: 調査
    profile: claude-default
turns:
  - human -> lead
  - lead -> builder
`
	got := YAMLLines(text)
	want := []YAMLLine{
		{Text: "version: 1", At: ""},
		{Text: "# mission は必須。", At: ""},
		{Text: "mission: このチームの使命を 1 行で", At: "mission"},
		{Text: "members:", At: ""},
		{Text: "  - name: lead", At: "node:lead"},
		{Text: "    role: lead", At: "node:lead"},
		{Text: "    profile: claude-default", At: "node:lead"},
		{Text: "  - name: builder", At: "node:builder"},
		{Text: "    role: builder", At: "node:builder"},
		{Text: "    profile: claude-default", At: "node:builder"},
		{Text: "workers:", At: ""},
		{Text: "  - from: lead", At: "worker:0"},
		{Text: "    purpose: 調査", At: "worker:0"},
		{Text: "    profile: claude-default", At: "worker:0"},
		{Text: "turns:", At: ""},
		{Text: "  - human -> lead", At: "turn:human->lead"},
		{Text: "  - lead -> builder", At: "turn:lead->builder"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(got), len(want), got)
	}
	for i, line := range got {
		if line != want[i] {
			t.Errorf("line %d: got %+v, want %+v", i+1, line, want[i])
		}
	}
}

func TestYAMLLinesOfTextThatDoesNotRead(t *testing.T) {
	got := YAMLLines("mission: [oops\n")
	if len(got) != 1 || got[0].Text != "mission: [oops" || got[0].At != "" {
		t.Fatalf("got %+v, want the line with no at", got)
	}
	if lines := YAMLLines(""); len(lines) != 0 {
		t.Errorf("empty text gave %d lines, want 0", len(lines))
	}
}
