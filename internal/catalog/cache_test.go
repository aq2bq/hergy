package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

func cacheWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cacheCatalog(t *testing.T) (*Catalog, *source.Memo) {
	t.Helper()
	root := t.TempDir()
	local := filepath.Join(root, "proj", ".hergy")
	global := filepath.Join(root, "cfg", "hergy")
	cacheWrite(t, filepath.Join(local, "teams", "demo", "team.yaml"),
		"version: 1\ndescription: demo's description\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> human\n")
	cacheWrite(t, filepath.Join(local, "roles", "lead.yaml"),
		"version: 1\nsections:\n  - rules:\n      - id: K7QF\n        text: lead moves first.\n")
	cacheWrite(t, filepath.Join(local, "profiles.yaml"),
		"version: 1\nprofiles:\n  coder:\n    kind: claude\n")
	memo := source.NewMemo()
	return New(global, local, filepath.Join(root, "proj"), memo, func(p string) string { return p }), memo
}

// TestADefinitionIsParsedOnceAndAgainWhenAFileChanges: a second request parses
// nothing and opens no file; a team.yaml written again is read, and only it, on
// the next answer (d-e00b).
func TestADefinitionIsParsedOnceAndAgainWhenAFileChanges(t *testing.T) {
	cat, memo := cacheCatalog(t)
	if _, err := cat.Definition("demo", ""); err != nil {
		t.Fatal(err)
	}
	memo.ResetStats()
	if _, err := cat.Definition("demo", ""); err != nil {
		t.Fatal(err)
	}
	if reads, lists, _ := memo.Stats(); reads != 0 || lists != 0 {
		t.Errorf("the second read opened %d file(s) and listed %d directory(ies), want none", reads, lists)
	}
	path := cat.TeamYAML("demo", source.LayerLocal)
	cacheWrite(t, path, "version: 1\ndescription: a new description\nmembers:\n  - name: lead\n    role: lead\n    profile: coder\nturns:\n  - human -> lead\n  - lead -> human\n")
	memo.ResetStats()
	def, err := cat.Definition("demo", "")
	if err != nil {
		t.Fatal(err)
	}
	if def.Description != "a new description" {
		t.Errorf("description = %q, want the written one", def.Description)
	}
	if reads, _, _ := memo.Stats(); reads != 1 {
		t.Errorf("reads = %d, want only the team.yaml read again", reads)
	}
}

// TestAListOpensNoFileTheSecondTime: the list a screen draws twice reads the
// files once and opens no directory the second time (d-e00b).
func TestAListOpensNoFileTheSecondTime(t *testing.T) {
	cat, memo := cacheCatalog(t)
	if got := cat.Roles(); len(got) == 0 {
		t.Fatal("the roles list is empty")
	}
	memo.ResetStats()
	if got := cat.Roles(); len(got) == 0 {
		t.Fatal("the second roles list is empty")
	}
	if reads, lists, _ := memo.Stats(); reads != 0 || lists != 0 {
		t.Errorf("the second list opened %d file(s) and listed %d directory(ies), want none", reads, lists)
	}
	// A role document added shows on the next list: the directory's version moved.
	cacheWrite(t, filepath.Join(cat.LayerDir(source.LayerLocal), "roles", "builder.yaml"),
		"version: 1\nsections:\n  - rules:\n      - id: MAAA\n        text: builder builds.\n")
	found := false
	for _, row := range cat.Roles() {
		found = found || row.Name == "builder"
	}
	if !found {
		t.Error("the role added after the first list is not in the next one")
	}
}

// TestASavedFileShowsOnTheNextAnswer: a write hergy makes to a file is in the
// next reading, without a held answer standing in its way (d-e00b).
func TestASavedFileShowsOnTheNextAnswer(t *testing.T) {
	cat, _ := cacheCatalog(t)
	rolePath := filepath.Join(cat.LayerDir(source.LayerLocal), "roles", "lead.yaml")
	cacheWrite(t, rolePath, "version: 1\nsections:\n  - rules:\n      - id: K7QF\n        text: written by a save.\n")
	text, err := cat.Reader().Read(source.Doc{Path: rolePath})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(text); !strings.Contains(got, "written by a save") {
		t.Errorf("the saved text is not read: %q", got)
	}
}
