package main

// The owners gate holds one concept to one owner (r-ea17, ac-f62f, d-4d88).
//
// Concept ② "一覧の行と件数" has two owners:
//
//   - the server's internal/catalog is the only place that decides which rows a
//     list draws and what the band's numbers are; a list answer and the band ask
//     it (ListedRows / ShownRows / Counts) and never count rows themselves.
//   - the WebUI's lib/band-counts.ts is the only module that writes the band's
//     numbers (setCount / setActivity); a screen hands in what a reading said.
//
// The gate reads the source, not the behaviour: a name that writes the band or
// asks the owner for rows may stand only in the files the rule below names. A
// file that needs the name and is not there is either moved to the owner or
// added to tools/check/owners.yml's allowlist, with why and the decision that
// allowed it (gy's d-…).
//
// It starts as a report and becomes a gate: with enforce: false the findings are
// printed and the gate passes, so the copies can be merged before the gate
// fails. done is the gate failing, which is what enforce: true makes it.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ownersPath is the gate's own settings, from the module root.
const ownersPath = "tools/check/owners.yml"

// ownerRule is one name and the files that may hold it: the owner first, then
// the places that ask it.
type ownerRule struct {
	name  string
	where string // "go" or "ts", which files the name is looked for in
	files []string
}

// ownerRules is the table the gate reads. The lists are paths or directory
// prefixes from the module root.
var ownerRules = []ownerRule{
	// The band's numbers are written by band-counts.ts, through the DOM
	// primitives nav.ts holds, and nowhere else.
	{name: "setCount", where: "ts", files: []string{"internal/web/src/lib/nav.ts", "internal/web/src/lib/band-counts.ts"}},
	{name: "setActivity", where: "ts", files: []string{"internal/web/src/lib/nav.ts", "internal/web/src/lib/band-counts.ts"}},
	{name: "applyCounts", where: "ts", files: []string{"internal/web/src/lib/band-counts.ts", "internal/web/src/lib/hq/state.ts"}},
	{name: "applyActivity", where: "ts", files: []string{"internal/web/src/lib/band-counts.ts", "internal/web/src/lib/hq/state.ts"}},
	// The rows and the counts are decided in internal/catalog; the lists and the
	// band's first value ask it. The row rules of r-929e are gone: a name that
	// decided rows anywhere else fails here.
	{name: "Counts", where: "go", files: []string{"internal/catalog/", "internal/app/hq.go", "internal/web/nav.go"}},
	{name: "ListedRows", where: "go", files: []string{"internal/catalog/", "internal/web/"}},
	{name: "ShownRows", where: "go", files: []string{"internal/catalog/", "internal/web/"}},
	{name: "ReadableOf", where: "go", files: []string{"internal/catalog/", "internal/web/"}},
	{name: "ListedRoles", where: "go", files: []string{"internal/catalog/"}},
	{name: "navRoles", where: "go", files: []string{"internal/catalog/"}},
}

// ownerAllow is one exception: a name in a file, with why and the decision that
// allowed it.
type ownerAllow struct {
	Path     string `yaml:"path"`
	Name     string `yaml:"name"`
	Why      string `yaml:"why"`
	Decision string `yaml:"decision"`
}

// ownersFile is tools/check/owners.yml.
type ownersFile struct {
	Enforce bool         `yaml:"enforce"`
	Allow   []ownerAllow `yaml:"allow"`
}

// ownersGate reports (and, once enforced, refuses) a name outside its owner.
func (r *runner) ownersGate() error {
	conf, err := r.ownersConf()
	if err != nil {
		return err
	}
	for _, one := range conf.Allow {
		if one.Path == "" || one.Name == "" || one.Why == "" || one.Decision == "" {
			return fmt.Errorf("%s: 許可リストの %q（%s）に、道・名前・理由・決定（d-…）のすべてを書いてください", ownersPath, one.Name, one.Path)
		}
		if !strings.HasPrefix(one.Decision, "d-") {
			return fmt.Errorf("%s: 許可リストの %q の決定は d-… で書いてください: %q", ownersPath, one.Name, one.Decision)
		}
	}
	findings, err := r.ownerFindings(conf)
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		return nil
	}
	for _, finding := range findings {
		fmt.Println(finding)
	}
	if !conf.Enforce {
		fmt.Printf("owners: %d 件（報告のみ。tools/check/owners.yml の enforce を true にする前に、持ち主へ寄せるか許可リストに理由と決定を添えてください）\n", len(findings))
		return nil
	}
	return fmt.Errorf("持ち主の外で帯の件数を書く・一覧の行を決める場所が %d 件", len(findings))
}

func (r *runner) ownersConf() (ownersFile, error) {
	var out ownersFile
	raw, err := os.ReadFile(filepath.Join(r.root, ownersPath))
	if err != nil {
		return out, err
	}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("%s: %w", ownersPath, err)
	}
	return out, nil
}

// ownerFindings is one line for every place a watched name stands outside its
// owner: the file and line, the name, and where it belongs.
func (r *runner) ownerFindings(conf ownersFile) ([]string, error) {
	files, err := r.sourceFiles()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, path := range files {
		rel, err := filepath.Rel(r.root, path)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		// The gate reads code: comments and the bodies of strings say the names
		// without calling them, and the mask keeps the places (reading.go's own
		// reading of a spec).
		code := maskSpec(string(raw))
		goFile := strings.HasSuffix(rel, ".go")
		for _, rule := range ownerRules {
			if rule.where == "go" != goFile {
				continue
			}
			if goFile && strings.HasSuffix(rel, "_test.go") {
				continue
			}
			if allowed(rel, rule.files) || allowedBy(rel, conf.Allow, rule.name) {
				continue
			}
			for _, line := range callLines(code, rule.name) {
				out = append(out, fmt.Sprintf("%s:%d: %s はここではなく持ち主（%s）に置いてください。寄せられない場合は %s の allow に理由と決定を添えてください",
					rel, line, rule.name, strings.Join(rule.files, "、"), ownersPath))
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// sourceFiles is the code the gate reads: the server's Go and the WebUI's
// TypeScript, from the module root.
func (r *runner) sourceFiles() ([]string, error) {
	var out []string
	err := filepath.Walk(filepath.Join(r.root, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".go"), strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"):
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// allowed is whether a path stands in one list of paths or directory prefixes.
func allowed(path string, files []string) bool {
	for _, one := range files {
		if path == one || strings.HasSuffix(one, "/") && strings.HasPrefix(path, one) {
			return true
		}
	}
	return false
}

// allowedBy is whether one allowlist entry covers this name and file.
func allowedBy(path string, entries []ownerAllow, name string) bool {
	for _, one := range entries {
		if one.Path == path && one.Name == name {
			return true
		}
	}
	return false
}

// callLines is the 1-based lines where a name is called or defined: the name at
// a word boundary, followed by ( or =, so a longer name is not it.
func callLines(code, name string) []int {
	var out []int
	for i := 0; i < len(code); {
		j := strings.Index(code[i:], name)
		if j < 0 {
			break
		}
		at := i + j
		before := byte(' ')
		if at > 0 {
			before = code[at-1]
		}
		after := byte(' ')
		end := at + len(name)
		if end < len(code) {
			after = code[end]
		}
		if !isIdentByte(before) && (after == '(' || after == '=') {
			out = append(out, lineOf(code, at))
		}
		i = at + len(name)
	}
	return out
}
