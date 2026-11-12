package place_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/domain/place"
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
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(filepath.Join(real, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(mustCwd(t), real)
	if err != nil {
		t.Fatal(err)
	}
	want := project(t, real)
	for _, path := range []string{alias, real + "/", filepath.Join(real, "child", ".."), relative} {
		got := project(t, path)
		if got != want || got.Compare(want) != place.Same {
			t.Errorf("%q = %v, want %v", path, got, want)
		}
	}
	if !want.Contains(project(t, filepath.Join(alias, "child"))) {
		t.Fatal("aliased child not inside project")
	}
	if want.Contains(project(t, real+"-other")) {
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

func TestObservedJSONKeepsSpelling(t *testing.T) {
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
		var seen place.Seen
		if err := json.Unmarshal(encoded, &seen); err != nil {
			t.Fatal(err)
		}
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

func TestPlaceAndTypedResourceReferences(t *testing.T) {
	p := project(t, t.TempDir())
	local := place.In(p)
	if local.IsGlobal() || local.Project() != p || local.API() != "project" || local.Layer() != "local" {
		t.Fatal(local)
	}
	global := place.Global()
	if !global.IsGlobal() || global.API() != "global" || global.Layer() != "global" {
		t.Fatal(global)
	}
	if place.In(place.Project{}).IsGlobal() {
		t.Fatal("unknown project converted to global")
	}
	for _, s := range []place.Selection{"project", "local", "unknown", ""} {
		if s.At(p) != local || s.Layer() != "local" || s.API() != "project" {
			t.Fatal(s)
		}
	}
	if place.Selection("global").At(p) != global {
		t.Fatal("global selection changed")
	}
	definition, _ := name.ParseDefinitionName("demo")
	role, _ := name.ParseRoleName("builder")
	profile, _ := name.ParseProfileName("default")
	for _, ref := range []place.Ref{place.DefinitionRef(local, definition), place.RoleRef(local, role), place.ProfileRef(local, profile)} {
		if ref.Place() != local || ref.Name() == "" {
			t.Fatal(ref)
		}
	}
}
