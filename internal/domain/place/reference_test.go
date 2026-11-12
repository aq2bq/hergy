package place_test

import (
	"testing"

	"github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/domain/place"
)

func project(t *testing.T, raw string) place.Project {
	t.Helper()
	p, e := place.NewProject(raw)
	if e != nil {
		t.Fatal(e)
	}
	return p
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
	for _, tc := range []struct {
		ref  place.Ref
		kind place.Kind
		name string
	}{
		{place.DefinitionRef(local, definition), place.Definition, "demo"},
		{place.RoleRef(local, role), place.Role, "builder"},
		{place.ProfileRef(local, profile), place.Profile, "default"},
	} {
		if tc.ref.Place() != local || tc.ref.Kind() != tc.kind || tc.ref.Name() != tc.name {
			t.Fatal(tc.ref)
		}
	}
}
