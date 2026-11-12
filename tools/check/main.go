// Command check runs hergy's quality gates. Local work ends with diff;
// CI runs every gate. Both use the same gate implementations.
//
//	go run ./tools/check             every gate, in order
//	go run ./tools/check lint test   only the named gates
//	go run ./tools/check diff        the gates a change can fail here, picked from what differs from the upstream
//	go run ./tools/check e2e tests/profiles.spec.ts   only those browser walks
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
	"time"
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
	{"webbuild", Hard, "internal/web/static's scripts are what internal/web/src makes, from the entries the templates load and the packages pinned in tools/webvendor (go run ./tools/check webbuild -write); CSS, fonts and the other files there are not the build's", (*runner).webbuildGate},
	{"names", Hard, "every name the WebUI's modules use is defined, imported or the browser's own, and every name they define or import is read (eslint no-undef and no-unused-vars from tools/webtools over the unit tests in internal/web/jstest; the screens' TypeScript is the types gate's)", (*runner).namesGate},
	{"types", Hard, "tsc --noEmit (strict, noUnusedLocals, noUnusedParameters) over internal/web/src with internal/web/tsconfig.json (typescript from tools/webtools)", (*runner).typesGate},
	{"apitypes", Hard, "internal/apicontract/api-types.d.ts is what openapi-typescript (tools/webtools) makes from internal/apicontract/openapi.yaml, the API's written promise (go run ./tools/check apitypes -write)", (*runner).apitypesGate},
	{"unit", Hard, "the WebUI's modules' own tests (internal/web/jstest/*.test.js) under bun test, with happy-dom's document and no browser (tools/webtools)", (*runner).unitGate},
	{"lint", Hard, "golangci-lint run (.golangci.yml); gocognit and cyclop are Ratchet, their thresholds move down only", (*runner).lintGate},
	{"arch", Hard, "the direction of imports (arch_test.go); lint restricts os/exec to herdrcli, gycli, browser and tools/check (test files excepted)", (*runner).archGate},
	{"test", Hard, "go test -race -count=1 ./... with a coverage profile; diff runs go test over the changed packages alone, with the cache and without -race or the profile", (*runner).testGate},
	{"reading", Hard, "no walk compares what the page said once: an expect( never takes an awaited read, a reading is read again until it matches (e2e/tests/*.spec.ts, r-291a)", (*runner).readingGate},
	{"owners", Hard, "the WebUI's TypeScript calls only the module owning each concept; Go resource components are private to ops (tools/check/owners.yml)", (*runner).ownersGate},
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
	args, write := splitWrite(args)
	r.write = write
	args, r.e2eSpecs = splitSpecs(args)
	if r.e2eSpecs != nil && (len(args) != 1 || args[0] != "e2e") {
		return errors.New("a spec path goes with the e2e gate alone (go run ./tools/check e2e tests/<name>.spec.ts)")
	}
	if len(args) == 1 && args[0] == "nightly" {
		return r.nightly()
	}
	if len(args) == 1 && args[0] == "diff" {
		return r.diff()
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

// splitSpecs takes the browser walks named on the command line (tests/<name>.spec.ts)
// out of the gate names: `go run ./tools/check e2e tests/profiles.spec.ts` runs that
// walk alone. No spec named keeps every walk (nil).
func splitSpecs(args []string) ([]string, []string) {
	var kept, specs []string
	for _, a := range args {
		if strings.HasSuffix(a, ".spec.ts") {
			specs = append(specs, a)
			continue
		}
		kept = append(kept, a)
	}
	return kept, specs
}

// -write is how the gates that write (webbuild, apitypes) are told to write: the rest of
// the gates only read, so a run is the same command in CI and on a laptop.
func splitWrite(args []string) ([]string, bool) {
	kept := make([]string, 0, len(args))
	write := false
	for _, a := range args {
		if a == "-write" || a == "--write" {
			write = true
			continue
		}
		kept = append(kept, a)
	}
	return kept, write
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
		started := time.Now()
		if err := g.run(r); err != nil {
			fmt.Printf("-- %s FAILED (%s): %v\n", g.name, time.Since(started).Round(time.Millisecond), err)
			failed = append(failed, g.name)
			if g.name == "test" {
				testFailed = true
			}
			continue
		}
		fmt.Printf("-- %s ok (%s)\n", g.name, time.Since(started).Round(time.Millisecond))
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
	b.WriteString("go run ./tools/check diff         the gates a change can fail here, picked from the files that differ from the upstream (tools/check/diff.go);\n")
	b.WriteString("                                 e2e, coverage and vuln are CI's: diff names the browser walks the change selects and the command to run them;\n")
	b.WriteString("                                 test runs over the changed packages with go test's cache; webbuild and apitypes write their files\n")
	b.WriteString("go run ./tools/check e2e tests/<name>.spec.ts ...   only those browser walks\n")
	b.WriteString("go run ./tools/check nightly      the daily run: gremlins mutation testing and go test -fuzz (tools/check/nightly.yml)\n")
	b.WriteString("go run ./tools/check webbuild -write  write internal/web/static's scripts again from internal/web/src\n")
	b.WriteString("go run ./tools/check apitypes -write  write internal/apicontract/api-types.d.ts again from openapi.yaml\n")
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
	if r.local {
		// The diff run: go test's cache, no -race and no profile (both CI's; see
		// diff.go), over the packages that changed.
		if len(r.testPackages) == 0 {
			fmt.Fprintln(r.stdout, "-- nothing a Go test reads changed")
			return nil
		}
		return r.exec("go", append([]string{"test"}, r.testPackages...)...)
	}
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
