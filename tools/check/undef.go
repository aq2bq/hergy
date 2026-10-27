package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// The undef gate: every name the WebUI's modules use is defined in the module,
// imported, or one of the browser's own (eslint's no-undef, from
// tools/weblint). A module that forgot an import throws only when the line runs,
// so a road no walk opens (the headquarters' roles list, 3ec7db3) is caught here
// rather than in a person's browser.

// webStatic is the WebUI's modules, from the module root; vendor/ is left out
// by tools/weblint/eslint.config.js.
var webStatic = filepath.Join("internal", "web", "static")

func (r *runner) undefGate() error {
	bun, err := r.bun()
	if err != nil {
		return err
	}
	// eslint is installed under .bin/, not beside its package.json: Go reads
	// every directory of the module but the dot ones, and one of eslint's
	// dependencies ships .go files the Go gates would then read as hergy's.
	src := filepath.Join(r.root, "tools", "weblint")
	dir := filepath.Join(r.bin, "weblint")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"package.json", "bun.lock", "eslint.config.js"} {
		raw, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil { //nolint:gosec // G703: dir is .bin/weblint and name one of the three fixed above
			return err
		}
	}
	env := append(os.Environ(), "BUN_INSTALL_CACHE_DIR="+filepath.Join(r.bin, "bun-cache"))
	if err := r.execIn(dir, env, bun, "install", "--frozen-lockfile"); err != nil {
		return fmt.Errorf("bun install --frozen-lockfile for tools/weblint: %w", err)
	}
	// eslint reads the files under the directory it runs in, so it runs in
	// static/ with the config named from where it was installed.
	return r.execIn(filepath.Join(r.root, webStatic), env, filepath.Join(dir, "node_modules", ".bin", "eslint"),
		"--no-config-lookup", "--config", filepath.Join(dir, "eslint.config.js"), "--max-warnings", "0", ".")
}
