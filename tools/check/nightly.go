package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// nightlyConfig is tools/check/nightly.yml: what the daily run measures and
// the thresholds it holds the code to.
type nightlyConfig struct {
	Mutation struct {
		// TimeoutCoefficient is passed to gremlins: a mutant's test run may take
		// this many times the unmutated run before it counts as timed out.
		TimeoutCoefficient int `yaml:"timeout_coefficient"`
		// Packages maps a package directory to the lowest test efficacy (%,
		// killed over killed+lived) it may have. Ratchet: up only.
		Packages map[string]float64 `yaml:"packages"`
	} `yaml:"mutation"`
	Fuzz struct {
		// Fuzztime is the -fuzztime of each target (a Go duration).
		Fuzztime string `yaml:"fuzztime"`
		Targets  []struct {
			Package string `yaml:"package"`
			Name    string `yaml:"name"`
		} `yaml:"targets"`
	} `yaml:"fuzz"`
}

const nightlyFile = "tools/check/nightly.yml"

// gremlinsReport is the part of gremlins' -o JSON the run reads.
type gremlinsReport struct {
	TestEfficacy float64 `json:"test_efficacy"`
	Killed       int     `json:"mutants_killed"`
	Lived        int     `json:"mutants_lived"`
	NotCovered   int     `json:"mutants_not_covered"`
}

type nightlyRow struct {
	kind, target, result string
	ok                   bool
}

func (r *runner) nightly() error {
	cfg, err := r.loadNightly()
	if err != nil {
		return err
	}
	var rows []nightlyRow
	rows = append(rows, r.mutationRows(cfg)...)
	rows = append(rows, r.fuzzRows(cfg)...)
	summary := nightlySummary(rows)
	fmt.Fprint(r.stdout, "\n"+summary)
	if err := appendJobSummary(summary); err != nil {
		return err
	}
	var failed []string
	for _, row := range rows {
		if !row.ok {
			failed = append(failed, row.kind+" "+row.target)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("nightly: %d of %d failed: %s", len(failed), len(rows), strings.Join(failed, ", "))
	}
	return nil
}

func (r *runner) loadNightly() (*nightlyConfig, error) {
	text, err := os.ReadFile(filepath.Join(r.root, nightlyFile))
	if err != nil {
		return nil, err
	}
	var cfg nightlyConfig
	if err := yaml.Unmarshal(text, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", nightlyFile, err)
	}
	if cfg.Mutation.TimeoutCoefficient <= 0 {
		return nil, fmt.Errorf("%s: mutation.timeout_coefficient must be positive", nightlyFile)
	}
	if cfg.Fuzz.Fuzztime == "" {
		return nil, fmt.Errorf("%s: fuzz.fuzztime is missing", nightlyFile)
	}
	return &cfg, nil
}

// mutationRows runs gremlins on each configured package and compares its
// test efficacy with the threshold.
func (r *runner) mutationRows(cfg *nightlyConfig) []nightlyRow {
	pkgs := make([]string, 0, len(cfg.Mutation.Packages))
	for pkg := range cfg.Mutation.Packages {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	gremlins, err := r.tool("gremlins")
	if err != nil {
		return []nightlyRow{{"mutation", "gremlins", err.Error(), false}}
	}
	var rows []nightlyRow
	for _, pkg := range pkgs {
		floor := cfg.Mutation.Packages[pkg]
		fmt.Fprintf(r.stdout, "\n== mutation %s (efficacy >= %.0f%%)\n", pkg, floor)
		rows = append(rows, r.mutationRow(gremlins, pkg, floor, cfg.Mutation.TimeoutCoefficient))
	}
	return rows
}

func (r *runner) mutationRow(gremlins, pkg string, floor float64, coefficient int) nightlyRow {
	report := filepath.Join(r.bin, "gremlins-"+strings.ReplaceAll(pkg, "/", "-")+".json")
	err := r.exec(gremlins, "unleash", "./"+pkg, "-o", report, "--timeout-coefficient", fmt.Sprint(coefficient))
	rep, readErr := readGremlins(report)
	if readErr != nil {
		if err != nil {
			return nightlyRow{"mutation", pkg, "gremlins failed: " + err.Error(), false}
		}
		return nightlyRow{"mutation", pkg, readErr.Error(), false}
	}
	result := fmt.Sprintf("efficacy %.1f%% (killed %d, lived %d, not covered %d), threshold %.0f%%",
		rep.TestEfficacy, rep.Killed, rep.Lived, rep.NotCovered, floor)
	return nightlyRow{"mutation", pkg, result, rep.TestEfficacy >= floor}
}

func readGremlins(path string) (*gremlinsReport, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no gremlins report: %w", err)
	}
	var rep gremlinsReport
	if err := json.Unmarshal(text, &rep); err != nil {
		return nil, fmt.Errorf("gremlins report %s: %w", path, err)
	}
	return &rep, nil
}

// fuzzRows runs each fuzz target for the configured time. A crash is written
// by go test under testdata/fuzz, and its input is in the output.
func (r *runner) fuzzRows(cfg *nightlyConfig) []nightlyRow {
	var rows []nightlyRow
	for _, t := range cfg.Fuzz.Targets {
		target := t.Package + " " + t.Name
		fmt.Fprintf(r.stdout, "\n== fuzz %s for %s\n", target, cfg.Fuzz.Fuzztime)
		err := r.exec("go", "test", "-run", "^$", "-fuzz", "^"+t.Name+"$", "-fuzztime", cfg.Fuzz.Fuzztime, "./"+t.Package)
		if err != nil {
			rows = append(rows, nightlyRow{"fuzz", target, "failed: " + err.Error(), false})
			continue
		}
		rows = append(rows, nightlyRow{"fuzz", target, "no crash in " + cfg.Fuzz.Fuzztime, true})
	}
	return rows
}

// nightlySummary is the Markdown table the Job Summary and the log show.
func nightlySummary(rows []nightlyRow) string {
	var b strings.Builder
	b.WriteString("## nightly: mutation testing and fuzzing\n\n")
	b.WriteString("| kind | target | result | |\n|---|---|---|---|\n")
	for _, row := range rows {
		mark := "ok"
		if !row.ok {
			mark = "FAILED"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", row.kind, row.target, row.result, mark)
	}
	return b.String()
}

// appendJobSummary adds the table to the GitHub Actions Job Summary when the
// run is one.
func appendJobSummary(summary string) error {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // G703: the path is GITHUB_STEP_SUMMARY, set by GitHub Actions
	if err != nil {
		return err
	}
	_, werr := f.WriteString(summary + "\n")
	return errors.Join(werr, f.Close())
}
