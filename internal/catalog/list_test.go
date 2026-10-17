package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aq2bq/hergy/internal/source"
)

// writeFullTeam writes a team definition with everything a reverse lookup can
// name: two members with their own role documents, a worker one of them may
// hire, the team's own agreement, and the profiles all three name.
func (l layers) writeFullTeam(t *testing.T, layer, name, mission string) {
	t.Helper()
	dir := l.dir(layer)
	write(t, filepath.Join(dir, "profiles.yaml"), `version: 1
profiles:
  claude-default:
    kind: claude
  builder:
    kind: codex
  researcher:
    kind: opencode
`)
	write(t, filepath.Join(dir, "teams", name, "team.yaml"), `version: 1
mission: `+mission+`
team: `+name+`
turns:
  - human -> a
  - a -> b
  - b -> human
workers:
  - from: b
    purpose: 調査
    profile: researcher
members:
  - name: a
    role: lead-role
    profile: claude-default
  - name: b
    role: impl-role
    profile: builder
`)
	write(t, filepath.Join(dir, "teams", name, "agreement.yaml"), "the team's own agreement\n")
	write(t, filepath.Join(dir, "roles", "lead-role.yaml"), "lead's role document\n")
	write(t, filepath.Join(dir, "roles", "impl-role.yaml"), "impl's role document\n")
}

// readFile reads a file a test wrote, for building an edit of it.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestEntriesAreOneRowPerName checks the list a screen reads (C1): one row per
// name, the project's own where both layers hold the name, with the path that is
// not used beside it, and the row of a definition that stands outside the
// project marked as such (I11).
func TestEntriesAreOneRowPerName(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerGlobal, "t", "the global mission")
	l.writeFullTeam(t, source.LayerLocal, "t", "the local mission")
	l.writeFullTeam(t, source.LayerGlobal, "only-global", "outside")

	entries := l.catalog().Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2: %+v", len(entries), entries)
	}
	if entries[0].Name != "only-global" || entries[1].Name != "t" {
		t.Fatalf("entries are not by name: %+v", entries)
	}
	outside := entries[0]
	if !outside.Outside {
		t.Errorf("%q is not marked as outside the project", outside.Name)
	}
	if want := filepath.Join(l.global, "teams", "only-global", "team.yaml"); outside.Path != want {
		t.Errorf("outside path = %q, want %q", outside.Path, want)
	}
	if outside.UnusedPath != "" {
		t.Errorf("outside UnusedPath = %q, want none", outside.UnusedPath)
	}
	if outside.Mission != "outside" || outside.Err != nil {
		t.Errorf("outside row = %+v", outside)
	}

	local := entries[1]
	if local.Outside {
		t.Error("the project's own definition is marked as outside")
	}
	if want := filepath.Join(l.local, "teams", "t", "team.yaml"); local.Path != want {
		t.Errorf("path = %q, want %q", local.Path, want)
	}
	if want := filepath.Join(l.global, "teams", "t", "team.yaml"); local.UnusedPath != want {
		t.Errorf("UnusedPath = %q, want %q", local.UnusedPath, want)
	}
	if local.Mission != "the local mission" {
		t.Errorf("mission = %q, want the local one", local.Mission)
	}
}

// TestEntriesKeepABrokenDefinitionAsARow: a definition whose file does not read
// is still a row, with the reason (C1). A screen shows it rather than dropping
// it from the list.
func TestEntriesKeepABrokenDefinitionAsARow(t *testing.T) {
	l := newLayers(t)
	write(t, filepath.Join(l.local, "teams", "broken", "team.yaml"), "version: 1\nmission:\nmembers: []\n")

	entries := l.catalog().Entries()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Err == nil {
		t.Fatal("a definition that does not read has no reason")
	}
	if entries[0].Mission != "" {
		t.Errorf("mission = %q, want none", entries[0].Mission)
	}
	if entries[0].Path == "" {
		t.Error("the row has no path")
	}
}

// TestReadTakesNoLayer reads by name alone (C1): the project's own first, then
// the one outside it, with no layer for a screen to pass.
func TestReadTakesNoLayer(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerGlobal, "t", "the global mission")
	l.writeFullTeam(t, source.LayerLocal, "t", "the local mission")
	l.writeFullTeam(t, source.LayerGlobal, "only-global", "outside")

	got, err := l.catalog().Read("t")
	if err != nil {
		t.Fatal(err)
	}
	if got.Layer != source.LayerLocal || got.Mission != "the local mission" {
		t.Errorf("t = %q in %q, want the project's own", got.Mission, got.Layer)
	}
	only, err := l.catalog().Read("only-global")
	if err != nil {
		t.Fatal(err)
	}
	if only.Layer != source.LayerGlobal || only.Mission != "outside" {
		t.Errorf("only-global = %q in %q, want the one outside", only.Mission, only.Layer)
	}
	if _, err := l.catalog().Read("absent"); err == nil {
		t.Error("reading an unknown name succeeded, want an error")
	}
}

// TestReadWithNoNameIsTheOneDefinition reads the single definition when no name
// is given, the way the older read does.
func TestReadWithNoNameIsTheOneDefinition(t *testing.T) {
	l := newLayers(t)
	l.writeFullTeam(t, source.LayerLocal, "only", "the only mission")

	got, err := l.catalog().Read("")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "only" {
		t.Errorf("Read(\"\") = %q, want the single definition", got.Name)
	}
	l.writeFullTeam(t, source.LayerLocal, "second", "another mission")
	if _, err := l.catalog().Read(""); err == nil {
		t.Error("reading no name with two definitions succeeded, want the candidates")
	}
}
