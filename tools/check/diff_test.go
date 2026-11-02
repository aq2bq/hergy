package main

import (
	"slices"
	"testing"
)

func TestChangedPathsKeepsBothSidesOfAMove(t *testing.T) {
	r := &runner{root: t.TempDir()}
	git := func(args ...string) {
		t.Helper()
		if _, err := r.git(args...); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	git("config", "diff.renames", "true")
	old, moved := "screen.go", "internal/cheat/moved.go"
	tree(t, r.root, map[string]string{old: "package main\n", "internal/cheat/.keep": ""})
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-qm", "fixture")
	git("mv", old, moved)
	paths, err := r.changedPaths()
	if err != nil || !slices.Contains(paths, old) || !slices.Contains(paths, moved) {
		t.Fatalf("changedPaths = %v, %v", paths, err)
	}
	if !slices.ContainsFunc(gatesFor(paths), func(g gate) bool { return g.name == "e2e" }) {
		t.Fatal("moving browser code into a CLI directory must keep e2e")
	}
}

func TestSpecsForKeepsSharedAndUnknownWalks(t *testing.T) {
	specs := []string{"/repo/e2e/tests/profiles.spec.ts", "/repo/e2e/tests/roles.spec.ts", "/repo/e2e/tests/nav.spec.ts", "/repo/e2e/tests/new.spec.ts"}
	cases := []struct {
		paths []string
		want  []string
	}{
		{[]string{"internal/web/src/profiles/state.ts", "internal/web/static/profiles/main.js"}, []string{"tests/nav.spec.ts", "tests/new.spec.ts", "tests/profiles.spec.ts"}},
		{[]string{"internal/web/src/profiles/view.tsx", "internal/web/src/roles/state.ts"}, []string{"tests/nav.spec.ts", "tests/new.spec.ts", "tests/profiles.spec.ts", "tests/roles.spec.ts"}},
		{[]string{"e2e/tests/roles.spec.ts", "README.md"}, []string{"tests/roles.spec.ts"}},
		{[]string{"internal/web/src/profiles/state.ts", "e2e/tests/roles.spec.ts"}, []string{"tests/nav.spec.ts", "tests/new.spec.ts", "tests/profiles.spec.ts", "tests/roles.spec.ts"}},
		{[]string{"internal/web/src/lib/copy.ts"}, nil},
		{[]string{"internal/web/static/chunk/new.js"}, nil},
		{[]string{"internal/web/src/unknown/main.tsx"}, nil},
		{[]string{"internal/web/src/profiles/state.ts", "internal/web/api_profile.go"}, nil},
		{[]string{"e2e/tests/deleted.spec.ts"}, nil},
		{[]string{"e2e/tests/helpers.ts"}, nil},
		{[]string{"internal/web/tsconfig.json"}, nil},
	}
	for _, c := range cases {
		if got := specsFor(c.paths, specs); !slices.Equal(got, c.want) {
			t.Errorf("specsFor(%v) = %v, want %v", c.paths, got, c.want)
		}
	}
}
