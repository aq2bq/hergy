package rules

import "testing"

// TestLinesOfPointsAtTheLinesAProblemStandsOn: a problem's field is looked up
// against the file, so the YAML tab can mark the line a problem stands on and
// put the cursor on it (spec-roles 6 C6). A place the file does not hold answers
// 0, which the caller reads as the file as a whole.
func TestLinesOfPointsAtTheLinesAProblemStandsOn(t *testing.T) {
	const text = `version: 1
title: 進行管理
revision: 3
sections:
  - heading: 使命
    style: ids
    rules:
      - id: LEAD-1
        text: 人間と話す。
        origin:
          - kind: derived
            refs: [D-1, D-2]
      - id: LEAD-2
        text: 払い出す。
`
	lines := LinesOf(text, "roles/lead.yaml")
	want := []struct {
		where string
		line  int
	}{
		{"the title", lines.At(-1, -1, "title")},
		{"the revision", lines.At(-1, -1, "revision")},
		{"the section's heading", lines.At(0, -1, "sections.0.heading")},
		{"the section's style", lines.At(0, -1, "style")},
		{"the rule's id", lines.At(0, 0, "id")},
		{"the rule's text", lines.At(0, 0, "text")},
		{"the origin's refs", lines.At(0, 0, "origin.0.refs.0")},
		{"the second rule's text", lines.At(0, 1, "text")},
	}
	got := []int{2, 3, 5, 6, 8, 9, 12, 14}
	for i, w := range want {
		if w.line != got[i] {
			t.Errorf("%s = line %d, want %d", w.where, w.line, got[i])
		}
	}
	if n := lines.At(-1, -1, "nope"); n != 0 {
		t.Errorf("a field the file does not hold = %d, want 0", n)
	}
}

// TestLinesOfReadsNoLinesFromAFileThatDoesNotRead: a file that does not read has
// no line to point at, so every field answers 0 and the problem keeps the line it
// was read with.
func TestLinesOfReadsNoLinesFromAFileThatDoesNotRead(t *testing.T) {
	lines := LinesOf("sections: [\n", "roles/lead.yaml")
	if n := lines.At(0, 0, "text"); n != 0 {
		t.Errorf("At = %d, want 0 from a file that does not read", n)
	}
}
