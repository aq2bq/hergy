package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// The diff run: the gates a change can fail, picked from the files that
// differ from what was pushed, minus the gates that are CI's alone. Every gate
// still runs in CI on every push, so the diff run is what "done" means for one
// change on a laptop, and CI is what catches the rest.
//
// What stays out of the diff run, and why (measured 2026-11-06 on an 8-core
// laptop, every gate alone on a clean main):
//
//   - e2e: 148s for the whole suite, and a change under internal/web/src/lib or
//     to any Go file the screens use selects the whole suite. The browser walks
//     were 2.5 hours of one developer's 29-hour task, run 116 times. CI runs
//     them on every push; a laptop runs them on demand (go run ./tools/check e2e
//     tests/<spec>...), and diff prints which walks the change selects.
//   - coverage, vuln: a ratchet over the whole profile, and a network call.
//   - test's -race and -coverprofile: internal/cli's 259 serial tests take 75s
//     under -race and 54s without; both flags are CI's. The diff run tests only
//     the packages that changed, with go test's cache; the packages that depend
//     on them (internal/cli depends on nearly everything) are CI's.

// changedPaths returns the paths, from the module root, that differ from the
// commit the branch shares with its upstream: commits not yet pushed, changes
// not yet committed, and untracked files. A branch with no upstream is
// compared with HEAD.
func (r *runner) changedPaths() ([]string, error) {
	base := ""
	// A branch with no upstream has nothing pushed: compare with origin/main (or
	// main) so a committed change is still selected. Without this, a branch
	// that was never pushed answered "no change from the upstream" after its
	// first commit and every gate was skipped.
	for _, against := range []string{"@{upstream}", "origin/main", "main"} {
		if out, err := r.git("merge-base", "HEAD", against); err == nil {
			base = strings.TrimSpace(out)
			break
		}
	}
	if base == "" {
		base = "HEAD"
	}
	changed, err := r.git("diff", "--no-renames", "--name-only", base)
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
	want := map[string]bool{"words": slices.ContainsFunc(paths, wordsPath)}
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
		case strings.HasSuffix(p, "_test.go"):
			// A test goes into no walk; the Go side's checks still hold it.
			add("fmt", "fix", "lint", "arch", "coverage", "deadcode")
		case strings.HasPrefix(p, "tools/check/"):
			// The gates' own code goes into what the walks run, and holds the
			// owners' table.
			add("fmt", "fix", "lint", "arch", "coverage", "deadcode", "owners")
		case strings.HasPrefix(p, "tools/webtools/"):
			// The modules' gates' tools: eslint, typescript, happy-dom and
			// openapi-typescript.
			add("names", "types", "unit", "apitypes")
		case strings.HasPrefix(p, "internal/apicontract/") && !strings.HasSuffix(p, ".go"):
			// The API's promise and the types made from it; the tests that call the
			// API are held to it.
			add("apitypes", "e2e")
		case strings.HasPrefix(p, "internal/web/jstest/"):
			// The modules' own tests.
			add("names", "unit")
		case strings.HasPrefix(p, "tools/webvendor/"):
			// The packages the WebUI is built from, and the build itself.
			add("webbuild", "deadcode")
		case strings.HasSuffix(p, ".go"):
			// The Go side holds its half of the owners' table too (the names the
			// list answers ask internal/catalog for).
			add("fmt", "tidy", "fix", "lint", "arch", "coverage", "deadcode", "owners")
			if !cliOnly(p) {
				add("e2e")
			}
		case strings.HasPrefix(p, e2eDir+"/"):
			add("reading", "e2e")
		case strings.HasPrefix(p, "internal/web/src/"):
			// The WebUI's sources: the modules the screens are written in. A change
			// here is built into static/ (webbuild), checked (types, unit) and walked
			// (e2e).
			add("types", "unit", "webbuild", "e2e", "owners")
		case strings.HasPrefix(p, "internal/web/templates/"):
			// The templates name the build's entries; the pages are embedded too.
			add("webbuild", "e2e")
		case strings.HasPrefix(p, "internal/web/static/"):
			// The styles, the fonts and the built scripts are embedded into the
			// binary. A built script is the build's to write, so a change there is
			// held to the build.
			add("e2e")
			if strings.HasSuffix(p, ".js") {
				add("webbuild")
			}
		case strings.HasPrefix(p, "internal/web/tsconfig.json"):
			add("types", "webbuild", "unit", "e2e")
		case strings.HasPrefix(p, "internal/"):
			// The WebUI's pages, scripts and styles are embedded into the binary,
			// and the Go side holds its half of the owners' table.
			add("e2e", "owners")
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

// wordsPath covers the catalog, its renderers and generated outputs. Changes to
// screen text also check the vocabulary artifact before a future TermID migration.
func wordsPath(p string) bool {
	for _, prefix := range []string{"internal/words/", "internal/options/", "internal/cheat/", "internal/i18n/", "internal/ops/", "internal/web/src/", "tools/check/"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return p == "README.md" || p == "README.ja.md" || p == "skills/hergy/SKILL.md"
}

func (r *runner) diff() error {
	paths, err := r.changedPaths()
	if err != nil {
		return err
	}
	selected, later := localGates(gatesFor(paths))
	if len(selected) == 0 && len(later) == 0 {
		fmt.Println("no change from the upstream: no gate to run")
		return nil
	}
	fmt.Printf("%d changed paths; the gates they can fail here: %s\n", len(paths), gateNames(selected))
	if len(later) > 0 {
		fmt.Printf("left to CI (every push runs them): %s\n", gateNames(later))
	}
	r.local = true
	r.write = true // webbuild and apitypes write their files here; CI compares
	r.testPackages, err = existingTestPackages(r.root, testPackagesFor(paths))
	if err != nil {
		return err
	}
	specs, err := filepath.Glob(filepath.Join(r.root, browserGlob))
	if err != nil {
		return err
	}
	if slices.ContainsFunc(later, func(g gate) bool { return g.name == "e2e" }) {
		walks := specsFor(paths, specs)
		if walks == nil {
			fmt.Println("the browser walks this change selects: every spec (go run ./tools/check e2e)")
		} else {
			fmt.Printf("the browser walks this change selects: go run ./tools/check e2e %s\n", strings.Join(walks, " "))
		}
	}
	if len(selected) == 0 {
		return nil
	}
	return r.runGates(selected)
}

// ciOnly are the gates the diff run leaves to CI: the ones whose cost is the
// whole repository's, not the change's (see the top of this file).
var ciOnly = map[string]bool{"e2e": true, "coverage": true, "vuln": true}

// localGates splits the gates a change can fail into the ones the diff run runs
// here and the ones it leaves to CI, both in the table's order.
func localGates(selected []gate) (here, later []gate) {
	for _, g := range selected {
		if ciOnly[g.name] {
			later = append(later, g)
		} else {
			here = append(here, g)
		}
	}
	return here, later
}

func gateNames(gs []gate) string {
	names := make([]string, len(gs))
	for i, g := range gs {
		names[i] = g.name
	}
	return strings.Join(names, " ")
}

// testPackagesFor is what the diff run's test gate tests: the packages whose Go
// files changed, by their directory, and for a file that is not Go, the package
// that embeds or reads it (the locales, the templates, the API's promise, the
// documents terms_test.go holds to the glossary). A file nothing here names
// keeps ./..., which go test's cache makes cheap when it holds. The packages
// that import a changed one (internal/cli imports nearly everything, and its
// 259 tests take a minute) are CI's. existingTestPackages then removes the
// deleted side of package moves before go test runs.
func testPackagesFor(paths []string) []string {
	seen := map[string]bool{}
	var pkgs []string
	add := func(pkg string) {
		if !seen[pkg] {
			seen[pkg] = true
			pkgs = append(pkgs, pkg)
		}
	}
	for _, p := range paths {
		switch {
		case strings.HasSuffix(p, ".go"):
			add("./" + filepath.ToSlash(filepath.Dir(p)))
		case strings.HasPrefix(p, "internal/i18n/locales/"):
			add("./internal/i18n")
			add("./.") // terms_test.go reads the locales too
		case strings.HasPrefix(p, "internal/web/templates/") || strings.HasPrefix(p, "internal/web/static/"):
			add("./internal/web")
		case strings.HasPrefix(p, "internal/web/src/"):
			add("./.") // terms_test.go reads the screens' words
		case strings.HasPrefix(p, "internal/apicontract/"):
			add("./internal/apicontract")
		case strings.HasPrefix(p, "tools/check/"):
			add("./tools/check")
		case strings.HasSuffix(p, ".md") || strings.HasPrefix(p, "skills/") || strings.HasPrefix(p, "internal/cheat/"):
			add("./.") // the documents terms_test.go holds to the glossary
		case p == ".gitignore" || p == ".golangci.yml" || p == ".testcoverage.yml" || strings.HasPrefix(p, ".github/") || strings.HasPrefix(p, "e2e/") || strings.HasPrefix(p, "tools/webtools/") || strings.HasPrefix(p, "tools/webvendor/"):
			// Nothing a Go test reads.
		default:
			return []string{"./..."}
		}
	}
	slices.Sort(pkgs)
	return pkgs
}

// Only these CLI presentation files are known not to run in the browser
// fixtures (which also invoke hergy new). Unknown Go files keep all walks.
func cliOnly(path string) bool {
	return strings.HasPrefix(path, "internal/cheat/") || path == "internal/options/help.go" || path == "internal/cli/apply_cli.go"
}

// specsFor narrows only screen-owned changes. A nil selection means all specs,
// including new/unknown specs. Cross-screen walks always stay in the selection.
func specsFor(paths, specs []string) []string {
	want := map[string]bool{}
	for _, path := range paths {
		if !slices.ContainsFunc(gatesFor([]string{path}), func(g gate) bool { return g.name == "e2e" }) {
			continue
		}
		if strings.HasPrefix(path, "e2e/tests/") && strings.HasSuffix(path, ".spec.ts") {
			name := filepath.Base(path)
			if !slices.ContainsFunc(specs, func(s string) bool { return filepath.Base(s) == name }) {
				return nil // A deleted or renamed spec needs the remaining suite.
			}
			want[name] = true
			continue
		}
		screen := changedScreen(path)
		if screen == "" {
			return nil
		}
		for _, spec := range specs {
			name := filepath.Base(spec)
			if owner := specScreen(name); owner == "" || owner == screen {
				want[name] = true
			}
		}
	}
	var selected []string
	for name := range want {
		selected = append(selected, "tests/"+name)
	}
	slices.Sort(selected)
	return selected
}

func changedScreen(path string) string {
	for _, screen := range []string{"profiles", "roles", "teams", "activity"} {
		if strings.HasPrefix(path, "internal/web/src/"+screen+"/") ||
			strings.HasPrefix(path, "internal/web/static/"+screen+"/") {
			return screen
		}
	}
	return ""
}

func specScreen(name string) string {
	switch name {
	case "profiles.spec.ts":
		return "profiles"
	case "roles.spec.ts", "roles-rule-id.spec.ts":
		return "roles"
	case "teams.spec.ts", "teams-shape-size.spec.ts":
		return "teams"
	case "activity.spec.ts", "activity-actions.spec.ts":
		return "activity"
	default:
		return "" // Includes navigation, copy, save, conflict and unknown walks.
	}
}

// existingTestPackages omits a package deleted by a move, while preserving
// wildcard selections and refusing filesystem errors other than absence.
func existingTestPackages(root string, packages []string) ([]string, error) {
	var out []string
	for _, pkg := range packages {
		if strings.Contains(pkg, "...") {
			out = append(out, pkg)
			continue
		}
		entries, err := os.ReadDir(filepath.Join(root, pkg))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(entries, func(entry os.DirEntry) bool { return !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") }) {
			out = append(out, pkg)
		}
	}
	return out, nil
}
