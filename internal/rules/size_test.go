package rules

import (
	"strings"
	"testing"
)

// TestCharCountCountsCodePointsAndBringsCRLFToLF: a file written on Windows
// counts the same as one written here, and a character outside the BMP is one
// character.
func TestCharCountCountsCodePointsAndBringsCRLFToLF(t *testing.T) {
	cases := []struct {
		text string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"a\r\nb", 3},
		{"a\nb", 3},
		{"あいう", 3},
		{"🙂", 1},
		{strings.Repeat("x", 100), 100},
	}
	for _, tc := range cases {
		if got := CharCount(tc.text); got != tc.want {
			t.Errorf("CharCount(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}
