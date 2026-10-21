package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree writes one file of a made-up tree for the reading tests.
func writeTree(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The columns of a list are found by their headings, not by their places, so a
// list may put its ID, its size and what it checks in any order.
func TestReadChecksTableFindsItsColumnsByHeading(t *testing.T) {
	text := "# a list\n\n" +
		"| 確かめること | ID | 大きさ |\n" +
		"|---|---|---|\n" +
		"| opens the screen | X-1 | 大 |\n" +
		"| answers the api | X-2 | 中 |\n" +
		"| a small rule | X-3 | 小 |\n" +
		"| looks like this | X-4 | 監査 |\n" +
		"\ntrailing words\n"
	items, err := readChecksTable(text, "checks/x.md")
	if err != nil {
		t.Fatal(err)
	}
	want := []checkItem{
		{ID: "X-1", Size: sizeLarge, Where: "checks/x.md"},
		{ID: "X-2", Size: sizeMedium, Where: "checks/x.md"},
		{ID: "X-3", Size: sizeSmall, Where: "checks/x.md"},
		{ID: "X-4", Size: sizeAudit, Where: "checks/x.md"},
	}
	if len(items) != len(want) {
		t.Fatalf("read %d items, want %d: %+v", len(items), len(want), items)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Errorf("item %d is %+v, want %+v", i, items[i], want[i])
		}
	}
}

// A row whose size is not one of the four, and a table whose heading names no
// ID, both stop the reading: the lists are the gate's ground.
func TestReadChecksTableRefusesAListItCannotRead(t *testing.T) {
	for name, text := range map[string]string{
		"an unknown size": "| ID | 大きさ |\n|---|---|\n| X-1 | 大 |\n| X-2 | ふつうのテスト |\n",
		"no ID column":    "| 名前 | 大きさ |\n|---|---|\n| X-1 | 大 |\n",
		"no size column":  "| ID | 名前 |\n|---|---|\n| X-1 | one |\n",
	} {
		if _, err := readChecksTable(text, "checks/x.md"); err == nil {
			t.Errorf("%s: read without an error", name)
		}
	}
}

// The subtests the gate holds to the lists are the t.Run calls standing directly
// under an outer TestE2E* function: one inside another t.Run stands under that
// one, and one in a helper stands under the helper.
func TestReadE2ESubtestsTakesTheRunsOfTheOuterTests(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/app/e2e_x_test.go", `package app

import "testing"

func TestE2EThing(t *testing.T) {
	t.Run("X-1 the first", func(t *testing.T) {})
	t.Run("X-2 the second", func(t *testing.T) {
		t.Run("a run inside a run", func(t *testing.T) {})
	})
	t.Run("a name the ID does not start", func(t *testing.T) {})
}

func TestE2EOther(t *testing.T) {
	t.Run("Y-1 the other", func(t *testing.T) {})
}

func helper(t *testing.T) {
	t.Run("a run in a helper", func(t *testing.T) {})
}

func TestSomethingElse(t *testing.T) {
	t.Run("no e2e test holds this", func(t *testing.T) {})
}
`)
	got, err := readE2ESubtests(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, sub := range got {
		names = append(names, sub.Test+"/"+sub.Name)
	}
	want := []string{
		"TestE2EThing/X-1 the first",
		"TestE2EThing/X-2 the second",
		"TestE2EThing/a name the ID does not start",
		"TestE2EOther/Y-1 the other",
	}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Errorf("read\n%s\nwant\n%s", strings.Join(names, "\n"), strings.Join(want, "\n"))
	}
}

// A 小・中 test may carry the ID in its function name (the - written as _) or in
// a t.Run name; both are read, and the e2e files are left out.
func TestReadTestNamesReadsFunctionsAndRuns(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "internal/web/web_test.go", `package web

import "testing"

func TestX_1TheFunctionName(t *testing.T) {}

func TestSomethingElse(t *testing.T) {
	t.Run("X-2 the run name", func(t *testing.T) {})
}
`)
	writeTree(t, root, "internal/app/e2e_x_test.go", `package app

import "testing"

func TestE2EX(t *testing.T) {
	t.Run("X-9 the browser walk", func(t *testing.T) {})
}
`)
	names, err := readTestNames(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(names, "\n")
	for _, want := range []string{"TestX_1TheFunctionName", "X-2 the run name"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the names lack %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "X-9 the browser walk") {
		t.Errorf("the e2e names were read as a 小・中 test:\n%s", joined)
	}
}

// The two things the gate says: an item no test holds, and an e2e subtest no
// 大 item holds.
func TestCheckFindingsNamesBothKindsOfGap(t *testing.T) {
	items := []checkItem{
		{ID: "X-1", Size: sizeLarge, Where: "checks/x.md"},
		{ID: "X-2", Size: sizeLarge, Where: "checks/x.md"},
		{ID: "X-3", Size: sizeMedium, Where: "checks/x.md"},
		{ID: "X-4", Size: sizeMedium, Where: "checks/x.md"},
		{ID: "X-5", Size: sizeSmall, Where: "checks/x.md"},
		{ID: "X-6", Size: sizeAudit, Where: "checks/x.md"},
	}
	subtests := []e2eSubtest{
		{Test: "TestE2EX", Name: "X-1 the first", Where: "e2e_x_test.go:10"},
		{Test: "TestE2EX", Name: "a stray walk", Where: "e2e_x_test.go:20"},
	}
	testNames := []string{"TestX_3AnswersTheApi"}

	got := strings.Join(checkFindings(items, subtests, testNames), "\n")
	for _, want := range []string{
		"X-2 にテストがありません（checks/x.md）",
		"X-4 にテストがありません（checks/x.md）",
		"X-5 にテストがありません（checks/x.md）",
		"TestE2EX/a stray walk に一覧の ID がありません（e2e_x_test.go:20）",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the findings lack %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "X-1 にテストがありません") || strings.Contains(got, "X-3 にテストがありません") || strings.Contains(got, "X-6 にテストがありません") {
		t.Errorf("a covered item was called missing:\n%s", got)
	}
	if n := len(checkFindings(items, subtests, testNames)); n != 4 {
		t.Errorf("found %d findings, want 4:\n%s", n, got)
	}
}

// A 大 item is not held by a 小・中 test, and the reverse: only the e2e walk
// stands for a 大 item.
func TestCheckFindingsKeepsTheSizesApart(t *testing.T) {
	items := []checkItem{
		{ID: "X-1", Size: sizeLarge, Where: "checks/x.md"},
		{ID: "X-2", Size: sizeMedium, Where: "checks/x.md"},
	}
	subtests := []e2eSubtest{{Test: "TestE2EX", Name: "X-2 the medium walk", Where: "e2e_x_test.go:10"}}
	testNames := []string{"X-1 the large name"}

	got := strings.Join(checkFindings(items, subtests, testNames), "\n")
	if !strings.Contains(got, "X-1 にテストがありません") {
		t.Errorf("the 大 item was taken as covered by a 小・中 name:\n%s", got)
	}
	if !strings.Contains(got, "X-2 にテストがありません") {
		t.Errorf("the 中 item was taken as covered by an e2e walk:\n%s", got)
	}
	if !strings.Contains(got, "TestE2EX/X-2 the medium walk に一覧の ID がありません") {
		t.Errorf("the e2e walk of a 中 ID was not called unnamed:\n%s", got)
	}
}

// An ID covers a name only where the ID ends there: a list's TEAMS-P1 is not
// held by a test named TEAMS-P11, at any size.
func TestCheckFindingsKeepsALongerIDFromCoveringAShorterOne(t *testing.T) {
	items := []checkItem{
		{ID: "X-1", Size: sizeLarge, Where: "checks/x.md"},
		{ID: "X-11", Size: sizeLarge, Where: "checks/x.md"},
		{ID: "X-2", Size: sizeMedium, Where: "checks/x.md"},
		{ID: "X-22", Size: sizeMedium, Where: "checks/x.md"},
	}
	subtests := []e2eSubtest{{Test: "TestE2EX", Name: "X-11 the longer one", Where: "e2e_x_test.go:10"}}
	testNames := []string{"TestX_22TheLongerOne"}

	got := strings.Join(checkFindings(items, subtests, testNames), "\n")
	for _, want := range []string{"X-1 にテストがありません", "X-2 にテストがありません"} {
		if !strings.Contains(got, want) {
			t.Errorf("the shorter ID was taken as covered:\n%s", got)
		}
	}
	for _, never := range []string{"X-11 にテストがありません", "X-22 にテストがありません"} {
		if strings.Contains(got, never) {
			t.Errorf("the longer ID was called missing:\n%s", got)
		}
	}
}

// An ID two lists both write stops the gate: one item, one test, one list.
func TestCheckFindingsNamesAnIDTwoListsWrite(t *testing.T) {
	items := []checkItem{
		{ID: "X-1", Size: sizeLarge, Where: "checks/a.md"},
		{ID: "X-1", Size: sizeLarge, Where: "checks/b.md"},
	}
	got := strings.Join(checkFindings(items, nil, nil), "\n")
	if !strings.Contains(got, "checks/a.md") || !strings.Contains(got, "checks/b.md") {
		t.Errorf("the findings name one list only:\n%s", got)
	}
}

// The line the gate fails with counts each kind of finding it was given.
func TestChecksSummaryCountsEachKind(t *testing.T) {
	findings := []string{
		"X-2 にテストがありません（checks/x.md）",
		"X-4 にテストがありません（checks/x.md）",
		"TestE2EX/a stray walk に一覧の ID がありません（e2e_x_test.go:20）",
	}
	got := checksSummary(findings)
	if !strings.Contains(got, "テストの無い項目 2 件") || !strings.Contains(got, "一覧の ID の無い e2e のテスト 1 件") {
		t.Errorf("the summary is %q", got)
	}
}

// The repo's own lists read, and every row says a size the gate knows. This is
// what the gate stands on: a list it cannot read stops every item in it.
func TestTheRepoListsRead(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	items, err := readChecks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 50 {
		t.Errorf("read %d items from the repo's lists, want the five screens' own", len(items))
	}
	bySize := map[string]int{}
	for _, item := range items {
		bySize[item.Size]++
	}
	for _, size := range []string{sizeSmall, sizeMedium, sizeLarge, sizeAudit} {
		if bySize[size] == 0 {
			t.Errorf("the repo's lists hold no %s item: %v", size, bySize)
		}
	}
	if subtests, err := readE2ESubtests(root); err != nil {
		t.Errorf("the repo's e2e tests read as: %v", err)
	} else if len(subtests) < 11 {
		t.Errorf("read %d e2e subtests from the repo, want every walk the outer tests hold", len(subtests))
	}
	if names, err := readTestNames(root); err != nil {
		t.Errorf("the repo's 小・中 tests read as: %v", err)
	} else if len(names) < 100 {
		t.Errorf("read %d 小・中 test names from the repo", len(names))
	}
}
