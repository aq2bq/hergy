package ops

import (
	"os"
	"path/filepath"
	"testing"

	domainplace "github.com/aq2bq/hergy/internal/domain/place"
	"github.com/aq2bq/hergy/internal/source"
)

// TestAddedProjectIsOneServeReads is ac-db73's last line: once a project is
// listed, it is among the projects the activity and the headquarters read, and
// a start in it has nothing to say about the list.
func TestAddedProjectIsOneServeReads(t *testing.T) {
	f := newFixture(t)
	initLocales(t)
	other := filepath.Join(filepath.Dir(f.proj), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.cfg, "hergy", "config.yaml"), "version: 1\nprojects:\n  - "+other+"\n")
	env := serveEnv(t)
	reads := func() bool {
		for _, view := range projectViews(env, source.Disk{}, nil) {
			if view.Path.Compare(env.ProjectRoot) == domainplace.Same {
				return true
			}
		}
		return false
	}
	if reads() || unlistedProjectNote(env) == "" {
		t.Fatalf("before the add: serve reads the project %t, note %q", reads(), unlistedProjectNote(env))
	}
	added, err := AddProject(env, ".")
	if err != nil {
		t.Fatal(err)
	}
	if added.Entry != f.proj {
		t.Errorf("the entry reads %q, want %q", added.Entry, f.proj)
	}
	if !reads() {
		t.Error("serve does not read the project that was just listed")
	}
	if note := unlistedProjectNote(env); note != "" {
		t.Errorf("a listed project still gets the note: %s", note)
	}
}
