package ruleid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/catalog"
	"github.com/aq2bq/hergy/internal/rules"
	"github.com/aq2bq/hergy/internal/source"
)

// testCatalog is one project's two layers, made under a temporary root.
func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	root := t.TempDir()
	local := filepath.Join(root, "proj", ".hergy")
	global := filepath.Join(root, "cfg", "hergy")
	for _, dir := range []string{local, global} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	return catalog.New(global, local, filepath.Join(root, "proj"), source.Disk{}, func(path string) string { return path })
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const agreement = `version: 1
title: Demo
sections:
  - heading: Rules
    rules:
      - id: C1
        text: keep C1 in mind, and not C10.
      - text: a rule with no id
`

const roleDoc = `version: 1
title: Lead
sections:
  - heading: Rules
    rules:
      - id: B2
        text: the lead reads C1 too
`

// TestAssignGivesEveryRuleAnIDAndRewritesReferences: a write gives a new-shape
// id to every rule without one, in both layers, and writes old ids out of the
// texts that named them; the file's comments and lines around the ids stay.
func TestAssignGivesEveryRuleAnIDAndRewritesReferences(t *testing.T) {
	cat := testCatalog(t)
	agreePath := filepath.Join(cat.LayerDir(source.LayerLocal), "teams", "demo", "agreement.yaml")
	rolePath := filepath.Join(cat.LayerDir(source.LayerGlobal), "roles", "lead.yaml")
	writeFile(t, agreePath, "# a comment\n"+agreement)
	writeFile(t, rolePath, roleDoc)

	given, err := Assign(cat)
	if err != nil {
		t.Fatal(err)
	}
	if given != 3 {
		t.Fatalf("Assign gave %d ids, want 3", given)
	}
	text := readFile(t, agreePath)
	if !strings.Contains(text, "# a comment\n") {
		t.Errorf("the file's comment went away:\n%s", text)
	}
	doc, problems := rules.ReadAll(text, agreePath)
	if len(problems) != 0 {
		t.Fatalf("the written file reads with problems: %+v\n%s", problems, text)
	}
	for _, rule := range doc.Rules() {
		if !rules.RuleIDPattern.MatchString(rule.ID) {
			t.Errorf("rule %q got no new-shape id", rule.Text)
		}
	}
	// The rule with no id gets its id before its own keys, the way hergy writes
	// a rule, and the text it had stays on the next line (d-4aab).
	noID := doc.Rules()[1].ID
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "- id: "+noID) {
			if i+1 >= len(lines) || !strings.Contains(lines[i+1], "a rule with no id") {
				t.Errorf("the id line stands over %q, want the rule's own text", lines[min(i+1, len(lines)-1)])
			}
			break
		}
		if i == len(lines)-1 {
			t.Errorf("the rule with no id has no line of its own:\n%s", text)
		}
	}
	role, _ := rules.ReadAll(readFile(t, rolePath), rolePath)
	newC1 := doc.Rules()[0].ID
	if newC1 == "C1" {
		t.Fatalf("the old id stayed C1")
	}
	if role.Rules()[0].ID == "B2" {
		t.Errorf("the role's old id stayed")
	}
	if !strings.Contains(role.Rules()[0].Text, newC1) {
		t.Errorf("the role's reference = %q, want the new id %q", role.Rules()[0].Text, newC1)
	}
	if strings.Contains(doc.Rules()[0].Text, " C1 ") {
		t.Errorf("the old reference stayed: %q", doc.Rules()[0].Text)
	}
	if !strings.Contains(doc.Rules()[0].Text, "C10") {
		t.Errorf("C10 was rewritten: %q", doc.Rules()[0].Text)
	}
}

// TestAssignKeepsCurrentIDsAndIsIdempotent: a second write finds nothing to
// give, and leaves the files byte for byte.
func TestAssignKeepsCurrentIDsAndIsIdempotent(t *testing.T) {
	cat := testCatalog(t)
	path := filepath.Join(cat.LayerDir(source.LayerLocal), "roles", "lead.yaml")
	writeFile(t, path, roleDoc)
	if _, err := Assign(cat); err != nil {
		t.Fatal(err)
	}
	first := readFile(t, path)
	given, err := Assign(cat)
	if err != nil {
		t.Fatal(err)
	}
	if given != 0 {
		t.Errorf("the second assign gave %d ids, want 0", given)
	}
	if readFile(t, path) != first {
		t.Errorf("the second assign wrote the file again")
	}
}

// TestMissingCountsWithoutWriting: a read-only count names the rules the next
// write will give an id, and writes nothing.
func TestMissingCountsWithoutWriting(t *testing.T) {
	cat := testCatalog(t)
	path := filepath.Join(cat.LayerDir(source.LayerLocal), "roles", "lead.yaml")
	writeFile(t, path, roleDoc+"      - text: a rule with no id\n")
	before := readFile(t, path)
	if got := Missing(cat); got != 1 {
		t.Errorf("Missing = %d, want 1 (the old id is an id, and is not counted)", got)
	}
	if readFile(t, path) != before {
		t.Errorf("Missing wrote the file")
	}
}

// TestTakenKeepsAnIDOutOfTheNewOnes: an id the launch record holds is not given
// again, so a rule taken out keeps its name out of use.
func TestTakenKeepsAnIDOutOfTheNewOnes(t *testing.T) {
	cat := testCatalog(t)
	// A launch record with one prompt holding K7QF.
	logDir := filepath.Join(os.Getenv("XDG_STATE_HOME"), "hergy", "launches")
	blobs := filepath.Join(logDir, "blobs")
	if err := os.MkdirAll(blobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blobs, "one"), []byte("**K7QF** a rule"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !Taken(cat)["K7QF"] {
		t.Error("Taken does not hold the id the launch record holds")
	}
}
