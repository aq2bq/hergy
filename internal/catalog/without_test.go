package catalog

import (
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// TestWithoutReadsAPathAsThoughItIsNotThere: a screen checks a draft that
// removes a document, and the reading has to see the removal before it is
// written (C5). The catalog it was called on is left as it was.
func TestWithoutReadsAPathAsThoughItIsNotThere(t *testing.T) {
	l := newLayers(t)
	path := filepath.Join(l.local, "roles", "gone.yaml")
	write(t, path, "a role document the screen is removing\n")

	cat := l.catalog()
	doc, ok := cat.Role("gone")
	if !ok {
		t.Fatal("the role document is not there before the removal")
	}
	if _, err := cat.Reader().Read(doc); err != nil {
		t.Fatalf("reading it before the removal: %v", err)
	}

	removed := cat.Without(path)
	if _, ok := removed.Role("gone"); ok {
		t.Error("the removed role document is still found")
	}
	if removed.Reader().IsFile(doc) {
		t.Error("the removed path still reads as a file")
	}
	if _, err := removed.Reader().Read(doc); err == nil {
		t.Error("the removed path still reads")
	}
	if _, ok := cat.Role("gone"); !ok {
		t.Error("Without changed the catalog it was called on")
	}
}

// TestWithoutDropsATextLaidOverTheSamePath: a file cannot be both there and not
// there, so a removal wins the text a screen was checking (C5).
func TestWithoutDropsATextLaidOverTheSamePath(t *testing.T) {
	l := newLayers(t)
	path := filepath.Join(l.local, "roles", "new.yaml")

	cat := l.catalog().With(path, "a role document the screen made\n")
	if _, ok := cat.Role("new"); !ok {
		t.Fatal("the edit's role document is not found")
	}
	removed := cat.Without(path)
	if _, ok := removed.Role("new"); ok {
		t.Error("the path both laid over and removed is still found")
	}
	if _, ok := cat.Role("new"); !ok {
		t.Error("Without changed the catalog it was called on")
	}
}

// TestWithoutLeavesOtherPathsToDisk: a removal stands for one path; every other
// path reads as before (C5).
func TestWithoutLeavesOtherPathsToDisk(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "t", "m")
	removed := l.catalog().Without(filepath.Join(l.local, "roles", "lead-role.yaml"))

	if _, ok := removed.Role("lead-role"); ok {
		t.Error("the removed role document is still found")
	}
	if _, ok := removed.Role("impl-role"); !ok {
		t.Error("a removal hid another role document")
	}
}
