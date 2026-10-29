package source

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMemoFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestMemoReadsAFileOnceAndAgainWhenItChanges: the second read of a file that
// did not change opens nothing, and a file written again is read on the next
// answer (d-e00b).
func TestMemoReadsAFileOnceAndAgainWhenItChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.yaml")
	writeMemoFile(t, path, "one")
	m := NewMemo()
	doc := Doc{Path: path}
	if got, err := m.Read(doc); err != nil || got != "one" {
		t.Fatalf("Read = %q, %v", got, err)
	}
	if _, err := m.Read(doc); err != nil {
		t.Fatal(err)
	}
	if reads, _, hits := m.Stats(); reads != 1 || hits != 1 {
		t.Errorf("reads=%d hits=%d, want one read and one held answer", reads, hits)
	}
	// A write with a different size moves the version.
	writeMemoFile(t, path, "one and two")
	if got, err := m.Read(doc); err != nil || got != "one and two" {
		t.Fatalf("Read after a write = %q, %v", got, err)
	}
	if reads, _, _ := m.Stats(); reads != 2 {
		t.Errorf("reads = %d, want the changed file read again", reads)
	}
}

// TestMemoListsADirectoryOnceAndAgainWhenItChanges: a listing that did not
// change opens no directory, and a file added is in the next listing.
func TestMemoListsADirectoryOnceAndAgainWhenItChanges(t *testing.T) {
	dir := t.TempDir()
	writeMemoFile(t, filepath.Join(dir, "a.yaml"), "one")
	writeMemoFile(t, filepath.Join(dir, "b.md"), "two")
	m := NewMemo()
	doc := Doc{Path: dir}
	if docs, err := m.YAMLIn(doc); err != nil || len(docs) != 1 {
		t.Fatalf("YAMLIn = %v, %v", docs, err)
	}
	if _, err := m.YAMLIn(doc); err != nil {
		t.Fatal(err)
	}
	if _, lists, _ := m.Stats(); lists != 1 {
		t.Errorf("lists = %d, want the directory opened once", lists)
	}
	writeMemoFile(t, filepath.Join(dir, "c.yaml"), "three")
	docs, err := m.YAMLIn(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || filepath.Base(docs[1].Path) != "c.yaml" {
		t.Errorf("YAMLIn after a file was added = %v, want c.yaml there", docs)
	}
	if _, lists, _ := m.Stats(); lists != 2 {
		t.Errorf("lists = %d, want the changed directory listed again", lists)
	}
}

// TestMemoSaysAFileIsNotThere: a file that is not there reads as not found, and
// one made after it is read on the next answer.
func TestMemoSaysAFileIsNotThere(t *testing.T) {
	dir := t.TempDir()
	m := NewMemo()
	doc := Doc{Path: filepath.Join(dir, "later.yaml")}
	if _, err := m.Read(doc); err == nil {
		t.Fatal("a file that is not there read")
	}
	writeMemoFile(t, doc.Path, "now")
	if got, err := m.Read(doc); err != nil || got != "now" {
		t.Fatalf("Read after the file was made = %q, %v", got, err)
	}
}
