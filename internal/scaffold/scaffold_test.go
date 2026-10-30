package scaffold

import (
	"strings"
	"testing"
)

// TestDescriptionPlaceholderIsWhatTheStarterWrites: the placeholder hergy validate
// warns about (d-3cae) is the description line the starter team actually writes, so
// the warning cannot drift from the file.
func TestDescriptionPlaceholderIsWhatTheStarterWrites(t *testing.T) {
	for lang, example := range DescriptionPlaceholder {
		files, err := Team(lang)
		if err != nil {
			t.Fatalf("Team(%q): %v", lang, err)
		}
		found := false
		for _, file := range files {
			if file.Path != "team.yaml" {
				continue
			}
			for line := range strings.SplitSeq(string(file.Data), "\n") {
				if line == "description: "+example {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("the starter team of %s does not write the description placeholder %q", lang, example)
		}
		if !IsPlaceholderDescription(example) {
			t.Errorf("IsPlaceholderDescription(%q) = false", example)
		}
	}
	if IsPlaceholderDescription("a description of one's own") {
		t.Error("IsPlaceholderDescription accepted a description that is not an example")
	}
}
