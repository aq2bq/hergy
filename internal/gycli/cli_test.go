package gycli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFile writes one file, making its directory.
func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
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
	writeFile(t, filepath.Join(dir, "gy.toml"), "[scopes.common]\n")
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
	writeFile(t, filepath.Join(dir, "gy.toml"), "[scopes.common]\n")
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
