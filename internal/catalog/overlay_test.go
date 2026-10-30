package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// TestWithReadsATeamYAMLThatIsNotWritten: a screen reads a change it has not
// saved (C3b). The file on disk is untouched.
func TestWithReadsATeamYAMLThatIsNotWritten(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "before")

	path := filepath.Join(l.local, "teams", "t", "team.yaml")
	after := strings.Replace(readFile(t, path), "description: before", "description: after", 1)
	overlaid := l.catalog().With(path, after)

	def, err := overlaid.Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if def.Description != "after" {
		t.Errorf("description = %q, want the edit's", def.Description)
	}
	onDisk, err := l.catalog().Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Description != "before" {
		t.Errorf("description = %q, want the file's", onDisk.Description)
	}
}

// TestWithReadsAProfilesYAMLThatIsNotThere reads a profiles.yaml that is not on
// disk at all: the first save of a layer writes one (C3b, r-daf8).
func TestWithReadsAProfilesYAMLThatIsNotThere(t *testing.T) {
	l := newLayers(t)
	path := filepath.Join(l.global, "profiles.yaml")
	text := "version: 1\nprofiles:\n  made:\n    kind: claude\n"

	profiles, paths, err := l.catalog().With(path, text).Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := profiles["made"]; !ok {
		t.Fatalf("profiles = %v, want the edit's", profiles)
	}
	if paths != path {
		t.Errorf("paths = %q, want %q", paths, path)
	}
	if _, _, err := l.catalog().Profiles(); err != nil {
		t.Errorf("reading the layers without the edit: %v", err)
	}
}

// TestWithLeavesOtherPathsToDisk: a text stands in for one path. Everything else
// reads from disk (C3b).
func TestWithLeavesOtherPathsToDisk(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "before")
	l.writeFullTeam(t, source.LayerGlobal, "other", "the other description")

	path := filepath.Join(l.local, "teams", "t", "team.yaml")
	after := strings.Replace(readFile(t, path), "description: before", "description: after", 1)
	overlaid := l.catalog().With(path, after)

	def, err := overlaid.Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if def.Description != "after" {
		t.Errorf("t = %q, want the edit's", def.Description)
	}
	other, err := overlaid.Read("other")
	if err != nil {
		t.Fatal(err)
	}
	if other.Description != "the other description" {
		t.Errorf("other = %q, want the file's", other.Description)
	}
}

// TestWithFindsARoleDocumentThatIsNotWritten: a role document the screen is
// about to make is found, and its text reads back (C3b).
func TestWithFindsARoleDocumentThatIsNotWritten(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	path := filepath.Join(l.local, "roles", "new-role.yaml")
	text := "a role document the screen made\n"

	cat := l.catalog()
	if _, ok := cat.Role("new-role"); ok {
		t.Fatal("the role document is there before the edit")
	}
	overlaid := cat.With(path, text)
	doc, ok := overlaid.Role("new-role")
	if !ok {
		t.Fatal("the edit's role document is not found")
	}
	if doc.Path != path || doc.Layer != source.LayerLocal {
		t.Errorf("doc = %+v, want %q in the project's layer", doc, path)
	}
	read, err := overlaid.Reader().Read(doc)
	if err != nil {
		t.Fatal(err)
	}
	if read != text {
		t.Errorf("read %q, want the edit's text", read)
	}
}

// TestWithFindsAnAgreementThatIsNotWritten: a team's own agreement the screen is
// about to make is read as the team's (C3b). Whether a definition has one is
// asked of the reader, so it is answered by the edit.
func TestWithFindsAnAgreementThatIsNotWritten(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	path := filepath.Join(l.local, "teams", "t", "agreement.yaml")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	cat := l.catalog()
	before, err := cat.Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if before.HasAgreement {
		t.Fatal("the team has an agreement before the edit")
	}
	after, err := cat.With(path, "the agreement the screen wrote\n").Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if !after.HasAgreement || after.Agreement.Path != path {
		t.Errorf("agreement = %+v (has %v), want the edit's", after.Agreement, after.HasAgreement)
	}
}

// TestWithFindsAFileThatIsNotWritten: a document a screen is making, laid over
// the disk by the same edit (C3b).
func TestWithFindsAFileThatIsNotWritten(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	path := filepath.Join(l.local, "roles", "r.yaml")

	cat := l.catalog()
	if _, ok := cat.Role("r"); ok {
		t.Fatal("the role document is there before the edit")
	}
	overlaid := cat.With(path, "version: 1\ntitle: role\n")
	doc, ok := overlaid.Role("r")
	if !ok {
		t.Fatal("the edit's role document is not found")
	}
	if doc.Path != path {
		t.Errorf("doc = %+v, want %q", doc, path)
	}
}

// TestDefinitionEditAndProfilesEditAreWith: the two older mouths are thin covers
// over With (C3b), so a caller that still uses them reads the same text.
