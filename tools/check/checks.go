package main

// The checks gate is TESTING.md's「一覧とテスト」を守らせる: every item of
// checks/<画面>.md has its own test, and every e2e subtest stands under an item.
//
// The lists are the designers' — one row an item, with an ID and the layer the
// item is tested at. The layer says where the test stands: an E2E item is a
// t.Run directly under one of the outer TestE2E* functions in
// internal/app/e2e*_test.go whose name starts with the ID, and a 受け口 item is
// a function or a t.Run of internal/web/*_test.go whose name holds the ID (a
// function name writes the ID's - as _). The two things the gate reports are
// the items nobody tests yet and the e2e subtests that belong to no item: the
// first is the 確かめのワーカー's list of what to write, the second is the
// designer's list of what the lists still lack.

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The layers a checks/<画面>.md row writes in its 層 column.
const (
	layerE2E = "E2E"
	layerAPI = "受け口"
	// The lists, and the two places a test may stand: the e2e tests of
	// internal/app (TESTING.md writes their names as e2e_*_test.go; the file
	// e2e_test.go, where TestE2ENav stands, is one of them) and the 受け口 tests
	// of internal/web.
	checksGlob = "checks/*.md"
	e2eGlob    = "internal/app/e2e*_test.go"
	apiGlob    = "internal/web/*_test.go"
)

// checkItem is one row of a checks/<画面>.md table.
type checkItem struct {
	ID    string
	Layer string
	Where string // checks/activity.md
}

// e2eSubtest is one t.Run standing directly under an outer TestE2E* function.
type e2eSubtest struct {
	Test  string // TestE2EActivity
	Name  string // the name the t.Run is given
	Where string // internal/app/e2e_activity_test.go:95
}

// checksGate holds the lists and the tests to each other.
func (r *runner) checksGate() error {
	items, err := readChecks(r.root)
	if err != nil {
		return err
	}
	subtests, err := readE2ESubtests(r.root)
	if err != nil {
		return err
	}
	names, err := readAPITestNames(r.root)
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
// a list may put its ID, its layer and what it checks in any order.
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
// 層 columns, and every row under it.
func readChecksTable(text, where string) ([]checkItem, error) {
	idCol, layerCol := -1, -1
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
				case "層":
					layerCol = i
				}
			}
			if idCol < 0 || layerCol < 0 {
				return nil, fmt.Errorf("%s: the table's heading names no ID or 層 column", where)
			}
			continue
		}
		if isTableRule(cells) || len(cells) <= idCol || len(cells) <= layerCol {
			continue
		}
		id, layer := cells[idCol], cells[layerCol]
		if id == "" {
			continue
		}
		if layer != layerE2E && layer != layerAPI {
			return nil, fmt.Errorf("%s: %s stands at the layer %q, want %s or %s", where, id, layer, layerE2E, layerAPI)
		}
		out = append(out, checkItem{ID: id, Layer: layer, Where: where})
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

// readE2ESubtests reads every t.Run that stands directly under an outer TestE2E*
// function of the e2e tests of internal/app. A t.Run inside another t.Run stands
// under that one, not under the outer test, so it is not read here.
func readE2ESubtests(root string) ([]e2eSubtest, error) {
	paths, err := filepath.Glob(filepath.Join(root, e2eGlob))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []e2eSubtest
	for _, path := range paths {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, err
		}
		where := filepath.Base(path)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "TestE2E") {
				continue
			}
			for _, run := range directRuns(fn.Body) {
				out = append(out, e2eSubtest{
					Test:  fn.Name.Name,
					Name:  run.name,
					Where: fmt.Sprintf("%s:%d", where, fset.Position(run.pos).Line),
				})
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no t.Run under a TestE2E* function in %s", e2eGlob)
	}
	return out, nil
}

// namedPos is one t.Run's name and where it stands.
type namedPos struct {
	name string
	pos  token.Pos
}

// directRuns reads the t.Run calls one function body registers, without
// descending into a t.Run's own function: those stand under it.
func directRuns(body *ast.BlockStmt) []namedPos {
	var out []namedPos
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isTRun(call) {
			return true
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if name, err := strconv.Unquote(lit.Value); err == nil {
				out = append(out, namedPos{name: name, pos: lit.Pos()})
			}
		}
		return false
	})
	return out
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

// readAPITestNames reads the names a 受け口 test may carry the ID in: every Test*
// function of internal/web/*_test.go, and every t.Run name inside them.
func readAPITestNames(root string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(root, apiGlob))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []string
	for _, path := range paths {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, err
		}
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
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no Test* function in %s", apiGlob)
	}
	return out, nil
}

// checkFindings holds the lists and the tests to each other and returns what
// does not meet: one line an item nobody tests, one line an e2e subtest that
// belongs to no item, one line an ID two lists both write.
func checkFindings(items []checkItem, subtests []e2eSubtest, apiNames []string) []string {
	var findings []string
	seen := map[string]string{}
	for _, item := range items {
		norm := strings.ReplaceAll(item.ID, "-", "_")
		if where, twice := seen[norm]; twice {
			findings = append(findings, fmt.Sprintf("%s が %s と %s の両方にあります", item.ID, where, item.Where))
			continue
		}
		seen[norm] = item.Where

		if item.Layer == layerE2E {
			found := false
			for _, sub := range subtests {
				if startsWithID(sub.Name, item.ID) {
					found = true
					break
				}
			}
			if !found {
				findings = append(findings, fmt.Sprintf("%s にテストがありません（%s）", item.ID, item.Where))
			}
			continue
		}
		found := false
		for _, name := range apiNames {
			if holdsID(name, norm) || holdsID(name, item.ID) {
				found = true
				break
			}
		}
		if !found {
			findings = append(findings, fmt.Sprintf("%s にテストがありません（%s）", item.ID, item.Where))
		}
	}
	for _, sub := range subtests {
		named := false
		for _, item := range items {
			if startsWithID(sub.Name, item.ID) {
				named = true
				break
			}
		}
		if !named {
			findings = append(findings, fmt.Sprintf("%s/%s に一覧の ID がありません（%s）", sub.Test, sub.Name, sub.Where))
		}
	}
	return findings
}

// startsWithID reports whether a name starts with the ID the way a list writes
// it: the ID itself, with its own end, so TEAMS-P1 is not held by a subtest
// named TEAMS-P11.
func startsWithID(name, id string) bool {
	return strings.HasPrefix(name, id) && endsID(name, len(id))
}

// holdsID reports whether a name holds the ID, at a place where the ID ends:
// the 受け口 side, where the ID may stand anywhere in the name.
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
