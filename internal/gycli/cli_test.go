package gycli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// gyToml writes the one gy.toml a test project keeps: its presence is what says
// the project uses gy (設計 5.3).
func gyToml(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "gy.toml"), []byte("[scopes.common]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeGy writes a fake gy and returns its path. HERGY_GY points at it, so the
// tests never call the machine's own gy.
func fakeGy(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gy")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_GY", path)
}

// TestNoGyIsSilent: a project with no gy.toml, and a machine with no gy, answers
// no questions and no error, so the activity is made from herdr alone (I17,
// 設計 5.3).
func TestNoGyIsSilent(t *testing.T) {
	t.Setenv("HERGY_GY", "")
	t.Setenv("PATH", t.TempDir())
	questions, err := OpenQuestions(t.TempDir())
	if err != nil || questions != nil {
		t.Fatalf("OpenQuestions without gy = %v, %v; want none and no error", questions, err)
	}
}

// TestMissingGyIsSilent: a HERGY_GY that points at nothing is no gy either — a
// binary that is not there cannot have been read, so a screen says nothing rather
// than reporting a failure that did not happen (spec-activity 2.4, E2). The
// project here has its gy.toml, so only the binary is missing.
func TestMissingGyIsSilent(t *testing.T) {
	dir := t.TempDir()
	gyToml(t, dir)
	t.Setenv("HERGY_GY", filepath.Join(dir, "not-here"))
	questions, err := OpenQuestions(dir)
	if err != nil || questions != nil {
		t.Fatalf("OpenQuestions with a gy that is not there = %v, %v; want none and no error", questions, err)
	}
}

// fakeGyLog writes a fake gy that appends one line per call to log, with the
// first two words of the subcommand, and then script. A test reads the lines to
// see which reads a caller ran (6 A5).
func fakeGyLog(t *testing.T, log, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gy")
	body := "#!/bin/sh\necho \"$1 $2\" >> '" + log + "'\n" + script
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERGY_GY", path)
}

// callsOf is the lines one fake gy logged.
func callsOf(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// TestGyWithoutAConfigIsSilent: a machine with gy, in a project that keeps no
// gy.toml, is no gy either (設計 5.3).
func TestGyWithoutAConfigIsSilent(t *testing.T) {
	fakeGy(t, "exit 0\n")
	questions, err := OpenQuestions(t.TempDir())
	if err != nil || questions != nil {
		t.Fatalf("OpenQuestions outside a gy project = %v, %v; want none and no error", questions, err)
	}
}

// TestReadsTheOpenQuestions: with a gy.toml and a gy, the questions come back
// with their title, options, decider, time and writer (C10). The writer is read
// from the write that created the question (設計 10.1).
func TestReadsTheOpenQuestions(t *testing.T) {
	dir := t.TempDir()
	gyToml(t, dir)
	fakeGy(t, `if [ "$1" = "show" ]; then
  echo '[{"id":"'"$2"'","data":{"Question":{"decider":"po","options":["A one","B two"]}}}]'
  exit 0
fi
if [ "$2" = "--since" ]; then
  echo '[{"seq":1,"actor":"demo-lead","node":"q-1","what":"created"}]'
  exit 0
fi
echo '[{"id":"q-1","title":"Which way?","created":"2026-10-17T02:59:58Z"}]'
`)

	questions, err := OpenQuestions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(questions) != 1 {
		t.Fatalf("OpenQuestions = %d questions, want 1", len(questions))
	}
	got := questions[0]
	if got.ID != "q-1" || got.Title != "Which way?" || got.Decider != "po" {
		t.Errorf("question = %+v, want the read title, decider and id", got)
	}
	if len(got.Options) != 2 || got.Options[0] != "A one" {
		t.Errorf("the options did not come back: %v", got.Options)
	}
	if got.Writer != "demo-lead" {
		t.Errorf("the writer is %q, want the actor of the created write", got.Writer)
	}
	want := time.Date(2026, 10, 17, 2, 59, 58, 0, time.UTC)
	if !got.At.Equal(want) {
		t.Errorf("the asked-at time is %s, want %s", got.At, want)
	}
}

// TestAnUnreadableGyIsAnError: a gy that is there and fails is told apart from
// there being none: the screens show one line for it (設計 3.1).
func TestAnUnreadableGyIsAnError(t *testing.T) {
	dir := t.TempDir()
	gyToml(t, dir)
	fakeGy(t, "echo 'boom' >&2\nexit 1\n")

	questions, err := OpenQuestions(dir)
	if err == nil {
		t.Fatalf("OpenQuestions = %v, nil; want an error from the failing gy", questions)
	}
	if questions != nil {
		t.Errorf("a failed read handed back questions: %v", questions)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("the error does not carry what gy said: %v", err)
	}
}

// TestTheLedgerIsNoLedgerWithoutGyOrConfig: the ledger check's opening is the
// activity's: no gy, or no gy.toml, is no ledger here rather than an error, so
// the check says one line and moves on (spec-ledger 3).
func TestTheLedgerIsNoLedgerWithoutGyOrConfig(t *testing.T) {
	t.Setenv("HERGY_GY", "")
	t.Setenv("PATH", t.TempDir())
	if _, ok := OpenLedger(t.TempDir()); ok {
		t.Error("OpenLedger without gy = true, want no ledger here")
	}

	fakeGy(t, "exit 0\n")
	if _, ok := OpenLedger(t.TempDir()); ok {
		t.Error("OpenLedger outside a gy project = true, want no ledger here")
	}
}

// TestReadsTheLedgerWrites: with a gy.toml and a gy, every write comes back with
// its writer, its node and its why, and a write with no writer is skipped (the
// rules cannot judge it). The why is what names the write (spec-ledger).
func TestReadsTheLedgerWrites(t *testing.T) {
	dir := t.TempDir()
	gyToml(t, dir)
	fakeGy(t, `if [ "$1" = "list" ]; then
  echo '[{"seq":1,"actor":"hergy-dev-lead","node":"r-1","what":"created","why":"req add r-1"},{"seq":2,"actor":"hergy-dev-designer","node":"r-1","what":"changed","why":"req approve r-1"},{"seq":3,"actor":"","node":"r-2","what":"changed","why":"req done r-2"},{"seq":4,"actor":"hergy-dev-lead","node":"r-3","what":"changed","why":""}]'
  exit 0
fi
`)

	ledger, ok := OpenLedger(dir)
	if !ok {
		t.Fatal("OpenLedger = false, want a ledger here")
	}
	writes, err := ledger.Writes()
	if err != nil {
		t.Fatal(err)
	}
	want := []Write{
		{Actor: "hergy-dev-lead", Node: "r-1", Why: "req add r-1"},
		{Actor: "hergy-dev-designer", Node: "r-1", Why: "req approve r-1"},
	}
	if len(writes) != len(want) {
		t.Fatalf("Writes = %v, want %v", writes, want)
	}
	for i := range want {
		if writes[i] != want[i] {
			t.Errorf("Writes[%d] = %+v, want %+v", i, writes[i], want[i])
		}
	}
}

// TestReadsAQuestionsDecider: the decider of one question comes back, so the
// check can tell a question to a member from a question to the human.
func TestReadsAQuestionsDecider(t *testing.T) {
	dir := t.TempDir()
	gyToml(t, dir)
	fakeGy(t, `if [ "$1" = "show" ]; then
  echo '[{"id":"'"$2"'","data":{"Question":{"decider":"hergy-dev-lead","options":["A one","B two"]}}}]'
  exit 0
fi
`)

	ledger, ok := OpenLedger(dir)
	if !ok {
		t.Fatal("OpenLedger = false, want a ledger here")
	}
	got, err := ledger.Decider("q-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hergy-dev-lead" {
		t.Errorf("Decider = %q, want %q", got, "hergy-dev-lead")
	}
}

// TestAnUnreadableLedgerIsAnError: a gy that is there and fails is an error from
// the ledger reads too, so the check says the ledger was not checked rather than
// finding no broken rule (spec-ledger 3).
func TestAnUnreadableLedgerIsAnError(t *testing.T) {
	dir := t.TempDir()
	gyToml(t, dir)
	fakeGy(t, "echo 'boom' >&2\nexit 1\n")

	ledger, ok := OpenLedger(dir)
	if !ok {
		t.Fatal("OpenLedger = false, want a ledger here")
	}
	writes, err := ledger.Writes()
	if err == nil {
		t.Fatalf("Writes = %v, nil; want an error from the failing gy", writes)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("the error does not carry what gy said: %v", err)
	}
}

// TestOpenQuestionsByFiltersBeforeShowing (6 A5): a caller that names the
// writers it wants — a screen names one team's member herdr names — asks gy
// about those questions only, so 55 open questions asked by 20 people cost 3
// reads for one team rather than 55.
func TestOpenQuestionsByFiltersBeforeShowing(t *testing.T) {
	dir := t.TempDir()
	gyToml(t, dir)
	log := filepath.Join(dir, "calls")
	fakeGyLog(t, log, `if [ "$1" = "show" ]; then
  echo '[{"id":"'"$2"'","data":{"Question":{"decider":"po","options":["A one"]}}}]'
  exit 0
fi
if [ "$2" = "--since" ]; then
  echo '[{"seq":1,"actor":"demo-lead","node":"q-1","what":"created"},{"seq":2,"actor":"other-lead","node":"q-2","what":"created"},{"seq":3,"actor":"demo-coder","node":"q-3","what":"created"}]'
  exit 0
fi
echo '[{"id":"q-1","title":"mine","created":"2026-10-17T02:59:58Z"},{"id":"q-2","title":"theirs","created":"2026-10-17T02:00:00Z"},{"id":"q-3","title":"my worker'"'"'s","created":"2026-10-17T01:00:00Z"}]'
`)

	questions, err := OpenQuestionsBy(dir, []string{"demo-lead", "demo-coder"})
	if err != nil {
		t.Fatal(err)
	}
	if len(questions) != 2 || questions[0].ID != "q-1" || questions[1].ID != "q-3" {
		t.Fatalf("OpenQuestionsBy = %+v, want q-1 and q-3 only", questions)
	}
	for _, call := range callsOf(t, log) {
		if strings.Contains(call, "q-2") {
			t.Errorf("a question of another writer was read: %q", call)
		}
	}
	if got := callsOf(t, log); !slices.Contains(got, "show q-1") || !slices.Contains(got, "show q-3") {
		t.Errorf("the wanted questions were not read: %v", got)
	}
}

// TestAListIsRememberedForTenSeconds (6 A5): the activity's own read runs the
// same two lists once per row a screen opens, so the answer is remembered for
// ten seconds and the ledger is not asked again. A question's own read is not
// remembered.
func TestAListIsRememberedForTenSeconds(t *testing.T) {
	dir := t.TempDir()
	gyToml(t, dir)
	log := filepath.Join(dir, "calls")
	fakeGyLog(t, log, `if [ "$1" = "show" ]; then
  echo '[{"id":"'"$2"'","data":{"Question":{"decider":"po","options":[]}}}]'
  exit 0
fi
if [ "$2" = "--since" ]; then
  echo '[{"seq":1,"actor":"demo-lead","node":"q-1","what":"created"}]'
  exit 0
fi
echo '[{"id":"q-1","title":"mine","created":"2026-10-17T02:59:58Z"}]'
`)

	for range 2 {
		if _, err := OpenQuestions(dir); err != nil {
			t.Fatal(err)
		}
	}
	lists := 0
	for _, call := range callsOf(t, log) {
		if strings.HasPrefix(call, "list ") {
			lists++
		}
	}
	if lists != 2 {
		t.Errorf("two reads ran %d lists, want 2 (one of each, the second remembered)", lists)
	}
}
