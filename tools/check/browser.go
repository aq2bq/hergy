package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The browser walks' tools (d-3c9b, d-bb0d): Playwright, run by bun, with both
// pinned. bun is fetched into .bin/ beside the Go tools the first time it is
// needed, and so is the browser Playwright drives; the person's home directory
// and GOPATH are not written to, on a laptop or on CI.
//
// The e2e directory holds the walks: e2e/tests/*.spec.ts, its dependency in
// e2e/node_modules (bun install, from e2e/bun.lock), and the browser in
// e2e/.bin/browsers. All three are ignored by git.
const (
	// bunVersion is the bun the gate runs; the switch from the measured 1.4.2 to
	// 1.4.3 is d-bb0d's own (2026-10-22 の最新).
	bunVersion = "1.4.3"
	// e2eDir is where the walks and their lockfile stand, from the module root.
	e2eDir = "e2e"
)

// bun returns the pinned bun, fetching it into .bin/ when the binary or its
// version stamp is missing — the same shape as the Go tools go install puts
// there (runner.tool).
func (r *runner) bun() (string, error) {
	path := filepath.Join(r.bin, "bun")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	stamp := filepath.Join(r.bin, "bun.version")
	if have, err := os.ReadFile(stamp); err == nil && strings.TrimSpace(string(have)) == bunVersion {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	url, err := bunURL()
	if err != nil {
		return "", err
	}
	fmt.Fprintf(r.stdout, "-- installing bun@%s into %s\n", bunVersion, r.bin)
	if err := fetchZipEntry(url, "bun", path); err != nil {
		return "", fmt.Errorf("fetch bun@%s: %w", bunVersion, err)
	}
	if err := os.WriteFile(stamp, []byte(bunVersion+"\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// bunURL is where the release for this machine stands. bun names its builds by
// the operating system and the processor, in its own words.
func bunURL() (string, error) {
	osName := map[string]string{"darwin": "darwin", "linux": "linux"}[runtime.GOOS]
	arch := map[string]string{"amd64": "x64", "arm64": "aarch64"}[runtime.GOARCH]
	if osName == "" || arch == "" {
		return "", fmt.Errorf("no bun build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Sprintf(
		"https://github.com/oven-sh/bun/releases/download/bun-v%s/bun-%s-%s.zip",
		bunVersion, osName, arch,
	), nil
}

// fetchZipEntry downloads one zip and writes the entry named name to to, with
// the permission a program needs. The archives bun and Playwright publish hold
// the binary at a path inside a directory of their own, so the entry is found by
// its file name rather than by its place.
func fetchZipEntry(url, name, to string) error {
	//nolint:gosec // G107: the URL is built here from fixed parts and this repository's own version.
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answers %d", url, resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	for _, entry := range archive.File {
		if filepath.Base(entry.Name) != name {
			continue
		}
		return writeZipEntry(entry, to)
	}
	return fmt.Errorf("%s holds no %s", url, name)
}

// maxEntry is the most one entry may write: bun's own archive is about a
// hundred megabytes unpacked, so this is room to spare and a stop long before a
// corrupt archive fills the disk.
const maxEntry = 200 << 20

// writeZipEntry writes one entry of an archive to one path, executable.
func writeZipEntry(entry *zip.File, to string) error {
	reader, err := entry.Open()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755) //nolint:gosec // G302: the file is a program the gate runs.
	if err != nil {
		return err
	}
	if _, err := io.CopyN(out, reader, maxEntry); err != nil && !errors.Is(err, io.EOF) {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// e2eGate drives the WebUI's screens in a real headless browser: the walks of
// e2e/tests, written in Playwright and run by bun (d-3c9b). The
// dependency comes from e2e/bun.lock and the browser is Playwright's own pinned
// chromium; both live under e2e/.bin and e2e/node_modules, so nothing is
// installed into the person's home directory.
func (r *runner) e2eGate() error {
	bun, err := r.bun()
	if err != nil {
		return err
	}
	dir := filepath.Join(r.root, e2eDir)
	env := append(os.Environ(),
		"BUN_INSTALL_CACHE_DIR="+filepath.Join(dir, ".bin", "bun-cache"),
		"PLAYWRIGHT_BROWSERS_PATH="+filepath.Join(dir, ".bin", "browsers"),
	)
	if err := r.execIn(dir, env, bun, "install", "--frozen-lockfile"); err != nil {
		return err
	}
	// install is a check as much as a download: with the pinned browser already
	// in place it answers at once.
	if err := r.execIn(dir, env, bun, "x", "playwright", "install", "chromium"); err != nil {
		return err
	}
	return r.execIn(dir, env, bun, "x", "playwright", "test")
}
