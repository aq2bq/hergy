package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This file holds the WebUI's one door to the packages it is built on. The
// screens read Preact, htm and the signals through src/lib/preact.js and
// nowhere else (webui-preact-design 3 章): what a screen may use is decided in
// that one file, so the packages cannot be reached around it. That decision
// used to stand in the vendor file the screens imported, and the file is the
// build's now (r-893d), so the rule is a test: it reads the sources whatever
// language they are written in, which is what the names gate (eslint, .js
// only) cannot do once a screen moves to TypeScript.

// packageImport is an import of one of the three packages, from the modules
// they are pinned in: preact (and its hooks), htm, and the signals.
var packageImport = regexp.MustCompile(`(?m)^\s*(?:export\s+[^;]*?from|import\s+[^;]*?from|import)\s*['"](?:preact(?:/[a-z-]+)?|htm|@preact/signals(?:/[a-z-]+)?)['"]`)

// typeImport is an import that names types alone. It is erased before a browser
// sees the file, so a screen may name Preact's types where it draws (JSX.Element,
// Signal, ComponentChildren) without a second copy of anything.
var typeImport = regexp.MustCompile(`(?ms)^\s*(?:import|export)\s+type\b.*?(?:;|$)`)

// preactModule is the one file allowed to name them.
const preactModule = "lib/preact"

// TestOnlyTheOneModuleImportsThePackages: the modules under src/ import the
// three packages only through lib/preact.js (lib/preact.ts when it moves).
func TestOnlyTheOneModuleImportsThePackages(t *testing.T) {
	root := "src"
	err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".js", ".jsx", ".ts", ".tsx":
		default:
			return nil
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		body := typeImport.ReplaceAllString(string(raw), "")
		if !packageImport.MatchString(body) {
			return nil
		}
		if strings.TrimSuffix(path, filepath.Ext(path)) == preactModule {
			return nil
		}
		t.Errorf("%s imports preact, htm or @preact/signals itself: read them through %s so what a screen may use is decided in one place (webui-preact-design 3 章)",
			path, preactModule+filepath.Ext(path))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}
