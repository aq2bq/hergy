package place_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/ops/internal/place"
)

func project(t *testing.T, path string) place.Project {
	t.Helper()
	p, err := place.NewProject(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCanonicalProjectIdentity(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "realDir")
	if err := os.MkdirAll(filepath.Join(realDir, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(mustCwd(t), realDir)
	if err != nil {
		t.Fatal(err)
	}
	want := project(t, realDir)
	for _, path := range []string{alias, realDir + "/", filepath.Join(realDir, "child", ".."), relative} {
		got := project(t, path)
		if got != want || got.Compare(want) != place.Same {
			t.Errorf("%q = %v, want %v", path, got, want)
		}
	}
	if !want.Contains(project(t, filepath.Join(alias, "child"))) {
		t.Fatal("aliased child not inside project")
	}
	if want.Contains(project(t, realDir+"-other")) {
		t.Fatal("sibling counted as child")
	}
}
func mustCwd(t *testing.T) string {
	t.Helper()
	p, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestMissingAndUnknownProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	missing := project(t, path)
	if missing.String() != filepath.Clean(path) {
		t.Fatal(missing)
	}
	relative, err := filepath.Rel(mustCwd(t), path)
	if err != nil {
		t.Fatal(err)
	}
	if project(t, relative) != missing {
		t.Fatal("missing relative and absolute differ")
	}
	unknown := project(t, "")
	for _, pair := range [][2]place.Project{{unknown, missing}, {missing, unknown}, {unknown, unknown}} {
		if pair[0].Compare(pair[1]) != place.Unknown {
			t.Fatal("unknown compared as known")
		}
		if pair[0].Contains(pair[1]) {
			t.Fatal("unknown contained as cwd")
		}
	}
	if missing.Compare(project(t, path+"2")) != place.Different {
		t.Fatal("different missing paths match")
	}
}

func TestPrivateObservedJSONKeepsSpelling(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{alias + "/", "", filepath.Join(root, "missing")} {
		encoded, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		seen := place.Observe(raw)
		if seen.String() != raw || seen.Project() != project(t, raw) {
			t.Fatalf("observed %q = %+v", raw, seen)
		}
		got, err := json.Marshal(seen)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(encoded) {
			t.Fatalf("wire %s, want %s", got, encoded)
		}
	}
}
