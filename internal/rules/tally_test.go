package rules

import "testing"

// TestTallyCountsFilesAndRules: a file is counted once, and the rules in it are
// summed; the per-file aggregate lines the old model needed are gone (r-81be).
func TestTallyCountsFilesAndRules(t *testing.T) {
	one := parseAt(t, "one.yaml", "version: 1\nsections:\n  - rules:\n      - id: A1\n        text: x\n      - id: A2\n        text: y\n")
	two := parseAt(t, "two.yaml", "version: 1\nsections:\n  - rules:\n      - id: B1\n        text: z\n")
	tally := &Tally{}
	tally.Add(one)
	tally.Add(two)
	if tally.Files != 2 || tally.Total != 3 {
		t.Errorf("tally = %+v, want 2 files and 3 rules", tally)
	}
}
