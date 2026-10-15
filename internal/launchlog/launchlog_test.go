package launchlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testLog is a log in a working directory, with XDG_STATE_HOME pointing at it,
// so a test never writes where the person's launches live.
func testLog(t *testing.T) Log {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	log, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	return log
}

// TestDefaultDirFollowsXDGStateHome: state is not settings, and it belongs to
// this machine, so it goes under XDG_STATE_HOME when that is an absolute path,
// and under the home directory when it is not.
func TestDefaultDirFollowsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state")
	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/tmp/state", "hergy", "launches"); dir != want {
		t.Errorf("DefaultDir() = %q, want %q", dir, want)
	}

	// A relative XDG_STATE_HOME is not a place, so it is ignored, the way
	// XDG_CONFIG_HOME is.
	t.Setenv("XDG_STATE_HOME", "somewhere")
	t.Setenv("HOME", "/tmp/home")
	dir, err = DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/tmp/home", ".local", "state", "hergy", "launches"); dir != want {
		t.Errorf("DefaultDir() with a relative XDG_STATE_HOME = %q, want %q", dir, want)
	}
}

// TestAppendAndRead: a record is one line, the texts it points at come back by
// hash, the same text is stored once, and the records read oldest first.
func TestAppendAndRead(t *testing.T) {
	log := testLog(t)
	prompt, err := log.Put([]byte("you are the lead"))
	if err != nil {
		t.Fatal(err)
	}
	twice, err := log.Put([]byte("you are the lead"))
	if err != nil {
		t.Fatal(err)
	}
	if twice != prompt {
		t.Fatalf("the same text was stored as %q and %q", prompt, twice)
	}
	doc, err := log.Put([]byte("rules: one"))
	if err != nil {
		t.Fatal(err)
	}

	first := Record{
		Time: "2026-10-15T22:10:03+09:00", Command: CommandUp, Hergy: "hergy abc1234",
		Project: "/tmp/project", Layer: "local", Definition: "agile-dev", Team: "agile-dev",
		TeamYAML: "/tmp/project/.hergy/teams/agile-dev/team.yaml", Hash: "0123456789abcdef",
		Members: []Member{{Name: "agile-dev-lead", Short: "lead", Prompt: prompt}},
		Docs:    []Doc{{Path: "/tmp/project/.hergy/common/00_TEAM_PROTOCOL.yaml", Layer: "local", Revision: 3, SHA256: doc}},
	}
	if err := log.Append(first); err != nil {
		t.Fatal(err)
	}
	second := Record{
		Time: "2026-10-15T22:20:00+09:00", Command: CommandReprompt, Layer: "local",
		Definition: "agile-dev", Team: "agile-dev", Hash: "0123456789abcdef",
		Members: []Member{{Name: "agile-dev-worker", Short: "worker", Prompt: doc}},
	}
	if err := log.Append(second); err != nil {
		t.Fatal(err)
	}

	records, err := log.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("Read() returned %d records, want 2", len(records))
	}
	if records[0].Command != CommandUp || records[1].Command != CommandReprompt {
		t.Errorf("Read() returned %q then %q, want them oldest first", records[0].Command, records[1].Command)
	}
	if got := records[0]; got.Members[0].Prompt != prompt || got.Docs[0].Revision != 3 || got.Team != "agile-dev" {
		t.Errorf("the first record came back as %+v", got)
	}
	text, err := log.Text(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "you are the lead" {
		t.Errorf("Text(%q) = %q", prompt, text)
	}

	// One blob per text: the prompt was put twice and the doc once.
	entries, err := os.ReadDir(filepath.Join(log.Dir(), blobsDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("blobs/ holds %d files, want 2 (the same text is stored once)", len(entries))
	}
	// The line carries no prompt text: a log is handed around and read by a
	// screen, and the text lives in one place.
	line, err := os.ReadFile(filepath.Join(log.Dir(), logName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(line), "you are the lead") {
		t.Errorf("a line carries the prompt text:\n%s", line)
	}
	if strings.Count(string(line), "\n") != 2 {
		t.Errorf("log.jsonl holds %d lines, want one per launch", strings.Count(string(line), "\n"))
	}
}

// TestReadWithoutALog: nothing has been launched on this machine is an empty
// list, not a failure.
func TestReadWithoutALog(t *testing.T) {
	log := At(filepath.Join(t.TempDir(), "nothing-here"))
	records, err := log.Read()
	if err != nil {
		t.Fatalf("Read() of a machine with no record: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Read() returned %d records, want none", len(records))
	}
	if _, ok, err := log.Latest("local", "demo"); err != nil || ok {
		t.Errorf("Latest() = %v, %v, want none and no failure", ok, err)
	}
}

// TestReadNamesABrokenLine: a line that is not a record is reported with its
// place, rather than read as a record with empty fields.
func TestReadNamesABrokenLine(t *testing.T) {
	log := testLog(t)
	if err := os.MkdirAll(log.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(log.Dir(), logName), []byte("{\"command\":\"up\"}\nnot json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := log.Read()
	if err == nil {
		t.Fatal("Read() accepted a line that is not a record")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("the failure does not name the line: %v", err)
	}
}

// TestLatestPicksTheNewestOfOneDefinition: the last launch of a team definition
// wins, whatever command it was, and a launch of another definition or another
// layer is not it.
func TestLatestPicksTheNewestOfOneDefinition(t *testing.T) {
	log := testLog(t)
	for _, rec := range []Record{
		{Command: CommandUp, Layer: "local", Definition: "demo", Hash: "first"},
		{Command: CommandUp, Layer: "global", Definition: "demo", Hash: "global"},
		{Command: CommandUp, Layer: "local", Definition: "other", Hash: "other"},
		{Command: CommandReprompt, Layer: "local", Definition: "demo", Hash: "last"},
	} {
		if err := log.Append(rec); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := log.Latest("local", "demo")
	if err != nil || !ok {
		t.Fatalf("Latest() = %v, %v", ok, err)
	}
	if got.Hash != "last" || got.Command != CommandReprompt {
		t.Errorf("Latest(local, demo) = %+v, want the last launch of it", got)
	}
	if _, ok, err := log.Latest("global", "other"); err != nil || ok {
		t.Errorf("Latest(global, other) found a record: %v, %v", ok, err)
	}
}

// TestHashIsStableAndSeparates: the hash of a launch does not depend on the
// order an environment happens to be written in, and two launches that hand a
// member different things — a different prompt, another argument, another
// environment, another settings file, another member — do not hash alike.
func TestHashIsStableAndSeparates(t *testing.T) {
	base := Material{
		TeamYAML: []byte("version: 1\n"),
		Members: []MaterialMember{{
			Kind:   "claude",
			Args:   []string{"--model", "opus"},
			Env:    map[string]string{"A": "1", "B": "2"},
			Prompt: "you are the lead",
		}},
	}
	same := Material{
		TeamYAML: []byte("version: 1\n"),
		Members: []MaterialMember{{
			Kind: "claude",
			Args: []string{"--model", "opus"},
			// The same environment, written in the other order.
			Env:    map[string]string{"B": "2", "A": "1"},
			Prompt: "you are the lead",
		}},
	}
	if Hash(base) != Hash(same) {
		t.Errorf("the same launch hashed as %q and %q", Hash(base), Hash(same))
	}

	different := map[string]Material{
		"a prompt that differs": {TeamYAML: base.TeamYAML, Members: []MaterialMember{{Kind: "claude", Args: []string{"--model", "opus"}, Env: base.Members[0].Env, Prompt: "you are the lead!"}}},
		"another argument":      {TeamYAML: base.TeamYAML, Members: []MaterialMember{{Kind: "claude", Args: []string{"--model", "sonnet"}, Env: base.Members[0].Env, Prompt: "you are the lead"}}},
		"another kind":          {TeamYAML: base.TeamYAML, Members: []MaterialMember{{Kind: "codex", Args: []string{"--model", "opus"}, Env: base.Members[0].Env, Prompt: "you are the lead"}}},
		"another environment":   {TeamYAML: base.TeamYAML, Members: []MaterialMember{{Kind: "claude", Args: []string{"--model", "opus"}, Env: map[string]string{"A": "9"}, Prompt: "you are the lead"}}},
		"another settings file": {TeamYAML: []byte("version: 2\n"), Members: base.Members},
		"another member":        {TeamYAML: base.TeamYAML, Members: append(append([]MaterialMember{}, base.Members...), MaterialMember{Kind: "claude", Prompt: "you are the worker"})},
	}
	for what, material := range different {
		if Hash(base) == Hash(material) {
			t.Errorf("%s hashed like the launch it is not: %q", what, Hash(base))
		}
	}
}

// TestHashSeparatesFields: a field cannot slide into the next one, so two
// launches whose members carry the same bytes split differently do not hash
// alike.
func TestHashSeparatesFields(t *testing.T) {
	one := Material{Members: []MaterialMember{{Kind: "ab", Args: []string{"c"}}}}
	other := Material{Members: []MaterialMember{{Kind: "a", Args: []string{"bc"}}}}
	if Hash(one) == Hash(other) {
		t.Errorf("kind ab + arg c hashed like kind a + arg bc")
	}
	env := Material{Members: []MaterialMember{{Kind: "a", Env: map[string]string{"b": "c"}}}}
	if Hash(one) == Hash(env) {
		t.Errorf("a kind hashed like an environment key")
	}
}

// TestPutKeepsABlobWhole: a blob is written under its own hash, so a text that
// is already there is left alone rather than rewritten.
func TestPutKeepsABlobWhole(t *testing.T) {
	log := testLog(t)
	hash, err := log.Put([]byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(log.Dir(), blobsDir, hash)
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Put([]byte("one")); err != nil {
		t.Fatalf("Put() of a text that is already stored: %v", err)
	}
	text, err := log.Text(hash)
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "one" {
		t.Errorf("the stored text is %q", text)
	}
}
