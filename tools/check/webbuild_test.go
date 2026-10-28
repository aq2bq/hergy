package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These are the webbuild gate's two promises, held on trees of their own: the
// gate writes and removes the .js the build makes, and nothing else under
// static/ (r-893d).

// tree writes the files of one tree, by their path from its root.
func tree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestWebBuildWritesAndRemovesOnlyWhatItMakes: -write puts the built files in
// place, takes away a .js the build no longer makes, and leaves the styles, the
// fonts and every other file exactly as they were.
func TestWebBuildWritesAndRemovesOnlyWhatItMakes(t *testing.T) {
	root := t.TempDir()
	static := filepath.Join(root, webStatic)
	tree(t, static, map[string]string{
		"roles/main.js":    "old",
		"lib/gone.js":      "a script nothing makes any more",
		"roles-v2.css":     "body { color: black }",
		"fonts/body.woff2": "font bytes",
		"notes.txt":        "a person's note",
	})
	built := map[string][]byte{
		"roles/main.js": []byte("new"),
		"_chunk/a-1.js": []byte("chunk"),
	}
	if err := writeWebTree(root, built); err != nil {
		t.Fatal(err)
	}

	if got, err := os.ReadFile(filepath.Join(static, "roles", "main.js")); err != nil || string(got) != "new" {
		t.Errorf("roles/main.js = %q, %v; want the built file", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(static, "_chunk", "a-1.js")); err != nil || string(got) != "chunk" {
		t.Errorf("_chunk/a-1.js = %q, %v; want the built chunk", got, err)
	}
	if _, err := os.Stat(filepath.Join(static, "lib", "gone.js")); !os.IsNotExist(err) {
		t.Errorf("a .js the build does not make is still there: %v", err)
	}
	// The build's own directory can be left empty when a chunk is renamed; the
	// empty directory goes with it.
	if _, err := os.Stat(filepath.Join(static, "lib")); !os.IsNotExist(err) {
		t.Errorf("the emptied directory lib/ is still there: %v", err)
	}
	for _, keep := range []string{"roles-v2.css", "fonts/body.woff2", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(static, filepath.FromSlash(keep))); err != nil {
			t.Errorf("%s was taken away: %v", keep, err)
		}
	}
}

// TestWebBuildNamesWhatDiffers: without -write nothing is touched, and each
// difference is named — a file static/ does not hold, one it holds differently,
// and a .js the build no longer makes — with the command that writes them again.
func TestWebBuildNamesWhatDiffers(t *testing.T) {
	root := t.TempDir()
	static := filepath.Join(root, webStatic)
	tree(t, static, map[string]string{
		"roles/main.js": "old",
		"lib/gone.js":   "nothing makes this",
		"roles-v2.css":  "body { color: black }",
	})
	built := map[string][]byte{
		"roles/main.js":    []byte("new"),
		"activity/main.js": []byte("a file static/ does not hold"),
	}
	err := compareWebTree(root, built)
	if err == nil {
		t.Fatal("a tree that is not what the build makes passed")
	}
	text := err.Error()
	for _, want := range []string{
		"roles/main.js: static/ is not what internal/web/src makes of it",
		"activity/main.js: the build makes it and static/ does not hold it",
		"lib/gone.js: static/ holds it and the build does not make it",
		"go run ./tools/check webbuild -write",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the finding does not say %q:\n%s", want, text)
		}
	}
	// Nothing was written: the gate reports, and -write is what writes.
	if got, _ := os.ReadFile(filepath.Join(static, "roles", "main.js")); string(got) != "old" {
		t.Errorf("the check wrote the file: %q", got)
	}
}

// TestWebBuildPassesOnTheTreeItMakes: the same tree the build makes passes,
// styles and fonts beside it and all.
func TestWebBuildPassesOnTheTreeItMakes(t *testing.T) {
	root := t.TempDir()
	static := filepath.Join(root, webStatic)
	built := map[string][]byte{
		"roles/main.js": []byte("new"),
		"_chunk/a-1.js": []byte("chunk"),
	}
	files := map[string]string{"roles-v2.css": "body { color: black }", "fonts/body.woff2": "font bytes"}
	for rel, text := range built {
		files[rel] = string(text)
	}
	tree(t, static, files)
	if err := compareWebTree(root, built); err != nil {
		t.Fatalf("the tree the build makes does not pass: %v", err)
	}
}
