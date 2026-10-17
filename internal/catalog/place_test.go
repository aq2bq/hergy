package catalog

import (
	"path/filepath"
	"testing"
)

// TestNewPlaceInsideTheProject writes inside the project: a git repository
// keeps what hergy makes in its own layer (C13, K2).
func TestNewPlaceInsideTheProject(t *testing.T) {
	l := newLayers(t)
	cat := l.catalog().InRepo(true)

	cases := []struct {
		kind string
		name string
		want string
		what string
	}{
		{NewTeam, "t", filepath.Join(l.local, "teams", "t"), "a team definition's directory"},
		{NewRole, "watcher", filepath.Join(l.local, "roles", "watcher.yaml"), "a role document"},
		{NewProfile, "builder", filepath.Join(l.local, "profiles.yaml"), "a profile"},
	}
	for _, tc := range cases {
		if got := cat.NewPlace(tc.kind, tc.name); got != tc.want {
			t.Errorf("NewPlace(%q, %q) = %q, want %q (%s)", tc.kind, tc.name, got, tc.want, tc.what)
		}
	}
}

// TestNewPlaceOutsideTheProject writes outside it: with no repository, what
// hergy makes goes into the global layer (C13, K2).
func TestNewPlaceOutsideTheProject(t *testing.T) {
	l := newLayers(t)
	for _, cat := range []*Catalog{l.catalog().InRepo(false), l.catalog()} {
		if got, want := cat.NewPlace(NewTeam, "t"), filepath.Join(l.global, "teams", "t"); got != want {
			t.Errorf("NewPlace(NewTeam, t) = %q, want %q", got, want)
		}
		if got, want := cat.NewPlace(NewRole, "watcher"), filepath.Join(l.global, "roles", "watcher.yaml"); got != want {
			t.Errorf("NewPlace(NewRole, watcher) = %q, want %q", got, want)
		}
		if got, want := cat.NewPlace(NewProfile, "builder"), filepath.Join(l.global, "profiles.yaml"); got != want {
			t.Errorf("NewPlace(NewProfile, builder) = %q, want %q", got, want)
		}
	}
}

// TestNewPlaceOfAnUnknownKind answers nothing: a thing hergy does not make has
// no place (C13).
func TestNewPlaceOfAnUnknownKind(t *testing.T) {
	if got := newLayers(t).catalog().NewPlace("something-else", "x"); got != "" {
		t.Errorf("NewPlace of an unknown kind = %q, want none", got)
	}
}
