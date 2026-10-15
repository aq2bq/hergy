package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The direction of imports between hergy's packages, read from the files
// themselves (not from the go tool, so this test needs no subprocess).
// The rules are the graph as it stood when the gates were introduced (d-ddd2
// D2); go run ./tools/check runs this test as the "arch" gate.

const modulePath = "github.com/aq2bq/hergy"

// leaves import no package of this module.
var leaves = []string{"i18n", "layout", "scaffold", "source", "version"}

// topLayer is imported by internal/app only. A package nested under one of
// these (internal/web/rulesview) is a part of it and may import its parent; the
// nested package itself is listed too, so nothing but app reaches it.
var topLayer = []string{"cheat", "herdrcli", "launcher", "spawn", "team", "web", "web/rulesview"}

// importsOf maps each non-test Go file to the module packages it imports,
// as package names under internal/ ("app", "i18n", …); main.go is "main".
func importsOf(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	walk := func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		var imports []string
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if p == modulePath || strings.HasPrefix(p, modulePath+"/") {
				imports = append(imports, path.Base(p))
			}
		}
		sort.Strings(imports)
		out[filepath.ToSlash(file)] = imports
		return nil
	}
	if err := filepath.WalkDir("internal", walk); err != nil {
		t.Fatal(err)
	}
	if err := walk("main.go", fakeEntry{}, nil); err != nil {
		t.Fatal(err)
	}
	return out
}

type fakeEntry struct{ fs.DirEntry }

func (fakeEntry) IsDir() bool { return false }

// packageOf is the package a file under internal/ belongs to, as the path under
// internal/ writes it: a package nested under another (internal/web/rulesview)
// is told apart from its parent, because it is a package of its own.
func packageOf(file string) string {
	if file == "main.go" {
		return "main"
	}
	return strings.TrimPrefix(path.Dir(file), "internal/")
}

func TestImportDirection(t *testing.T) {
	imports := importsOf(t)
	files := make([]string, 0, len(imports))
	for f := range imports {
		files = append(files, f)
	}
	sort.Strings(files)

	t.Run("leaves import nothing of the module", func(t *testing.T) {
		for _, f := range files {
			pkg := packageOf(f)
			if !slices.Contains(leaves, pkg) {
				continue
			}
			for _, imp := range imports[f] {
				t.Errorf("%s: %s is a leaf but imports internal/%s", f, pkg, imp)
			}
		}
	})

	t.Run("only main imports app", func(t *testing.T) {
		for _, f := range files {
			if packageOf(f) != "main" && slices.Contains(imports[f], "app") {
				t.Errorf("%s: only main.go may import internal/app", f)
			}
		}
	})

	t.Run("only app imports the top layer", func(t *testing.T) {
		for _, f := range files {
			pkg := packageOf(f)
			if pkg == "app" || pkg == "main" {
				continue
			}
			for _, imp := range imports[f] {
				if !slices.Contains(topLayer, imp) {
					continue
				}
				// A package nested inside a top-layer package
				// (internal/web/homeview, internal/web/rulesview) is part of
				// that layer, not a lower one reaching up: it renders through
				// internal/web's frame, and app is still the one importer of
				// the layer.
				if strings.HasPrefix(pkg, imp+"/") {
					continue
				}
				t.Errorf("%s: internal/%s is imported by app only, not by %s", f, imp, pkg)
			}
		}
	})

	t.Run("the rules name real packages", func(t *testing.T) {
		seen := map[string]bool{}
		for _, f := range files {
			seen[packageOf(f)] = true
		}
		for _, pkg := range append(append([]string{}, leaves...), topLayer...) {
			if !seen[pkg] {
				t.Errorf("internal/%s is in the rules but has no Go file", pkg)
			}
		}
	})
}
