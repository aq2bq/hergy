package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// The WebUI's modules get two gates before a browser opens them, both run
// from the tools pinned in tools/webtools:
//
//   - names: every name a module uses is defined in the module, imported, or
//     one of the browser's own (eslint's no-undef), and every name it defines
//     or imports is read (no-unused-vars). A module that forgot an import
//     throws only when the line runs, so a road no walk opens (the
//     headquarters' roles list, 3ec7db3) is caught here rather than in a
//     person's browser; and the deadcode gate reads Go only.
//   - unit: the modules' own tests (internal/web/jstest/*.test.js) under bun
//     test, with a document from happy-dom and no browser. What a screen
//     decides from its state and the server's answers is held here; the
//     browser walks keep only what a real browser alone can show.

// webStatic is the WebUI's modules, from the module root; static/vendor/ is
// left out by tools/webtools/eslint.config.js.
var webStatic = filepath.Join("internal", "web", "static")

// webTests is where the modules' tests stand: beside static/, not in it, so
// the embedded files the server answers do not include them.
var webTests = filepath.Join("internal", "web", "jstest")

// webtools installs tools/webtools under .bin/webtools and returns that
// directory and the environment to run its tools in. It is installed there,
// not beside its package.json: Go reads every directory of the module but the
// dot ones, and one of eslint's dependencies ships .go files the Go gates
// would then read as hergy's.
func (r *runner) webtools() (bun, dir string, env []string, err error) {
	bun, err = r.bun()
	if err != nil {
		return "", "", nil, err
	}
	src := filepath.Join(r.root, "tools", "webtools")
	dir = filepath.Join(r.bin, "webtools")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", nil, err
	}
	for _, name := range []string{"package.json", "bun.lock", "eslint.config.js", "setup.js"} {
		raw, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			return "", "", nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil { //nolint:gosec // G703: dir is .bin/webtools and name one of the four fixed above
			return "", "", nil, err
		}
	}
	env = append(os.Environ(), "BUN_INSTALL_CACHE_DIR="+filepath.Join(r.bin, "bun-cache"))
	if err := r.execIn(dir, env, bun, "install", "--frozen-lockfile"); err != nil {
		return "", "", nil, fmt.Errorf("bun install --frozen-lockfile for tools/webtools: %w", err)
	}
	return bun, dir, env, nil
}

func (r *runner) namesGate() error {
	_, dir, env, err := r.webtools()
	if err != nil {
		return err
	}
	// eslint reads the files under the directory it runs in, so it runs in
	// internal/web, over the modules and their tests, with the config named
	// from where it was installed.
	return r.execIn(filepath.Join(r.root, "internal", "web"), env, filepath.Join(dir, "node_modules", ".bin", "eslint"),
		"--no-config-lookup", "--config", filepath.Join(dir, "eslint.config.js"), "--max-warnings", "0",
		filepath.Base(webStatic), filepath.Base(webTests))
}

func (r *runner) unitGate() error {
	bun, dir, env, err := r.webtools()
	if err != nil {
		return err
	}
	// The preload is named from where it was installed, so it finds happy-dom
	// there; the tests themselves import only bun:test and the modules.
	// --isolate gives each file a fresh global and module registry: a screen's
	// module is imported once (its state stands per page), so two files that
	// each import one would otherwise read the first file's page.
	return r.execIn(filepath.Join(r.root, webTests), env, bun, "test", "--isolate", "--preload", filepath.Join(dir, "setup.js"), ".")
}
