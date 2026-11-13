package launchlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	observedplace "github.com/aq2bq/hergy/internal/ops/internal/place"
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

// TestAppendAndRead checks metadata order and leaves no blob store behind.
func TestAppendAndRead(t *testing.T) {
	log := testLog(t)
	for _, rec := range []Record{
		{Time: "2026-10-15T22:10:03+09:00", Command: CommandUp, Project: observedplace.Observe("/tmp/project"), Team: fixtureSeen("demo"), TeamYAML: observedplace.Observe("/tmp/project/team.yaml"), Workspace: "workspace", WorkspaceID: "w1"},
		{Time: "2026-10-15T22:20:00+09:00", Command: CommandReprompt, Team: fixtureSeen("demo")},
	} {
		if err := log.Append(rec); err != nil {
			t.Fatal(err)
		}
	}
	records, err := log.Read()
	if err != nil || len(records) != 2 {
		t.Fatalf("records=%+v err=%v", records, err)
	}
	if records[0].Command != CommandUp || records[1].Command != CommandReprompt || records[0].Team.String() != "demo" || records[0].WorkspaceID != "w1" {
		t.Fatalf("metadata changed: %+v", records)
	}
	if _, err := os.Stat(filepath.Join(log.Dir(), "blobs")); !os.IsNotExist(err) {
		t.Fatalf("history created blobs: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(log.Dir(), logName))
	if err != nil || strings.Count(string(raw), "\n") != 2 {
		t.Fatalf("history lines=%q err=%v", raw, err)
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
	if _, ok, err := log.Latest(projectPath("/p/.hergy/teams/demo/team.yaml")); err != nil || ok {
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
// with a workspace wins. Later prompt operations and another project's
// same-named definition cannot hide it.
func TestLatestPicksTheNewestOfOneDefinition(t *testing.T) {
	log := testLog(t)
	const (
		demo   = "/proj/.hergy/teams/demo/team.yaml"
		other  = "/proj/.hergy/teams/other/team.yaml"
		global = "/cfg/hergy/teams/demo/team.yaml"
	)
	for _, rec := range []Record{
		{Command: CommandUp, Workspace: "workspace", WorkspaceID: "w1", TeamYAML: observedplace.Observe(demo)},
		{Command: CommandUp, Workspace: "workspace", WorkspaceID: "w1", TeamYAML: observedplace.Observe(global)},
		{Command: CommandUp, Workspace: "workspace", WorkspaceID: "w1", TeamYAML: observedplace.Observe(other)},
		// The same definition name in another project: the team.yaml tells them
		// apart, so this one is not the launch of the definition above (M1).
		{Command: CommandUp, Workspace: "workspace", WorkspaceID: "w1", TeamYAML: observedplace.Observe("/elsewhere/.hergy/teams/demo/team.yaml")},
		{Command: CommandUp, Workspace: "next-workspace", WorkspaceID: "w2", TeamYAML: observedplace.Observe(demo)},
		{Command: CommandReprompt, TeamYAML: observedplace.Observe(demo)},
	} {
		if err := log.Append(rec); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := log.Latest(projectPath(demo))
	if err != nil || !ok {
		t.Fatalf("Latest() = %v, %v", ok, err)
	}
	if got.Command != CommandUp || got.WorkspaceID != "w2" {
		t.Errorf("Latest(%s) = %+v, want the last launch of it", demo, got)
	}
	if _, ok, err := log.Latest(projectPath("/nowhere/.hergy/teams/demo/team.yaml")); err != nil || ok {
		t.Errorf("Latest() found a record for a definition no launch read: %v, %v", ok, err)
	}
}

// Old records keep their spelling, but the file they identify is canonical.
func TestLatestMatchesCanonicalDefinitionPath(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	realDir := filepath.Join(root, "project")
	if err := os.Mkdir(realDir, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Fatal(err)
	}
	realFile := filepath.Join(realDir, "team.yaml")
	if err := os.WriteFile(realFile, []byte("definition"), 0600); err != nil {
		t.Fatal(err)
	}
	log := At(filepath.Join(root, "launches"))
	recorded := filepath.Join(alias, "team.yaml")
	if err := log.Append(Record{Command: CommandUp, Workspace: "workspace", WorkspaceID: "w1", TeamYAML: observedplace.Observe(recorded)}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := log.Latest(projectPath(realFile))
	if err != nil || !ok || got.TeamYAML.String() != recorded {
		t.Fatalf("latest=%+v, found=%t, err=%v", got, ok, err)
	}
}
