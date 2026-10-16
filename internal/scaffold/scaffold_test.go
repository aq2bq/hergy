package scaffold

import (
	"strings"
	"testing"
)

// TestMissionPlaceholderIsWhatTheStarterWrites: the placeholder hergy validate
// warns about (d-3cae) is the mission line the starter team actually writes, so
// the warning cannot drift from the file.
func TestMissionPlaceholderIsWhatTheStarterWrites(t *testing.T) {
	for lang, example := range MissionPlaceholder {
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
				if line == "mission: "+example {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("the starter team of %s does not write the mission placeholder %q", lang, example)
		}
		if !IsPlaceholderMission(example) {
			t.Errorf("IsPlaceholderMission(%q) = false", example)
		}
	}
	if IsPlaceholderMission("a mission of one's own") {
		t.Error("IsPlaceholderMission accepted a mission that is not an example")
	}
}
