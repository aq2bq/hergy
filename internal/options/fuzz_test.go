package options

import "testing"

// FuzzParse feeds argument vectors to Parse. The seeds run with go test; the
// nightly workflow (go run ./tools/check nightly) runs it with -fuzz.
func FuzzParse(f *testing.F) {
	seeds := [][3]string{
		{"up", "dev", "--lang"},
		{"new", "dev", "--local"},
		{"hire", "x", "--purpose=調査"},
		{"status", "--all", ""},
		{"refresh", "dev", "--member"},
		{"ls", "", ""},
		{"show", "--as", "x"},
		{"cheat", "--ai", "--lang=ja"},
		{"--version", "", ""},
		{"", "", ""},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1], s[2])
	}
	f.Fuzz(func(t *testing.T, a, b, c string) {
		opts, err := Parse([]string{a, b, c})
		if (opts == nil) == (err == nil) {
			t.Fatalf("Parse(%q %q %q) = %v, %v: exactly one of the two must be set", a, b, c, opts, err)
		}
	})
}
