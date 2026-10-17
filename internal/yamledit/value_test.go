package yamledit_test

import (
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/yamledit"
	"go.yaml.in/yaml/v3"
)

// TestAValueThatReadsAsSomethingElseIsQuoted is spec/41 S1-2: a string that
// YAML would read as a number, a boolean or a null is written quoted, so it
// comes back as the text it is — in a sequence and in a mapping, and wherever a
// key holds it.
func TestAValueThatReadsAsSomethingElseIsQuoted(t *testing.T) {
	src := yamledit.New([]byte("version: 1\nprofiles:\n  a:\n    kind: claude\n"), "p")
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("version: 1\nprofiles:\n  a:\n    kind: claude\n"), &doc); err != nil {
		t.Fatal(err)
	}
	item := yamledit.MappingValue(doc.Content[0], "profiles").Content[1]
	if err := yamledit.ReplaceSeq(src, item, "args", []string{"007", "1.5", "--x"}); err != nil {
		t.Fatal(err)
	}
	if err := yamledit.ReplaceMap(src, item, "env", []yamledit.Pair{{Key: "A", Value: "1"}, {Key: "B", Value: "true"}}); err != nil {
		t.Fatal(err)
	}
	text := string(src.Bytes())
	for _, want := range []string{`- "007"`, `- "1.5"`, `- --x`, `A: "1"`, `B: "true"`} {
		if !strings.Contains(text, want) {
			t.Errorf("the written lines do not hold %q:\n%s", want, text)
		}
	}
	// What it wrote reads back as the strings it was given.
	var out struct {
		Profiles map[string]struct {
			Args []string          `yaml:"args"`
			Env  map[string]string `yaml:"env"`
		} `yaml:"profiles"`
	}
	if err := yaml.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("what it wrote does not read: %v\n%s", err, text)
	}
	a := out.Profiles["a"]
	if len(a.Args) != 3 || a.Args[0] != "007" || a.Args[1] != "1.5" || a.Args[2] != "--x" {
		t.Errorf("args read back as %q", a.Args)
	}
	if a.Env["A"] != "1" || a.Env["B"] != "true" {
		t.Errorf("env read back as %q", a.Env)
	}
}

// TestAValueThatReadsAsAStringStaysPlain: a value YAML reads as the same string
// is not quoted, so a file hergy writes stays readable.
func TestAValueThatReadsAsAStringStaysPlain(t *testing.T) {
	cases := map[string]string{
		"claude":   "claude",
		"--model":  "--model",
		"1 行で書く":   "1 行で書く",
		"リリース 1.5": "リリース 1.5",
	}
	for value, want := range cases {
		if got := yamledit.ScalarLiteral(value, 0); got != want {
			t.Errorf("ScalarLiteral(%q) = %q, want %q", value, got, want)
		}
	}
	for _, value := range []string{"1", "1.5", "007", "true", "null", "~"} {
		if got := yamledit.ScalarLiteral(value, 0); !strings.HasPrefix(got, `"`) {
			t.Errorf("ScalarLiteral(%q) = %q, want it quoted", value, got)
		}
	}
}
