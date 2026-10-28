package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// The webbuild gate: the scripts internal/web/static serves are what
// internal/web/src makes, built by tools/webvendor/build-site.js from the
// packages pinned there (webui-preact-design 3 章, r-893d).
//
// Nothing is built while the app runs: static/ is embedded as it is, so the
// built files are committed like any other. What this gate keeps is the two
// agreeing — src, the templates' entries and the pinned versions on one side,
// the committed files on the other. A screen edited without a rebuild, a
// template that names another file, or a version moved in package.json fails
// here until the files are written again.
//
// The only files this gate writes or removes are the .js the build makes. CSS,
// fonts and everything else under static/ are the styles' and the repository's,
// not the build's (r-893d).

// webPaths are where the WebUI's sources, its served files and the build stand,
// from the module root.
var (
	webSrc      = filepath.Join("internal", "web", "src")
	webStatic   = filepath.Join("internal", "web", "static")
	webBuildDir = filepath.Join("tools", "webvendor")
	webBuildJS  = filepath.Join("tools", "webvendor", "build-site.js")
)

func (r *runner) webbuildGate() error {
	built, err := r.buildWeb()
	if err != nil {
		return err
	}
	if r.write {
		return writeWebTree(r.root, built)
	}
	return compareWebTree(r.root, built)
}

// webPackages installs the packages the WebUI is built from, exactly as
// tools/webvendor/package.json and bun.lock pin them, and answers the bun that
// installed them and the environment to run the WebUI's tools in. Both the build
// and the types gate need them: tsc reads preact's types through
// internal/web/tsconfig.json's paths, into this node_modules, so a gate that
// runs on its own (types, without webbuild before it) has to install them too.
//
// bun's own install cache stands in .bin/ (which CI caches) and nowhere in the
// user's home, so a run leaves nothing behind outside the repository.
func (r *runner) webPackages() (bun string, env []string, err error) {
	bun, err = r.bun()
	if err != nil {
		return "", nil, err
	}
	env = append(os.Environ(), "BUN_INSTALL_CACHE_DIR="+filepath.Join(r.bin, "bun-cache"))
	if err := r.execIn(filepath.Join(r.root, webBuildDir), env, bun, "install", "--frozen-lockfile"); err != nil {
		return "", nil, fmt.Errorf("bun install --frozen-lockfile in %s: %w", webBuildDir, err)
	}
	return bun, env, nil
}

// buildWeb builds the WebUI into a directory of its own and answers the files
// it made, by the path static/ serves them at.
func (r *runner) buildWeb() (map[string][]byte, error) {
	bun, env, err := r.webPackages()
	if err != nil {
		return nil, err
	}

	out, err := os.MkdirTemp("", "hergy-webbuild")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(out) }()

	// The build runs where the sources are: its entries are named relative to
	// src/, so each one comes out at the path the template loads it at.
	script, err := filepath.Abs(filepath.Join(r.root, webBuildJS))
	if err != nil {
		return nil, err
	}
	buildEnv := append(slices.Clip(env), "HERGY_WEB_OUT="+out)
	if err := r.execIn(filepath.Join(r.root, webSrc), buildEnv, bun, script); err != nil {
		return nil, fmt.Errorf("bun %s: %w", webBuildJS, err)
	}
	return readWebTree(out)
}

// readWebTree reads every file of a built tree, by its path from the tree's
// root, which is the path static/ serves it at.
func readWebTree(root string) (map[string][]byte, error) {
	var rels []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rels = append(rels, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(rels))
	for _, rel := range rels {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return nil, err
		}
		out[filepath.ToSlash(rel)] = raw
	}
	return out, nil
}

// compareWebTree is what the gate fails on: a file the build made that static/
// does not hold, or holds differently, and a .js static/ holds that the build
// did not make.
func compareWebTree(root string, built map[string][]byte) error {
	static := filepath.Join(root, webStatic)
	var wrong []string
	for _, rel := range sortedKeys(built) {
		path := filepath.Join(static, filepath.FromSlash(rel))
		have, err := os.ReadFile(path)
		switch {
		case err != nil:
			wrong = append(wrong, fmt.Sprintf("%s: the build makes it and static/ does not hold it", rel))
		case string(have) != string(built[rel]):
			wrong = append(wrong, fmt.Sprintf("%s: static/ is not what internal/web/src makes of it", rel))
		}
	}
	stale, err := webStale(static, built)
	if err != nil {
		return err
	}
	for _, rel := range stale {
		wrong = append(wrong, fmt.Sprintf("%s: static/ holds it and the build does not make it", rel))
	}
	if len(wrong) > 0 {
		return fmt.Errorf("%s\nrun: go run ./tools/check webbuild -write", strings.Join(wrong, "\n"))
	}
	return nil
}

// webStale is the .js files static/ holds that the build did not make. Nothing
// else is looked at: the styles, the fonts and every other file there are not
// the build's.
func webStale(static string, built map[string][]byte) ([]string, error) {
	var out []string
	err := filepath.WalkDir(static, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}
		rel, err := filepath.Rel(static, path)
		if err != nil {
			return err
		}
		if _, ok := built[filepath.ToSlash(rel)]; !ok {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// writeWebTree writes the built files over static/ and removes the .js the
// build no longer makes. A file that is not a .js the build makes is never
// touched.
func writeWebTree(root string, built map[string][]byte) error {
	static := filepath.Join(root, webStatic)
	for _, rel := range sortedKeys(built) {
		path := filepath.Join(static, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, built[rel], 0o644); err != nil {
			return err
		}
	}
	stale, err := webStale(static, built)
	if err != nil {
		return err
	}
	for _, rel := range stale {
		if err := os.Remove(filepath.Join(static, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	// A directory the build emptied is taken away too, so the tree static/
	// serves is the tree the build makes: only directories that are now empty.
	removeEmptyDirs(static)
	return nil
}

// removeEmptyDirs removes the directories under root that hold nothing. static/
// itself and the directories that still hold a file are left.
func removeEmptyDirs(root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a tree that cannot be read is left as it is
		}
		if d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, dir := range dirs {
		_ = os.Remove(dir) // it goes only when it is empty
	}
}

func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
