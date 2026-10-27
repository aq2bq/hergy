package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The vendor gate: the one ESM file the WebUI's modules import Preact, htm and
// @preact/signals through (internal/web/static/vendor/preact.js) is what
// tools/webvendor makes from exact versions (webui-preact-design 3 章).
//
// Nothing is built while the app runs: static/ is embedded as it is, so the
// file is a source file of the repository, committed like any other. What this
// gate keeps is the file and the packages it was made from agreeing: a version
// moved in package.json or bun.lock, or an export added to entry.js, fails here
// until the file is written again.

// vendorFile is the generated file, from the module root.
var vendorFile = filepath.Join("internal", "web", "static", "vendor", "preact.js")

func (r *runner) vendorGate() error {
	bun, err := r.bun()
	if err != nil {
		return err
	}
	dir := filepath.Join(r.root, "tools", "webvendor")
	// bun's own install cache stands in .bin/ (which CI caches) and nowhere in
	// the user's home, so a run leaves nothing behind outside the repository.
	env := append(os.Environ(), "BUN_INSTALL_CACHE_DIR="+filepath.Join(r.bin, "bun-cache"))
	if err := r.execIn(dir, env, bun, "install", "--frozen-lockfile"); err != nil {
		return fmt.Errorf("bun install --frozen-lockfile in tools/webvendor: %w", err)
	}

	want := filepath.Join(r.root, vendorFile)
	where := want
	if !r.write {
		tmp, err := os.MkdirTemp("", "hergy-vendor")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		where = filepath.Join(tmp, "preact.js")
	}
	if err := r.execIn(dir, append(env, "BUN_BUILD_OUT="+where), bun, "build.js"); err != nil {
		return fmt.Errorf("bun build.js (tools/webvendor): %w", err)
	}
	if r.write {
		return nil
	}

	made, err := os.ReadFile(where)
	if err != nil {
		return err
	}
	have, err := os.ReadFile(want)
	if err != nil {
		return fmt.Errorf("%s: %w (run: go run ./tools/check vendor -write)", vendorFile, err)
	}
	if !bytes.Equal(have, made) {
		return errors.New(vendorFile + " is not what tools/webvendor makes: the versions in package.json/bun.lock or the exports in entry.js moved (write it again: go run ./tools/check vendor -write)")
	}
	return nil
}
