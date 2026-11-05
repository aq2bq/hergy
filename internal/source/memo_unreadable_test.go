package source

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAMemoStopsAnsweringAFileThatStopsOpening: a permission change moves
// neither size nor time, so the held text must not stand for a file that no
// longer opens; it comes back when the file opens again (d-e00b).
func TestAMemoStopsAnsweringAFileThatStopsOpening(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewMemo()
	doc := Doc{Path: path}
	if text, err := m.Read(doc); err != nil || text != "version: 1\n" {
		t.Fatalf("the first read = %q, %v", text, err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if text, err := m.Read(doc); err == nil {
		t.Errorf("the read of a file that cannot be opened = %q, want an error", text)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if text, err := m.Read(doc); err != nil || text != "version: 1\n" {
		t.Errorf("the read after the permission came back = %q, %v", text, err)
	}
}
