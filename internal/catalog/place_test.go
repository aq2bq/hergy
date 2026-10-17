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
		{NewAgreement, "", filepath.Join(l.local, "agreement.yaml"), "the agreement every team shares"},
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
	cat := l.catalog().InRepo(false)
	if got, want := cat.NewPlace(NewTeam, "t"), filepath.Join(l.global, "teams", "t"); got != want {
		t.Errorf("NewPlace(NewTeam, t) = %q, want %q", got, want)
	}
	if got, want := cat.NewPlace(NewRole, "watcher"), filepath.Join(l.global, "roles", "watcher.yaml"); got != want {
		t.Errorf("NewPlace(NewRole, watcher) = %q, want %q", got, want)
	}
	if got, want := cat.NewPlace(NewProfile, "builder"), filepath.Join(l.global, "profiles.yaml"); got != want {
		t.Errorf("NewPlace(NewProfile, builder) = %q, want %q", got, want)
	}
	if got, want := cat.NewPlace(NewAgreement, ""), filepath.Join(l.global, "agreement.yaml"); got != want {
		t.Errorf("NewPlace(NewAgreement, \"\") = %q, want %q", got, want)
	}
}

// TestNewPlaceWithoutInRepoAnswersNothing: a catalog nobody told whether the
// project is a repository answers no place, rather than writing outside the
// project by default (spec/38 T3).
func TestNewPlaceWithoutInRepoAnswersNothing(t *testing.T) {
	cat := newLayers(t).catalog()
	for _, kind := range []string{NewTeam, NewRole, NewProfile, NewAgreement, "something-else"} {
		if got := cat.NewPlace(kind, "x"); got != "" {
			t.Errorf("NewPlace(%q, x) = %q, want none", kind, got)
		}
	}
}

// TestInRepoLeavesTheCatalogItWasCalledOn: InRepo answers a copy, so a catalog
// that was built once is not quietly told a fact by another caller (spec/38 T3).
func TestInRepoLeavesTheCatalogItWasCalledOn(t *testing.T) {
	l := newLayers(t)
	was := l.catalog()
	inside := was.InRepo(true)
	if got := was.NewPlace(NewTeam, "t"); got != "" {
		t.Errorf("the catalog InRepo was called on answers %q, want none", got)
	}
	if got, want := inside.NewPlace(NewTeam, "t"), filepath.Join(l.local, "teams", "t"); got != want {
		t.Errorf("the catalog InRepo answered = %q, want %q", got, want)
	}
	if got, want := was.InRepo(false).NewPlace(NewTeam, "t"), filepath.Join(l.global, "teams", "t"); got != want {
		t.Errorf("InRepo(false) = %q, want %q", got, want)
	}
	if got := was.NewPlace(NewTeam, "t"); got != "" {
		t.Errorf("the catalog InRepo was called on answers %q, want none", got)
	}
}

// TestNewPlaceOfAnUnknownKind answers nothing: a thing hergy does not make has
// no place (C13).
func TestNewPlaceOfAnUnknownKind(t *testing.T) {
	if got := newLayers(t).catalog().InRepo(true).NewPlace("something-else", "x"); got != "" {
		t.Errorf("NewPlace of an unknown kind = %q, want none", got)
	}
}
