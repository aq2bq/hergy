package i18n

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// displayWidth counts a full-width character as two columns, the way a
// terminal draws it.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case r >= 0x1100 && r <= 0x115f, // Hangul Jamo
			r >= 0x2e80 && r <= 0xa4cf, // CJK radicals, punctuation, kana, ideographs
			r >= 0xac00 && r <= 0xd7a3, // Hangul syllables
			r >= 0xf900 && r <= 0xfaff, // CJK compatibility ideographs
			r >= 0xfe30 && r <= 0xfe4f, // CJK compatibility forms
			r >= 0xff00 && r <= 0xff60, // full-width forms
			r >= 0xffe0 && r <= 0xffe6:
			n += 2
		default:
			n++
		}
	}
	return n
}

// secondColumn is where the second column of a table row starts, in display
// columns, or -1 for a line that is not a table row. A row is indented two
// spaces and has a gap of two or more spaces after its first cell.
func secondColumn(line string) int {
	if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
		return -1
	}
	body := line[2:]
	i := strings.Index(body, "  ")
	if i < 0 {
		return -1
	}
	rest := strings.TrimLeft(body[i:], " ")
	if rest == "" {
		return -1
	}
	return displayWidth(line[:len(line)-len(rest)])
}

// TestHelpAndCheatColumnsLineUp: the second column of --help lines up across
// the whole text, and in the cheatsheet within each section, counting a
// full-width character as two columns.
func TestHelpAndCheatColumnsLineUp(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		raw, err := os.ReadFile("locales/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			t.Fatal(err)
		}
		checkColumns(t, lang, "usage", texts["usage"], false)
		checkColumns(t, lang, "cheat.people", texts["cheat.people"], true)
	}
}

func checkColumns(t *testing.T, lang, key, text string, perSection bool) {
	t.Helper()
	want := -1
	for _, line := range strings.Split(text, "\n") {
		if line == "" && perSection {
			want = -1
			continue
		}
		col := secondColumn(line)
		if col < 0 {
			continue
		}
		if want < 0 {
			want = col
			continue
		}
		if col != want {
			t.Errorf("%s %s: the second column starts at %d, want %d:\n%s", lang, key, col, want, line)
		}
	}
}
