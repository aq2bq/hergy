package rules

import (
	"strings"
	"unicode/utf8"
)

// CharCount is the characters of a text in Unicode code points, with CRLF
// brought to LF first so a file written on Windows counts the same as one
// written here. Whitespace, line breaks, headings and URLs are counted.
//
// It is the one way hergy counts a guide's size (check.RoleCharLimit and
// check.TeamCharLimit, d-00a9): a team definition counts the part of its rules
// that goes into the prompt and the words of its templates, and a role document
// counts the part of it that goes into the prompt. There is no per-member amount
// (d-a575, d-b87a).
func CharCount(text string) int {
	return utf8.RuneCountInString(strings.ReplaceAll(text, "\r\n", "\n"))
}
