package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// The diff run: the gates a change can fail, picked from the files that
// differ from what was pushed. Every gate still runs in CI on every push, so
// the diff run is what "done" means for one change, and CI is what catches the
// rest.

// changedPaths returns the paths, from the module root, that differ from the
// commit the branch shares with its upstream: commits not yet pushed, changes
// not yet committed, and untracked files. A branch with no upstream is
// compared with HEAD.
func (r *runner) changedPaths() ([]string, error) {
	base := "HEAD"
	if out, err := r.git("merge-base", "HEAD", "@{upstream}"); err == nil {
		base = strings.TrimSpace(out)
	}
	changed, err := r.git("diff", "--name-only", base)
	if err != nil {
		return nil, err
	}
	untracked, err := r.git("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return strings.Fields(changed + "\n" + untracked), nil
}

func (r *runner) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // G204: args are fixed by the caller
	cmd.Dir = r.root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// gatesFor returns, in the table's order, the gates a change to these paths
// can fail. test is among them whenever anything changed: in the diff run it
// keeps go test's cache, which reruns only the packages whose sources, or the
// files and environment their tests read, have changed (terms_test.go reads
// the documents, so a change to a README reruns it).
func gatesFor(paths []string) []gate {
	want := map[string]bool{}
	add := func(names ...string) {
		for _, n := range names {
			want[n] = true
		}
	}
	for _, p := range paths {
		add("test")
		switch {
		case p == "go.mod" || p == "go.sum":
			add("tidy", "lint", "deadcode", "e2e", "vuln")
		case strings.HasSuffix(p, "_test.go"), strings.HasPrefix(p, "tools/check/"):
			// Neither a test nor the gates' own code goes into what the walks run.
			add("fmt", "fix", "lint", "arch", "coverage", "deadcode")
		case strings.HasPrefix(p, "tools/webtools/"):
			// The modules' gates' tools: eslint and happy-dom.
			add("names", "unit")
		case strings.HasPrefix(p, "internal/web/jstest/"):
			// The modules' own tests.
			add("names", "unit")
		case strings.HasPrefix(p, "tools/webvendor/"):
			// The packages the WebUI's one module is made from.
			add("vendor", "deadcode")
		case strings.HasSuffix(p, ".go"):
			add("fmt", "tidy", "fix", "lint", "arch", "coverage", "deadcode", "e2e")
		case strings.HasPrefix(p, e2eDir+"/"):
			add("reading", "e2e")
		case filepath.ToSlash(p) == filepath.ToSlash(vendorFile):
			// The one module the screens import the packages through.
			add("vendor", "e2e")
		case strings.HasPrefix(p, "internal/"):
			// The WebUI's pages, scripts and styles are embedded into the binary.
			add("e2e")
			if strings.HasPrefix(p, "internal/web/static/") && strings.HasSuffix(p, ".js") {
				add("names", "unit")
			}
		case p == ".golangci.yml":
			add("fmt", "lint")
		case p == ".testcoverage.yml":
			add("coverage")
		}
	}
	var out []gate
	for _, g := range gates {
		if want[g.name] {
			out = append(out, g)
		}
	}
	return out
}

func (r *runner) diff() error {
	paths, err := r.changedPaths()
	if err != nil {
		return err
	}
	selected := gatesFor(paths)
	if len(selected) == 0 {
		fmt.Println("no change from the upstream: no gate to run")
		return nil
	}
	names := make([]string, len(selected))
	for i, g := range selected {
		names[i] = g.name
	}
	fmt.Printf("%d changed paths; the gates they can fail: %s\n", len(paths), strings.Join(names, " "))
	r.cachedTests = true
	return r.runGates(selected)
}
