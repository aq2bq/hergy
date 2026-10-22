package main

// The e2e gate's toolchain: the browser walks in e2e/ run on Playwright, and
// the gate fetches Node, pnpm and Playwright's Chromium into .bin/ when they
// are not there yet. Nothing lands in the person's GOPATH or their global
// npm/pnpm store: the store and the browsers stand under .bin/ too (d-d922,
// d-3c9b).
//
// The versions are pinned here, so the gate and the machine that runs it agree.

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	e2eNodeVersion       = "24.21.0"
	e2ePnpmVersion       = "11.7.0"
	e2ePlaywrightVersion = "1.64.1"
	// e2eBinary is the hergy the walks drive: built with -tags e2e, the only
	// build that reads HERGY_E2E_NOW (d-f978).
	e2eBinary = "hergy-e2e"
)

// e2eGate drives the browser walks in e2e/ through Playwright. It is a gate of
// its own because a browser is needed: the ordinary test gate builds with no
// tags, and the e2e Go files are gone (d-3c9b).
func (r *runner) e2eGate() error {
	began := time.Now()
	err := r.e2eRun()
	fmt.Fprintf(r.stdout, "-- e2e took %.1fs\n", time.Since(began).Seconds())
	return err
}

func (r *runner) e2eRun() error {
	if err := r.buildE2EBinary(); err != nil {
		return err
	}
	nodeBin, err := r.ensureNode()
	if err != nil {
		return err
	}
	pnpm, err := r.ensurePnpm(nodeBin)
	if err != nil {
		return err
	}
	if err := r.installE2EDeps(pnpm, nodeBin); err != nil {
		return err
	}
	if err := r.ensureBrowsers(pnpm, nodeBin); err != nil {
		return err
	}
	return r.runPlaywright(pnpm, nodeBin)
}

// buildE2EBinary builds the tagged hergy the walks run.
func (r *runner) buildE2EBinary() error {
	return r.exec("go", "build", "-tags", "e2e", "-o", filepath.Join(r.bin, e2eBinary), ".")
}

// e2eEnv is the environment the e2e commands run in: the machine's, with the
// fetched Node first on PATH and the npm/pnpm caches under .bin/, so nothing is
// written to the person's global cache.
func (r *runner) e2eEnv(nodeBin string, extra ...string) []string {
	env := append([]string(nil), os.Environ()...)
	env = append(env, "XDG_CACHE_HOME="+filepath.Join(r.bin, "cache"), "npm_config_cache="+filepath.Join(r.bin, "npm-cache"))
	if nodeBin != "" {
		env = append(env, "PATH="+nodeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return append(env, extra...)
}

// nodeName is the nodejs.org build for this machine.
func nodeName() (string, error) {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" || (runtime.GOOS != "darwin" && runtime.GOOS != "linux") {
		return "", fmt.Errorf("no pinned Node %s for %s/%s", e2eNodeVersion, runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Sprintf("node-v%s-%s-%s", e2eNodeVersion, runtime.GOOS, arch), nil
}

// ensureNode returns the directory the pinned Node stands in, fetching it into
// .bin/ when the stamp or the binary is missing.
func (r *runner) ensureNode() (string, error) {
	name, err := nodeName()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(r.bin, name)
	stamp := filepath.Join(r.bin, "node.version")
	// The directory's name holds the pinned version, so a Node that is there is
	// the one the gate wants: a restored CI cache carries it without the stamp.
	if _, err := os.Stat(filepath.Join(dir, "bin", "node")); err == nil {
		_ = os.WriteFile(stamp, []byte(e2eNodeVersion+"\n"), 0o644)
		return filepath.Join(dir, "bin"), nil
	}
	url := fmt.Sprintf("https://nodejs.org/dist/v%s/%s.tar.gz", e2eNodeVersion, name)
	fmt.Fprintf(r.stdout, "-- fetching %s into %s\n", url, r.bin)
	if err := fetchTarGz(url, r.bin); err != nil {
		return "", err
	}
	if err := os.WriteFile(stamp, []byte(e2eNodeVersion+"\n"), 0o644); err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin"), nil
}

// fetchTarGz downloads one .tar.gz and unpacks it under dir.
func fetchTarGz(url, dir string) error {
	resp, err := http.Get(url) //nolint:gosec // G107: the URL is built from the pinned versions above
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s answers %d", url, resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	return untar(gz, dir)
}

// untar writes one tar stream's entries under dir.
func untar(reader io.Reader, dir string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extract(dir, header, tarReader); err != nil {
			return err
		}
	}
}

// extract writes one entry of a tar stream under dir.
func extract(dir string, header *tar.Header, body io.Reader) error {
	//nolint:gosec // G305: the archive is the pinned Node build; the check below keeps every path under .bin
	target := filepath.Join(dir, header.Name)
	if target != dir && !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
		return fmt.Errorf("the archive names a path outside %s: %s", dir, header.Name)
	}
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeSymlink:
		_ = os.Remove(target)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(header.Linkname, target)
	case tar.TypeReg:
		return writeEntry(target, header.Mode, body)
	default:
		return nil
	}
}

// writeEntry writes one regular file of a tar stream.
func writeEntry(target string, mode int64, body io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	//nolint:gosec // G115: a tar header's mode is an int64, and only its permission bits are kept
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(mode)&0o777)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, body); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// ensurePnpm returns the pinned pnpm, installed with the fetched Node's npm
// into .bin/pnpm when the stamp or the binary is missing.
func (r *runner) ensurePnpm(nodeBin string) (string, error) {
	prefix := filepath.Join(r.bin, "pnpm")
	pnpm := filepath.Join(prefix, "node_modules", ".bin", "pnpm")
	stamp := filepath.Join(r.bin, "pnpm.version")
	// A restored cache carries the pnpm without the stamp; the stamp is written
	// back so the next save holds it.
	if _, err := os.Stat(pnpm); err == nil {
		_ = os.WriteFile(stamp, []byte(e2ePnpmVersion+"\n"), 0o644)
		return pnpm, nil
	}
	fmt.Fprintf(r.stdout, "-- installing pnpm@%s into %s\n", e2ePnpmVersion, prefix)
	env := r.e2eEnv(nodeBin)
	if err := r.execIn(r.root, env, filepath.Join(nodeBin, "npm"),
		"install", "--prefix", prefix, "--no-save", "--no-audit", "--no-fund", "pnpm@"+e2ePnpmVersion); err != nil {
		return "", err
	}
	if err := os.WriteFile(stamp, []byte(e2ePnpmVersion+"\n"), 0o644); err != nil {
		return "", err
	}
	return pnpm, nil
}

// installE2EDeps runs the frozen install of e2e/'s dependencies, with the store
// under .bin/ so the person's global store is never written.
func (r *runner) installE2EDeps(pnpm, nodeBin string) error {
	return r.execIn(filepath.Join(r.root, "e2e"), r.e2eEnv(nodeBin, "PNPM_HOME="+filepath.Join(r.bin, "pnpm-home")),
		pnpm, "install", "--frozen-lockfile", "--prefer-offline", "--store-dir", filepath.Join(r.bin, "pnpm-store"))
}

// ensureBrowsers fetches Playwright's Chromium into .bin/ms-playwright when the
// stamp says it is not there yet.
func (r *runner) ensureBrowsers(pnpm, nodeBin string) error {
	browsers := filepath.Join(r.bin, "ms-playwright")
	stamp := filepath.Join(r.bin, "playwright.version")
	// A restored cache carries the browsers without the stamp; the stamp is
	// written back so the next save holds it.
	if entries, err := os.ReadDir(browsers); err == nil && len(entries) > 0 {
		_ = os.WriteFile(stamp, []byte(e2ePlaywrightVersion+"\n"), 0o644)
		return nil
	}
	if err := r.execIn(filepath.Join(r.root, "e2e"), r.e2eEnv(nodeBin, "PLAYWRIGHT_BROWSERS_PATH="+browsers),
		pnpm, "exec", "playwright", "install", "chromium"); err != nil {
		return err
	}
	return os.WriteFile(stamp, []byte(e2ePlaywrightVersion+"\n"), 0o644)
}

// runPlaywright runs the walks. Every test brings its own project and serve, so
// the worker count is the config's (playwright.config.ts, E2E_WORKERS to try
// another number).
func (r *runner) runPlaywright(pnpm, nodeBin string) error {
	env := r.e2eEnv(nodeBin,
		"PLAYWRIGHT_BROWSERS_PATH="+filepath.Join(r.bin, "ms-playwright"),
		"HERGY_E2E_BIN="+filepath.Join(r.bin, e2eBinary),
		"TZ=Asia/Tokyo",
	)
	return r.execIn(filepath.Join(r.root, "e2e"), env, pnpm, "exec", "playwright", "test")
}

// e2eToolchainText is what -h says about the e2e toolchain.
func e2eToolchainText() string {
	return fmt.Sprintf("  %-17s %s\n  %-17s %s\n  %-17s %s\n",
		"node", e2eNodeVersion,
		"pnpm", e2ePnpmVersion,
		"playwright", e2ePlaywrightVersion+"'s chromium")
}
