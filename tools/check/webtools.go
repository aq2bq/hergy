package main

import (
	"bytes"
	"errors"
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
//   - types: the modules that are TypeScript are held to the compiler's reading
//     (tsc --noEmit, strict, noUnusedLocals, noUnusedParameters), with the
//     project's tsconfig at internal/web/tsconfig.json. src/ reads the API's
//     types and the packages pinned in tools/webvendor through it.
//   - apitypes: the TypeScript types the screens read the API through are what
//     openapi-typescript makes from internal/apicontract/openapi.yaml, the API's
//     one written promise (n-e3de). A promise changed without the types made
//     again fails here (go run ./tools/check apitypes -write makes them).
//   - unit: the modules' own tests (internal/web/jstest/*.test.js) under bun
//     test, with a document from happy-dom and no browser. What a screen
//     decides from its state and the server's answers is held here; the
//     browser walks keep only what a real browser alone can show.

// webTests is where the modules' tests stand: beside src/, not in it, so the
// embedded files the server answers do not include them. They read src/ itself
// (bun runs TypeScript as it stands), so no build comes before them.
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

func (r *runner) typesGate() error {
	// The answers and bodies come from the API's promise, and the packages
	// (preact's types through internal/web/tsconfig.json's paths) from
	// tools/webvendor: the gate installs them itself, so it works on a fresh
	// tree and when the diff run picks it alone.
	if _, _, err := r.webPackages(); err != nil {
		return err
	}
	_, dir, env, err := r.webtools()
	if err != nil {
		return err
	}
	return r.execIn(r.root, env, filepath.Join(dir, "node_modules", ".bin", "tsc"),
		"-p", filepath.Join("internal", "web"), "--noEmit")
}

func (r *runner) namesGate() error {
	_, dir, env, err := r.webtools()
	if err != nil {
		return err
	}
	// eslint reads the files under the directory it runs in, so it runs in
	// internal/web, over the modules and their tests, with the config named
	// from where it was installed. The modules moved to TypeScript are tsc's
	// (the types gate), so this reads the .js that are left.
	return r.execIn(filepath.Join(r.root, "internal", "web"), env, filepath.Join(dir, "node_modules", ".bin", "eslint"),
		"--no-config-lookup", "--config", filepath.Join(dir, "eslint.config.js"), "--max-warnings", "0",
		filepath.Base(webSrc), filepath.Base(webTests))
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
	// bun test runs in internal/web, where tsconfig.json stands: bun reads the
	// tsconfig of the directory it runs in, and a module the tests import as
	// .tsx needs its jsx settings (jsxImportSource preact). Run in jstest, bun
	// found none and looked for react/jsx-dev-runtime.
	return r.execIn(filepath.Join(r.root, "internal", "web"), env, bun, "test", "--isolate", "--preload", filepath.Join(dir, "setup.js"), filepath.Base(webTests))
}

// apiSpec and apiTypes are the API's promise and the types made from it, from
// the module root.
var (
	apiSpec  = filepath.Join("internal", "apicontract", "openapi.yaml")
	apiTypes = filepath.Join("internal", "apicontract", "api-types.d.ts")
)

func (r *runner) apitypesGate() error {
	bun, dir, env, err := r.webtools()
	if err != nil {
		return err
	}
	want := filepath.Join(r.root, apiTypes)
	where := want
	if !r.write {
		tmp, err := os.MkdirTemp("", "hergy-apitypes")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		where = filepath.Join(tmp, "api-types.d.ts")
	}
	cli := filepath.Join(dir, "node_modules", "openapi-typescript", "bin", "cli.js")
	if err := r.execIn(r.root, env, bun, cli, apiSpec, "-o", where); err != nil {
		return fmt.Errorf("openapi-typescript (tools/webtools): %w", err)
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
		return fmt.Errorf("%s: %w (run: go run ./tools/check apitypes -write)", apiTypes, err)
	}
	if !bytes.Equal(have, made) {
		return errors.New(apiTypes + " is not what openapi-typescript makes from " + apiSpec + ": the promise moved and the types were not made again (go run ./tools/check apitypes -write)")
	}
	return nil
}
