// Command check runs hergy's quality gates: the same set on a laptop and in
// CI, so that whoever says "done" has run what CI will run.
//
//	go run ./tools/check             every gate, in order
//	go run ./tools/check lint test   only the named gates
//	go run ./tools/check nightly     the daily run: mutation testing and fuzzing
//	go run ./tools/check -h          the gates, each with its rule
//
// The tools it needs are pinned here and installed into .bin/ at the module
// root (ignored by git) when they are missing, so a machine with only Go on
// it can run the gates.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Rule says what a failing gate means for the person fixing it.
type Rule string

const (
	// Hard gates fail on any finding: the code is fixed, not the gate.
	Hard Rule = "Hard"
	// Ratchet gates have a threshold in a file, and the threshold moves one
	// way only: coverage may go up, complexity may go down.
	Ratchet Rule = "Ratchet"
)

type gate struct {
	name  string
	rule  Rule
	about string
	run   func(r *runner) error
}

// gates is the one table the run, the -h text and the CI steps come from.
var gates = []gate{
	{"fmt", Hard, "gofmt and goimports leave no diff (golangci-lint fmt --diff)", (*runner).fmtGate},
	{"tidy", Hard, "go mod tidy -diff is empty", (*runner).tidyGate},
	{"fix", Hard, "go fix -diff is empty", (*runner).fixGate},
	{"lint", Hard, "golangci-lint run (.golangci.yml); gocognit and cyclop are Ratchet, their thresholds move down only", (*runner).lintGate},
	{"arch", Hard, "the direction of imports (arch_test.go); os/exec outside internal/herdrcli is caught by lint (depguard)", (*runner).archGate},
	{"test", Hard, "go test -race -count=1 ./... with a coverage profile", (*runner).testGate},
	{"checks", Hard, "every checks/*.md item has a test of its size: a 小・中 test whose name holds the ID, a Playwright test whose name starts with a 大 ID, and no test for 監査; a browser walk under no 大 item fails (checks.go)", (*runner).checksGate},
	{"reading", Hard, "no walk compares what the page said once: an expect( never takes an awaited read, a reading is read again until it matches (e2e/tests/*.spec.ts, r-291a)", (*runner).readingGate},
	{"e2e", Hard, "the browser walks of e2e/tests (*.spec.ts) in Playwright, run by bun: e2e/bun.lock's dependency and Playwright's pinned chromium are fetched into .bin/ and e2e/.bin/ when missing (d-bb0d)", (*runner).e2eGate},
	{"coverage", Ratchet, "go-test-coverage against .testcoverage.yml; the total threshold moves up only", (*runner).coverageGate},
	{"deadcode", Hard, "deadcode -test ./... reports nothing", (*runner).deadcodeGate},
	{"vuln", Hard, "govulncheck ./... finds no vulnerability", (*runner).vulnGate},
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Print(usage())
		return nil
	}
	r, err := newRunner()
	if err != nil {
		return err
	}
	if len(args) == 1 && args[0] == "nightly" {
		return r.nightly()
	}
	selected, err := selectGates(args)
	if err != nil {
		return err
	}
	return r.runGates(selected)
}

func selectGates(names []string) ([]gate, error) {
	if len(names) == 0 {
		return gates, nil
	}
	var out []gate
	for _, name := range names {
		g, ok := gateNamed(name)
		if !ok {
			return nil, fmt.Errorf("no gate named %q (see go run ./tools/check -h)", name)
		}
		out = append(out, g)
	}
	return out, nil
}

func gateNamed(name string) (gate, bool) {
	for _, g := range gates {
		if g.name == name {
			return g, true
		}
	}
	return gate{}, false
}

// runGates runs every selected gate, even after one fails, so that one run
// shows everything there is to fix. coverage is skipped when test failed:
// its profile would be incomplete.
func (r *runner) runGates(selected []gate) error {
	var failed []string
	testFailed := false
	for _, g := range selected {
		fmt.Printf("\n== %s (%s) %s\n", g.name, g.rule, g.about)
		if g.name == "coverage" && testFailed {
			fmt.Println("-- skipped: test failed")
			failed = append(failed, g.name)
			continue
		}
		if err := g.run(r); err != nil {
			fmt.Printf("-- %s FAILED: %v\n", g.name, err)
			failed = append(failed, g.name)
			if g.name == "test" {
				testFailed = true
			}
			continue
		}
		fmt.Printf("-- %s ok\n", g.name)
	}
	fmt.Println()
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d gates failed: %s", len(failed), len(selected), strings.Join(failed, " "))
	}
	fmt.Printf("all %d gates passed\n", len(selected))
	return nil
}

func usage() string {
	var b strings.Builder
	b.WriteString("go run ./tools/check [gate ...]   run every gate, or the named ones, in this order\n")
	b.WriteString("go run ./tools/check nightly      the daily run: gremlins mutation testing and go test -fuzz (tools/check/nightly.yml)\n")
	b.WriteString("\nGates:\n")
	for _, g := range gates {
		fmt.Fprintf(&b, "  %-9s %-8s %s\n", g.name, g.rule, g.about)
	}
	fmt.Fprintf(&b, "\n%s: fails on any finding; fix the code, not the gate.\n", Hard)
	fmt.Fprintf(&b, "%s: the threshold is in the named file and moves one way only.\n", Ratchet)
	b.WriteString("\nTools, installed into .bin/ when missing:\n")
	for _, t := range toolList() {
		fmt.Fprintf(&b, "  %-17s %s@%s\n", t.name, t.pkg, t.version)
	}
	return b.String()
}

// Gates.

func (r *runner) fmtGate() error {
	lint, err := r.tool("golangci-lint")
	if err != nil {
		return err
	}
	return r.exec(lint, "fmt", "--diff", "./...")
}

func (r *runner) tidyGate() error {
	return r.exec("go", "mod", "tidy", "-diff")
}

func (r *runner) fixGate() error {
	return r.exec("go", "fix", "-diff", "./...")
}

func (r *runner) lintGate() error {
	lint, err := r.tool("golangci-lint")
	if err != nil {
		return err
	}
	return r.exec(lint, "run", "./...")
}

func (r *runner) archGate() error {
	return r.exec("go", "test", "-count=1", "-run", "^TestImportDirection$", ".")
}

func (r *runner) testGate() error {
	return r.exec("go", "test", "-race", "-count=1", "-coverprofile="+r.coverProfile(), "./...")
}

func (r *runner) coverageGate() error {
	cov, err := r.tool("go-test-coverage")
	if err != nil {
		return err
	}
	return r.exec(cov, "--config", ".testcoverage.yml")
}

func (r *runner) deadcodeGate() error {
	dead, err := r.tool("deadcode")
	if err != nil {
		return err
	}
	out, err := r.output(dead, "-test", "./...")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		fmt.Print(out)
		return errors.New("deadcode found unreachable code")
	}
	return nil
}

func (r *runner) vulnGate() error {
	vuln, err := r.tool("govulncheck")
	if err != nil {
		return err
	}
	return r.exec(vuln, "./...")
}

func (r *runner) coverProfile() string {
	return filepath.ToSlash(filepath.Join(".bin", "cover.out"))
}
