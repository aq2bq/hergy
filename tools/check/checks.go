package main

// The checks gate is TESTING.md's「一覧とテスト」を守らせる: every 小・中・大 item
// of checks/<画面>.md has a test of its size, every browser walk stands under a 大
// item, and a 監査 item asks for no test at all.
//
// The lists are the designers': one row an item, with an ID and the size the item
// is tested at. The size says where the test stands:
//
//   - 小・中 — a Test* function or a t.Run of a *_test.go, whose name holds the ID
//     (a function name writes the ID's - as _).
//   - 大 — a test of e2e/tests/*.spec.ts, whose name starts with the ID: the
//     browser walks, written in Playwright and run by bun (d-3c9b).
//   - 監査 — no test: the designer looks at it.
//
// The three things the gate reports are the items nobody tests yet, the browser
// walks that belong to no 大 item (TESTING.md: an ID-less 大 test is dropped),
// and an ID two lists both write.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// The sizes a checks/<画面>.md row writes in its 大きさ column.
const (
	sizeSmall  = "小"
	sizeMedium = "中"
	sizeLarge  = "大"
	sizeAudit  = "監査"
	// The lists, and the two places a test may stand: the 小・中 tests of every
	// package (TESTING.md writes them as 各パッケージの *_test.go), and the browser
	// walks of e2e/tests (Playwright's spec files, run by bun).
	checksGlob    = "checks/*.md"
	browserGlob   = "e2e/tests/*.spec.ts"
	testSuffix    = "_test.go"
	browserSuffix = ".spec.ts"
)

// checkItem is one row of a checks/<画面>.md table.
type checkItem struct {
	ID    string
	Size  string
	Where string // checks/activity.md
}

// e2eSubtest is one browser walk: the name it is given, and where it stands.
type e2eSubtest struct {
	Name  string // NAV-N6 the changes one screen carries
	Where string // e2e/tests/nav.spec.ts:20
}

// checksGate holds the lists and the tests to each other.
func (r *runner) checksGate() error {
	items, err := readChecks(r.root)
	if err != nil {
		return err
	}
	subtests, err := readBrowserTests(r.root)
	if err != nil {
		return err
	}
	names, err := readTestNames(r.root)
	if err != nil {
		return err
	}
	findings := checkFindings(items, subtests, names)
	if len(findings) == 0 {
		return nil
	}
	for _, finding := range findings {
		fmt.Println(finding)
	}
	return errors.New(checksSummary(findings))
}

// readChecks reads every checks/<画面>.md and returns its rows in the order the
// files and their tables stand in. The columns are found by their headings, so
// a list may put its ID, its size and what it checks in any order.
func readChecks(root string) ([]checkItem, error) {
	paths, err := filepath.Glob(filepath.Join(root, checksGlob))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []checkItem
	for _, path := range paths {
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		where := filepath.ToSlash(filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path)))
		items, err := readChecksTable(string(text), where)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no items in %s: the lists are missing", checksGlob)
	}
	return out, nil
}

// readChecksTable reads one list's table: the heading row that names its ID and
// 大きさ columns, and every row under it.
func readChecksTable(text, where string) ([]checkItem, error) {
	idCol, sizeCol := -1, -1
	var out []checkItem
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			if idCol >= 0 && len(out) > 0 {
				break // the table ended
			}
			continue
		}
		cells := tableCells(trimmed)
		if idCol < 0 {
			for i, cell := range cells {
				switch cell {
				case "ID":
					idCol = i
				case "大きさ":
					sizeCol = i
				}
			}
			if idCol < 0 || sizeCol < 0 {
				return nil, fmt.Errorf("%s: the table's heading names no ID or 大きさ column", where)
			}
			continue
		}
		if isTableRule(cells) || len(cells) <= idCol || len(cells) <= sizeCol {
			continue
		}
		id, size := cells[idCol], cells[sizeCol]
		if id == "" {
			continue
		}
		if size != sizeSmall && size != sizeMedium && size != sizeLarge && size != sizeAudit {
			return nil, fmt.Errorf("%s: %s stands at the size %q, want %s, %s, %s or %s", where, id, size, sizeSmall, sizeMedium, sizeLarge, sizeAudit)
		}
		out = append(out, checkItem{ID: id, Size: size, Where: where})
	}
	return out, nil
}

// tableCells splits one markdown table row into its cells.
func tableCells(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// isTableRule reports whether a row is the heading's rule (|---|---|).
func isTableRule(cells []string) bool {
	for _, cell := range cells {
		if cell == "" || strings.Trim(cell, "-:") != "" {
			return false
		}
	}
	return true
}

// playwrightCall is a browser test's own call: test('name'), or the same with one
// modifier (test.only, test.skip, test.fixme, test.slow). A describe is a group
// of tests, not a test, so it is not read here.
var playwrightCall = regexp.MustCompile(`^\s*test(?:\.(?:only|skip|fixme|slow))?\(\s*`)

// readBrowserTests reads the name of every test of e2e/tests/*.spec.ts, in the
// order the files and their tests stand in.
func readBrowserTests(root string) ([]e2eSubtest, error) {
	paths, err := filepath.Glob(filepath.Join(root, browserGlob))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []e2eSubtest
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		where := filepath.ToSlash(rel)
		for i, line := range strings.Split(string(raw), "\n") {
			at := playwrightCall.FindStringIndex(line)
			if at == nil {
				continue
			}
			name, ok := leadingString(line[at[1]:])
			if !ok {
				return nil, fmt.Errorf("%s:%d: test( has no name to read", where, i+1)
			}
			out = append(out, e2eSubtest{Name: name, Where: fmt.Sprintf("%s:%d", where, i+1)})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no test in %s", browserGlob)
	}
	return out, nil
}

// leadingString reads the string literal a test's name is written as: single,
// double or back quotes. A backslash keeps the character after it as it stands,
// so a name with a quote in it reads back the same way.
func leadingString(s string) (string, bool) {
	text := strings.TrimLeft(s, " \t")
	if text == "" {
		return "", false
	}
	quote := text[0]
	if quote != '\'' && quote != '"' && quote != '`' {
		return "", false
	}
	var out strings.Builder
	for i := 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			if i+1 < len(text) {
				out.WriteByte(text[i])
				i++
				out.WriteByte(text[i])
			}
		case quote:
			return out.String(), true
		default:
			out.WriteByte(text[i])
		}
	}
	return "", false
}

// isTRun reports whether one call is t.Run(name, f) — a call of a Run method on
// something called t.
func isTRun(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Run" || len(call.Args) < 2 {
		return false
	}
	t, ok := sel.X.(*ast.Ident)
	return ok && t.Name == "t"
}

// readTestNames reads every name a 小・中 test may carry the ID in: every top
// level Test* function and every t.Run name of one *_test.go of the module, at
// any depth of its tree (TESTING.md: 各パッケージの *_test.go). The e2e files are
// left out — a 小・中 item stands at its own size, not in the browser's.
func readTestNames(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".bin", "testdata", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		base := d.Name()
		if !strings.HasSuffix(base, testSuffix) || strings.HasPrefix(base, "e2e") {
			return nil
		}
		names, err := readOneTestFile(path)
		if err != nil {
			return err
		}
		out = append(out, names...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no Test* function in any %s", testSuffix)
	}
	return out, nil
}

// readOneTestFile reads one test file's names: its top level Test* functions and
// every t.Run name it holds.
func readOneTestFile(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
			out = append(out, fn.Name.Name)
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isTRun(call) {
			return true
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if name, err := strconv.Unquote(lit.Value); err == nil {
				out = append(out, name)
			}
		}
		return true
	})
	return out, nil
}

// checkFindings holds the lists and the tests to each other and returns what
// does not meet: one line an item nobody tests, one line an e2e subtest that
// belongs to no 大 item, one line an ID two lists both write.
func checkFindings(items []checkItem, subtests []e2eSubtest, testNames []string) []string {
	var findings []string
	seen := map[string]string{}
	for _, item := range items {
		norm := strings.ReplaceAll(item.ID, "-", "_")
		if where, twice := seen[norm]; twice {
			findings = append(findings, fmt.Sprintf("%s が %s と %s の両方にあります", item.ID, where, item.Where))
			continue
		}
		seen[norm] = item.Where

		if !itemHasTest(item, norm, subtests, testNames) {
			findings = append(findings, fmt.Sprintf("%s にテストがありません（%s）", item.ID, item.Where))
		}
	}
	for _, sub := range subtests {
		named := false
		for _, item := range items {
			if item.Size == sizeLarge && startsWithID(sub.Name, item.ID) {
				named = true
				break
			}
		}
		if !named {
			findings = append(findings, fmt.Sprintf("%s に一覧の ID がありません（%s）", sub.Name, sub.Where))
		}
	}
	return findings
}

// itemHasTest is whether one item has a test of its size: an audit item asks
// for none, a large one for an e2e subtest that starts with its ID, a small or
// medium one for a test whose name holds the ID.
func itemHasTest(item checkItem, norm string, subtests []e2eSubtest, testNames []string) bool {
	switch item.Size {
	case sizeAudit:
		return true
	case sizeLarge:
		return slices.ContainsFunc(subtests, func(sub e2eSubtest) bool { return startsWithID(sub.Name, item.ID) })
	default: // 小 and 中
		return slices.ContainsFunc(testNames, func(name string) bool { return holdsID(name, norm) || holdsID(name, item.ID) })
	}
}

// startsWithID reports whether a name starts with the ID the way a list writes
// it: the ID itself, with its own end, so TEAMS-P1 is not held by a subtest
// named TEAMS-P11.
func startsWithID(name, id string) bool {
	return strings.HasPrefix(name, id) && endsID(name, len(id))
}

// holdsID reports whether a name holds the ID, at a place where the ID ends:
// the 小・中 side, where the ID may stand anywhere in the name.
func holdsID(name, id string) bool {
	for rest := name; ; {
		i := strings.Index(rest, id)
		if i < 0 {
			return false
		}
		if endsID(rest, i+len(id)) {
			return true
		}
		rest = rest[i+1:]
	}
}

// endsID reports whether the ID that ends at one place ends there and no
// further: the byte after it is not a digit. A list's X-1 is not held by a test
// named X-11, while a name may carry on in a space, a dash, an underscore or a
// word of its own.
func endsID(name string, at int) bool {
	if at >= len(name) {
		return true
	}
	c := name[at]
	return c < '0' || c > '9'
}

// checksSummary is the one line the gate fails with: how many findings of each
// kind stand above it. A kind with nothing to say stays out of the line.
func checksSummary(findings []string) string {
	missing, unnamed, twice := 0, 0, 0
	for _, finding := range findings {
		switch {
		case strings.Contains(finding, "にテストがありません"):
			missing++
		case strings.Contains(finding, "に一覧の ID がありません"):
			unnamed++
		default:
			twice++
		}
	}
	var parts []string
	for _, kind := range []struct {
		n    int
		text string
	}{
		{missing, "テストの無い項目"},
		{unnamed, "一覧の ID の無い e2e のテスト"},
		{twice, "2 枚の一覧にある ID"},
	} {
		if kind.n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d 件", kind.text, kind.n))
		}
	}
	return strings.Join(parts, "、")
}
