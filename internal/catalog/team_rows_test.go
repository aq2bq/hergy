package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// A folder left after deleting a definition is still a row when it holds a
// file. A newly made catalog must agree with one that saw the definition.
func TestTeamRowsKeepAFileLeftWithoutTeamYAML(t *testing.T) {
	l := newLayers(t)
	l.writeTeam(t, source.LayerLocal, "demo")
	folder := filepath.Join(l.local, "teams", "demo")
	write(t, filepath.Join(folder, "notes.txt"), "keep me\n")
	held := l.catalog()
	if rows := held.ShownRows(KindTeam, source.LayerLocal); len(rows) != 1 {
		t.Fatalf("initial rows = %v", rows)
	}
	if err := os.Remove(filepath.Join(folder, "team.yaml")); err != nil {
		t.Fatal(err)
	}
	for _, cat := range []*Catalog{l.catalog(), held} {
		if definitions := cat.Definitions(); len(definitions) != 0 {
			t.Errorf("the removed team.yaml is still a definition: %v", definitions)
		}
		rows := cat.ShownRows(KindTeam, source.LayerLocal)
		if len(rows) != 1 || rows[0].Name != "demo" {
			t.Fatalf("remaining file's rows = %v, want demo", rows)
		}
		if ok, why := cat.ReadableOf(rows[0]); ok || !strings.Contains(why, "team.yaml") {
			t.Errorf("readable = %v, %q, want missing team.yaml", ok, why)
		}
		if why, state := cat.Unreadable(rows[0].Path); !strings.Contains(why, "team.yaml") || state != StateUnreadable {
			t.Errorf("file state = %q, %q, want unreadable with reason", why, state)
		}
	}
}

// Only child directories change in this sequence. Restoring the parent's
// timestamps makes it explicit that a cache cannot use that parent as its key.
func TestTeamRowsNoticeChildFilesWithAnUnchangedParent(t *testing.T) {
	l := newLayers(t)
	parent := filepath.Join(l.local, "teams")
	folder := filepath.Join(parent, "demo")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	stamp, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	held := l.catalog()
	cases := []struct {
		file        string
		remove      bool
		want        []string
		definitions []string
	}{
		{want: []string{}, definitions: []string{}},
		{file: "notes.txt", want: []string{"demo"}, definitions: []string{}},
		{file: "notes.txt", remove: true, want: []string{}, definitions: []string{}},
		{file: "team.yaml", want: []string{"demo"}, definitions: []string{"demo"}},
		{file: "team.yaml", remove: true, want: []string{}, definitions: []string{}},
	}
	for _, step := range cases {
		if step.file != "" {
			path := filepath.Join(folder, step.file)
			if step.remove {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				write(t, path, "content\n")
			}
		}
		if err := os.Chtimes(parent, stamp.ModTime(), stamp.ModTime()); err != nil {
			t.Fatal(err)
		}
		for _, cat := range []*Catalog{held, l.catalog()} {
			names := []string{}
			for _, row := range cat.ShownRows(KindTeam, source.LayerLocal) {
				names = append(names, row.Name)
			}
			if !reflect.DeepEqual(names, step.want) {
				t.Fatalf("after %s remove=%v: rows = %v, want %v", step.file, step.remove, names, step.want)
			}
			definitions := []string{}
			for _, definition := range cat.Definitions() {
				definitions = append(definitions, definition.Name)
			}
			if !reflect.DeepEqual(definitions, step.definitions) {
				t.Errorf("after %s remove=%v: definitions = %v, want %v", step.file, step.remove, definitions, step.definitions)
			}
		}
	}
}
