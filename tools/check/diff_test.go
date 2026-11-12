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

// TestACommittedBranchWithNoUpstreamStillSelectsItsChange: a branch nobody
// pushed has no @{upstream}; its committed change must still be compared with
// main, or every gate is skipped after the first commit.
func TestACommittedBranchWithNoUpstreamStillSelectsItsChange(t *testing.T) {
	r := &runner{root: t.TempDir()}
	git := func(args ...string) {
		t.Helper()
		if _, err := r.git(args...); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	git("config", "commit.gpgsign", "false")
	tree(t, r.root, map[string]string{"README.md": "base\n"})
	git("add", ".")
	git("commit", "-qm", "base")
	git("switch", "-c", "wip/no-upstream")
	tree(t, r.root, map[string]string{"internal/cli/serve.go": "package app\n"})
	git("add", ".")
	git("commit", "-qm", "change")
	paths, err := r.changedPaths()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(paths, "internal/cli/serve.go") {
		t.Fatalf("changedPaths = %v, want the committed change", paths)
	}
}

// TestLocalGatesLeaveTheRepositorysCostToCI: the diff run keeps the gates whose
// cost is the change's and leaves e2e, coverage and vuln to CI, in the table's order.
func TestLocalGatesLeaveTheRepositorysCostToCI(t *testing.T) {
	here, later := localGates(gatesFor([]string{"internal/cli/serve.go"}))
	if got := gateNames(here); got != "fmt tidy fix lint arch test deadcode" {
		t.Errorf("here = %q", got)
	}
	if got := gateNames(later); got != "e2e coverage" {
		t.Errorf("later = %q", got)
	}
	here, later = localGates(gatesFor([]string{"go.sum"}))
	if got := gateNames(here); got != "tidy lint test deadcode" {
		t.Errorf("here = %q", got)
	}
	if got := gateNames(later); got != "e2e vuln" {
		t.Errorf("later = %q", got)
	}
}

// TestTestPackagesForTestsWhatChanged: Go changes test their own packages, a
// file a package embeds or reads tests that package, and a file nothing names
// keeps ./... and go test's cache.
func TestTestPackagesForTestsWhatChanged(t *testing.T) {
	cases := []struct {
		paths []string
		want  []string
	}{
		{[]string{"internal/catalog/rows.go", "internal/catalog/rows_test.go", "internal/web/api.go"}, []string{"./internal/catalog", "./internal/web"}},
		{[]string{"internal/catalog/rows.go", "README.md"}, []string{"./.", "./internal/catalog"}},
		{[]string{"internal/i18n/locales/ja.json"}, []string{"./.", "./internal/i18n"}},
		{[]string{"internal/web/src/profiles/state.ts", "internal/web/static/profiles/main.js"}, []string{"./.", "./internal/web"}},
		{[]string{"e2e/tests/nav.spec.ts", ".gitignore"}, nil},
		{[]string{"testdata/hq/config/hergy/config.yaml"}, []string{"./..."}},
		{[]string{"go.mod"}, []string{"./..."}},
		{[]string{"main.go"}, []string{"./."}},
	}
	for _, c := range cases {
		if got := testPackagesFor(c.paths); !slices.Equal(got, c.want) {
			t.Errorf("testPackagesFor(%v) = %v, want %v", c.paths, got, c.want)
		}
	}
}

func TestExistingTestPackagesExcludesTheOldSideOfAMove(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/current/current.go", "package current\n")
	writeTree(t, root, "internal/assets/template.html", "<p>no Go package remains</p>")
	got, err := existingTestPackages(root, []string{"./internal/removed", "./internal/assets", "./internal/current", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./internal/current", "./..."}
	if !slices.Equal(got, want) {
		t.Fatalf("packages=%v want=%v", got, want)
	}
}
