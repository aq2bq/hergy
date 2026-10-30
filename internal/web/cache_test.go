package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAListShowsAFileWrittenAfterIt: a list held under the files' version is
// built again when a definition is written, and the next answer reads the new
// file — whether hergy wrote it or a person did (d-e00b).
func TestAListShowsAFileWrittenAfterIt(t *testing.T) {
	f := newFixture(t)
	server := f.server(t)
	_, first := f.api(t, server, http.MethodGet, "/api/team", nil)
	if !strings.Contains(first, "demo の説明") {
		t.Fatalf("the list does not hold the definition's description:\n%s", first)
	}
	// The same list again is the held answer.
	if _, second := f.api(t, server, http.MethodGet, "/api/team", nil); second != first {
		t.Errorf("the second list differs from the first")
	}
	path := filepath.Join(f.proj, ".hergy", "teams", "demo", "team.yaml")
	text := readFile(t, path)
	if err := os.WriteFile(path, []byte(strings.Replace(text, "description: demo の説明", "description: 書き換えた説明", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	_, after := f.api(t, server, http.MethodGet, "/api/team", nil)
	if !strings.Contains(after, "書き換えた説明") {
		t.Errorf("the list after the file changed does not hold the new description:\n%s", after)
	}
}
