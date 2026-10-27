package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// The tools the gates run, pinned to one version each. They are installed
// with go install into .bin/ under the module root, never into the user's
// GOPATH, and called by that path so that another version on PATH is never
// picked up.
type tool struct {
	name    string // the binary's name
	pkg     string // the package go install takes
	version string
}

var tools = map[string]tool{
	"golangci-lint":    {"golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", "v2.14.0"},
	"deadcode":         {"deadcode", "golang.org/x/tools/cmd/deadcode", "v0.52.0"},
	"govulncheck":      {"govulncheck", "golang.org/x/vuln/cmd/govulncheck", "v1.9.0"},
	"go-test-coverage": {"go-test-coverage", "github.com/vladopajic/go-test-coverage/v2", "v2.19.0"},
	"gremlins":         {"gremlins", "github.com/go-gremlins/gremlins/cmd/gremlins", "v0.6.0"},
}

func toolList() []tool {
	out := make([]tool, 0, len(tools))
	for _, t := range tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

type runner struct {
	root   string // the module root, where go.mod is
	bin    string // root/.bin
	stdout io.Writer
	// cachedTests keeps go test's cache in the test gate (the diff run).
	cachedTests bool
}

func newRunner() (*runner, error) {
	root, err := moduleRoot()
	if err != nil {
		return nil, err
	}
	bin := filepath.Join(root, ".bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return nil, err
	}
	return &runner{root: root, bin: bin, stdout: os.Stdout}, nil
}

// moduleRoot walks up from the working directory to the nearest go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the working directory")
		}
		dir = parent
	}
}

// tool returns the path of an installed tool, installing it first when the
// binary or its version stamp is missing or holds another version. The stamp
// also holds the Go that built the tool: a tool that reads Go source (lint,
// deadcode, govulncheck) cannot read the standard library of a newer Go.
func (r *runner) tool(name string) (string, error) {
	t, ok := tools[name]
	if !ok {
		return "", fmt.Errorf("no tool named %q", name)
	}
	path := filepath.Join(r.bin, t.name)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	stamp := filepath.Join(r.bin, t.name+".version")
	want := t.version + " " + runtime.Version()
	if have, err := os.ReadFile(stamp); err == nil && strings.TrimSpace(string(have)) == want {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	fmt.Fprintf(r.stdout, "-- installing %s@%s into %s\n", t.pkg, t.version, r.bin)
	cmd := r.command("go", "install", t.pkg+"@"+t.version)
	cmd.Env = append(os.Environ(), "GOBIN="+r.bin)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go install %s@%s: %w", t.pkg, t.version, err)
	}
	if err := os.WriteFile(stamp, []byte(want+"\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (r *runner) command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...) //nolint:gosec // G204: name is go or a tool from the pinned table, args are fixed by the gate
	cmd.Dir = r.root
	cmd.Stdout = r.stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// exec runs a command at the module root with its output passed through.
func (r *runner) exec(name string, args ...string) error {
	fmt.Fprintf(r.stdout, "$ %s %s\n", filepath.Base(name), strings.Join(args, " "))
	return r.command(name, args...).Run()
}

// execIn runs a command in one directory with one environment, its output passed
// through: the browser walks' tools stand under e2e/ and read their own cache
// places from the environment.
func (r *runner) execIn(dir string, env []string, name string, args ...string) error {
	fmt.Fprintf(r.stdout, "$ %s %s\n", filepath.Base(name), strings.Join(args, " "))
	cmd := exec.Command(name, args...) //nolint:gosec // G204: name is a tool from the pinned table, args are fixed by the gate
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = r.stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// output runs a command at the module root and returns its stdout.
func (r *runner) output(name string, args ...string) (string, error) {
	fmt.Fprintf(r.stdout, "$ %s %s\n", filepath.Base(name), strings.Join(args, " "))
	var out bytes.Buffer
	cmd := r.command(name, args...)
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}
