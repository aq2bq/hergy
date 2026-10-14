package i18n

// Width is how many terminal columns s takes: a full-width character, such as
// kana, an ideograph or a full-width form, counts as two. --help uses it to line
// the second column up. layout_test.go keeps its own copy of the rule, so a test
// can check this one against it.
func Width(s string) int {
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
