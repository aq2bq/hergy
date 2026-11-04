package web

import "strings"

// PromptDiffLine retains line terminators in Text; joining either side restores
// the original bytes, including a missing final newline.
type PromptDiffLine struct {
	Kind string `json:"kind"`
	Old  int    `json:"old"`
	New  int    `json:"new"`
	Text string `json:"text"`
}

// PromptDiff aligns complete lines with Hirschberg's linear-space LCS. Unlike
// editing-preview marks it also retains every removed and unchanged line.
func PromptDiff(before, after string) []PromptDiffLine {
	a, b := promptLines(before), promptLines(after)
	out := make([]PromptDiffLine, 0, len(a)+len(b))
	alignPrompt(a, b, &out)
	old, next := 0, 0
	for i := range out {
		if out[i].Kind != "add" {
			old++
			out[i].Old = old
		}
		if out[i].Kind != "remove" {
			next++
			out[i].New = next
		}
	}
	return out
}
func promptLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
func alignPrompt(a, b []string, out *[]PromptDiffLine) {
	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		*out = append(*out, PromptDiffLine{Kind: "same", Text: a[0]})
		a = a[1:]
		b = b[1:]
	}
	if len(a) == 0 {
		for _, s := range b {
			*out = append(*out, PromptDiffLine{Kind: "add", Text: s})
		}
		return
	}
	if len(b) == 0 {
		for _, s := range a {
			*out = append(*out, PromptDiffLine{Kind: "remove", Text: s})
		}
		return
	}
	if len(a) == 1 {
		at := -1
		for i, s := range b {
			if s == a[0] {
				at = i
				break
			}
		}
		if at < 0 {
			*out = append(*out, PromptDiffLine{Kind: "remove", Text: a[0]})
		}
		for i, s := range b {
			kind := "add"
			if i == at {
				kind = "same"
			}
			*out = append(*out, PromptDiffLine{Kind: kind, Text: s})
		}
		return
	}
	mid := len(a) / 2
	split := promptSplit(a, b, mid)
	alignPrompt(a[:mid], b[:split], out)
	alignPrompt(a[mid:], b[split:], out)
}
func promptSplit(a, b []string, mid int) int {
	left := promptScores(a[:mid], b, false)
	right := promptScores(a[mid:], b, true)
	best, at := -1, 0
	for i := 0; i <= len(b); i++ {
		if score := left[i] + right[len(b)-i]; score > best {
			best, at = score, i
		}
	}
	return at
}
func promptScores(a, b []string, reverse bool) []int {
	row := make([]int, len(b)+1)
	for i := range a {
		diagonal := 0
		for j := range b {
			ai, bj := i, j
			if reverse {
				ai = len(a) - 1 - i
				bj = len(b) - 1 - j
			}
			previous := row[j+1]
			if a[ai] == b[bj] {
				row[j+1] = diagonal + 1
			} else {
				row[j+1] = max(row[j+1], row[j])
			}
			diagonal = previous
		}
	}
	return row
}
