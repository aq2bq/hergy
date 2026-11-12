package main

import (
	"go/parser"
	"go/token"
	"io/fs"
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

// leaves have no operational dependencies; source uses place, which uses name.
var leaves = []string{"browser", "domain/name", "domain/place", "i18n", "layout", "scaffold", "source", "version"}

// importsOf maps each non-test Go file to the module packages it imports, as
// package names under internal/ ("cli", "i18n", …); main.go is "main".
func importsOf(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	walk := func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
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
			switch {
			case p == modulePath:
				imports = append(imports, "main")
			case strings.HasPrefix(p, modulePath+"/internal/"):
				imports = append(imports, strings.TrimPrefix(p, modulePath+"/internal/"))
			case strings.HasPrefix(p, modulePath+"/"):
				imports = append(imports, strings.TrimPrefix(p, modulePath+"/"))
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
	return strings.TrimPrefix(filepath.ToSlash(filepath.Dir(file)), "internal/")
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
				// The model's place depends only on typed names. The source
				// boundary uses place to give locks the same project identity.
				if (pkg == "domain/place" && imp == "domain/name") || (pkg == "source" && imp == "domain/place") || (strings.HasPrefix(pkg, "domain/") && imp == "ops/model/"+strings.TrimPrefix(pkg, "domain/")) {
					continue
				}
				t.Errorf("%s: %s is a leaf but imports internal/%s", f, pkg, imp)
			}
		}
	})

	t.Run("ops does not import boundary packages", func(t *testing.T) {
		for _, f := range files {
			if packageOf(f) == "ops" || strings.HasPrefix(packageOf(f), "ops/") {
				for _, imp := range imports[f] {
					if imp == "cli" || imp == "web" {
						t.Errorf("%s: ops must not import %s", f, imp)
					}
				}
			}
		}
	})

	t.Run("only main imports cli", func(t *testing.T) {
		for _, f := range files {
			if packageOf(f) != "main" && slices.Contains(imports[f], "cli") {
				t.Errorf("%s: only main.go may import internal/cli", f)
			}
		}
	})

	t.Run("the rules name real packages", func(t *testing.T) {
		seen := map[string]bool{}
		for _, f := range files {
			seen[packageOf(f)] = true
		}
		for _, pkg := range append(append([]string{}, leaves...), "cli", "ops", "web") {
			if !seen[pkg] {
				t.Errorf("internal/%s is in the rules but has no Go file", pkg)
			}
		}
	})
}
