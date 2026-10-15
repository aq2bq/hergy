package source

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEntryDoc(t *testing.T) {
	cases := []struct {
		name  string
		entry string
		base  string
		layer string
		want  string
	}{
		{"relative entry is joined to base", "roles", "/team", LayerLocal, "/team/roles"},
		{"nested relative entry", filepath.Join("docs", "guide.md"), "/team", LayerGlobal, "/team/docs/guide.md"},
		{"absolute entry is kept", "/elsewhere/doc.md", "/team", LayerLocal, "/elsewhere/doc.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EntryDoc(tc.entry, tc.base, tc.layer)
			if got.Path != tc.want {
				t.Errorf("Path = %q, want %q", got.Path, tc.want)
			}
			if got.Layer != tc.layer {
				t.Errorf("Layer = %q, want %q", got.Layer, tc.layer)
			}
		})
	}
}

func TestYAMLInReadsTopLevelByName(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "b.yaml"), "b")
	write(t, filepath.Join(dir, "a.yaml"), "a")
	write(t, filepath.Join(dir, "note.md"), "not read")
	write(t, filepath.Join(dir, "team.yaml.bak"), "not a .yaml file")
	write(t, filepath.Join(dir, "sub", "d.yaml"), "deeper")

	got, err := Disk{}.YAMLIn(Doc{Path: dir, Layer: LayerLocal})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "a.yaml"), filepath.Join(dir, "b.yaml")}
	if len(got) != len(want) {
		t.Fatalf("YAMLIn returned %v, want %v", got, want)
	}
	for i, doc := range got {
		if doc.Path != want[i] || doc.Layer != LayerLocal {
			t.Errorf("doc %d = %+v, want path %q in %q", i, doc, want[i], LayerLocal)
		}
	}
}

func TestMarkdownInReadsTopLevelByName(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "b.md"), "b")
	write(t, filepath.Join(dir, "a.md"), "a")
	write(t, filepath.Join(dir, "c.MD"), "wrong case")
	write(t, filepath.Join(dir, "note.txt"), "not markdown")
	write(t, filepath.Join(dir, "sub", "d.md"), "deeper")

	got, err := Disk{}.MarkdownIn(Doc{Path: dir, Layer: LayerGlobal})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")}
	if len(got) != len(want) {
		t.Fatalf("MarkdownIn returned %v, want %v", got, want)
	}
	for i, doc := range got {
		if doc.Path != want[i] {
			t.Errorf("doc %d path = %q, want %q", i, doc.Path, want[i])
		}
		if doc.Layer != LayerGlobal {
			t.Errorf("doc %d layer = %q, want %q", i, doc.Layer, LayerGlobal)
		}
	}
}

func TestMarkdownInMissingDirectory(t *testing.T) {
	if _, err := (Disk{}).MarkdownIn(Doc{Path: filepath.Join(t.TempDir(), "absent")}); err == nil {
		t.Fatal("MarkdownIn of a missing directory succeeded, want an error")
	}
}

func TestDiskIsFileAndIsDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "team.yaml")
	write(t, file, "version: 1")

	disk := Disk{}
	cases := []struct {
		name     string
		path     string
		wantFile bool
		wantDir  bool
	}{
		{"file", file, true, false},
		{"directory", dir, false, true},
		{"missing", filepath.Join(dir, "absent"), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := Doc{Path: tc.path}
			if got := disk.IsFile(doc); got != tc.wantFile {
				t.Errorf("IsFile = %v, want %v", got, tc.wantFile)
			}
			if got := disk.IsDir(doc); got != tc.wantDir {
				t.Errorf("IsDir = %v, want %v", got, tc.wantDir)
			}
		})
	}
}

func TestDiskRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	write(t, path, "hello\n")
	text, err := Disk{}.Read(Doc{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello\n" {
		t.Errorf("Read = %q, want %q", text, "hello\n")
	}
}
